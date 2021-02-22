/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/version"
)

// setupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
//
// Based on k8s.io/apiserver/pkg/server.SetupSignalHandler. This implementation may be
// called multiple times.
func setupSignalHandler() <-chan struct{} {
	shutdownHandler := make(chan os.Signal, 2)
	stop := make(chan struct{})
	signal.Notify(shutdownHandler, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-shutdownHandler
		close(stop)
		<-shutdownHandler
		os.Exit(1) // Second signal - exit directly.
	}()

	return stop
}

func main() {
	klog.InitFlags(nil)
	flags.Register()

	klog.Infof("managed-certificate-controller %s starting. Latest commit hash: %s",
		version.Version, version.GitCommit)
	for i, a := range os.Args {
		klog.Infof("argv[%d]: %q", i, a)
	}
	klog.Infof("Flags = %+v", flags.F)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := config.New(ctx, flags.F.GCEConfigFilePath)
	if err != nil {
		klog.Fatal(err)
	}

	clients, err := clients.New(ctx, config)
	if err != nil {
		klog.Fatal(err)
	}

	leaderElection := componentbaseconfig.LeaderElectionConfiguration{
		LeaderElect:   true,
		LeaseDuration: metav1.Duration{Duration: config.MasterElection.LeaseDuration},
		RenewDeadline: metav1.Duration{Duration: config.MasterElection.RenewDeadline},
		RetryPeriod:   metav1.Duration{Duration: config.MasterElection.RetryPeriod},
		ResourceLock:  resourcelock.EndpointsResourceLock,
	}
	id, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Unable to get hostname: %v", err)
	}
	lock, err := resourcelock.New(
		leaderElection.ResourceLock,
		"kube-system",
		"managed-certificate-controller",
		clients.Core,
		clients.Coordination,
		resourcelock.ResourceLockConfig{Identity: id},
	)
	if err != nil {
		klog.Fatalf("Unable to create leader election lock: %v", err)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: leaderElection.LeaseDuration.Duration,
		RenewDeadline: leaderElection.RenewDeadline.Duration,
		RetryPeriod:   leaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				controller := controller.New(ctx, config, clients)

				go func() {
					<-setupSignalHandler()
					cancel()
				}()

				if err = controller.Run(ctx); err != nil {
					klog.Fatalf("Error running controller: %s", err)
				}
			},
			OnStoppedLeading: func() {
				// Cancel ctx, shut down and wait for being restarted by kubelet.
				cancel()
			},
		},
	})
}
