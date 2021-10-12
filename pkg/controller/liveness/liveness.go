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

// Package liveness implements Kubernetes health checks for the controller.
// The health checks affect whether the controller is considered alive or dead
// (in which case it is restarted).
package liveness

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
)

// HealthCheck implements an HTTP endpoint through which it communicates
// the liveness status of the controller by responding with 2xx HTTP codes
// when the controller is alive and with 5xx HTTP codes when it is dead.
//
// HealthCheck tracks the last time of when the following actions were performed
// by the controller and when they succeeded:
// - Add all known resources to the controller queues.
//
// If the last time the actions were performed or succeeded is too far ago, HealthCheck
// will respond that the controller is dead. If not enabled, it always responds that
// the controller is alive. It is not enabled by default.
type HealthCheck struct {
	mutex sync.Mutex

	lastActivity      time.Time // stores the time the last activity ran
	lastSuccessfulRun time.Time // stores the time the last activity ran successfully

	// activityTimeout stores max time allowed after last activity to consider the
	// controller alive.
	activityTimeout time.Duration

	// successTimeout stores max time allowed after last successful activity to
	// consider the controller alive.
	successTimeout time.Duration

	// enabled stores whether to check for last activity and last successful
	// activity timeout or not.
	enabled bool

	httpServer http.Server // is the endpoint handling health checks
}

// NewHealthCheck builds new HealthCheck object with given timeout.
func NewHealthCheck(activityTimeout, successTimeout time.Duration) *HealthCheck {
	now := time.Now()
	return &HealthCheck{
		lastActivity:      now,
		lastSuccessfulRun: now,
		activityTimeout:   activityTimeout,
		successTimeout:    successTimeout,
		enabled:           false,
		httpServer:        http.Server{},
	}
}

// StartMonitoring activates checks for controller inactivity.
func (hc *HealthCheck) StartMonitoring() {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.enabled = true
	now := time.Now()
	if now.After(hc.lastActivity) {
		hc.lastActivity = now
	}
	if now.After(hc.lastSuccessfulRun) {
		hc.lastSuccessfulRun = now
	}
}

func (hc *HealthCheck) StartServer(healthCheckAddress string) error {
	mux := http.NewServeMux()
	hc.httpServer = http.Server{Addr: healthCheckAddress, Handler: mux}
	mux.HandleFunc(flags.F.HealthCheckPath, hc.ServeHTTP)

	err := hc.httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		klog.Fatalf("Failed to start server: %v", err)
		return err
	}
	return nil
}

func (hc *HealthCheck) StopServer() error {
	if err := hc.httpServer.Shutdown(context.Background()); err != nil {
		klog.Fatalf("Failed to stop server: %v", err)
		return err
	}
	return nil
}

// ServeHTTP implements http.Handler interface to provide a health-check endpoint.
// If health-check is enabled, it checks the last time an activity was done and the
// last time a successful activity was done, and checks if any of them timed-out.
// If health-check is not, it always responds that the controller is alive.
func (hc *HealthCheck) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hc.mutex.Lock()

	lastActivity := hc.lastActivity
	lastSuccessfulRun := hc.lastSuccessfulRun
	now := time.Now()
	activityTimedOut := now.After(lastActivity.Add(hc.activityTimeout))
	successTimedOut := now.After(lastSuccessfulRun.Add(hc.successTimeout))
	timedOut := hc.enabled && (activityTimedOut || successTimedOut)

	hc.mutex.Unlock()

	if timedOut {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("Error: last activity more than %v ago, last success more than %v ago", time.Now().Sub(lastActivity).String(), time.Now().Sub(lastSuccessfulRun).String())))
	} else {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}
}

// UpdateLastSync updates last time of activity.
func (hc *HealthCheck) UpdateLastSync(timestamp time.Time) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	if timestamp.After(hc.lastActivity) {
		hc.lastActivity = timestamp
	}
}

// UpdateLastSuccessfulSync updates last time of successful (i.e. not ending in error) activity.
func (hc *HealthCheck) UpdateLastSuccessfulSync(timestamp time.Time) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	if timestamp.After(hc.lastSuccessfulRun) {
		hc.lastSuccessfulRun = timestamp
	}
	// finishing successful run is also a sign of activity
	if timestamp.After(hc.lastActivity) {
		hc.lastActivity = timestamp
	}
}
