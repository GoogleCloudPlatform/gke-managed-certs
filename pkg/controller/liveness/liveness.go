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
)

// ActivityName represents the actions of the controller which are monitored
// by the health checks.
type ActivityName int

const (
	// Undefined indicates a default action.
	Undefined ActivityName = iota

	// SynchronizeAll indicates a Controller.SynchronizeAll() call.
	SynchronizeAll

	// McrtResyncProcess indicates a ManagedCertificate object is
	// processed by the lower priority queue in the controller.
	McrtResyncProcess

	// IngressResyncProcess indicates an Ingress object is processed by
	// the lower priority queue in the controller.
	IngressResyncProcess
)

// HealthCheck implements an HTTP endpoint through which it communicates
// the liveness status of the controller by responding with 2xx HTTP codes
// when the controller is alive and with 5xx HTTP codes when it is dead.
//
// HealthCheck tracks the last time of when the following actions were performed
// by the controller and when they succeeded:
// - SynchronizeAll: Add all known resources to the controller queues.
// - McrtResyncProcess, IngressResyncProcess: process ManagedCertificate or Ingress,
// respectively, on a low priority workqueue.
//
// - If the last time SynchronizeAll was performed or succeeded is too far ago,
// HealthCheck will respond that the controller is dead.
// - If an Ingress or a ManagedCertificate resource was queued for processing by
// the *previous* SynchronizeAll call and was not processed until `syncSuccessTimeout`
// elapses, HealthCheck will respond that the controller is dead.
// - If health checking is not running, it always responds that the controller is
// alive. It is not running by default.
//
// ## Usage
//
// # Create health check with
// h := liveness.NewHealthCheck()
//
// # Begin serving the liveness status
// h.StartServing()
//
// # When ready, after the instance elected as a master, begin monitoring
// h.StartMonitoring()
//
// # End monitoring and shut down
// h.Stop()
type HealthCheck struct {
	mutex sync.RWMutex

	// lastSuccessSync contains details of the latest Controller.synchronizeAll
	// successful run.
	lastSuccessSync syncDetails
	// prevSuccessSync contains details of the second latest Controller.synchronizeAll
	// successful run.
	prevSuccessSync syncDetails

	// lastActivity contains the last time each activity ran.
	// Activities are accessed through the ActivityName enum.
	lastActivity map[ActivityName]time.Time

	// syncActivityTimeout is the max time allowed after last activity
	// to consider the controller alive.
	syncActivityTimeout time.Duration
	// syncSuccessTimeout is the max time allowed after last successful
	// activity to consider the controller alive.
	syncSuccessTimeout time.Duration

	// syncMonitoringEnabled is true if synchronizeAll health checks are enabled.
	syncMonitoringEnabled bool
	// healthCheckInterval configures how often the health-checks run.
	healthCheckInterval time.Duration
	// alive is true if the latest health-check found no errors, false otherwise.
	alive           bool
	syncAllErr      error
	ingressQueueErr error
	mcrtQueueErr    error

	// monitoringRunning is true if the healthiness is monitored and affects
	// the liveness status.
	monitoringRunning bool
	// httpServer handles HTTP liveness probe calls.
	httpServer http.Server
}

// syncDetails contains details about a single Conroller.SynchronizeAll() call.
type syncDetails struct {
	runTime time.Time
	// whether or not it scheduled an Ingress object to be processed
	ingressScheduled bool
	// whether or not it scheduled a ManagedCertificate object to be processed
	mcrtScheduled bool
}

// NewHealthCheck builds new HealthCheck object with given timeout.
func NewHealthCheck(healthCheckInterval, activityTimeout,
	successTimeout time.Duration) *HealthCheck {

	return &HealthCheck{
		lastActivity:          make(map[ActivityName]time.Time),
		syncActivityTimeout:   activityTimeout,
		syncSuccessTimeout:    successTimeout,
		syncMonitoringEnabled: false,
		healthCheckInterval:   healthCheckInterval,
		alive:                 false,
		monitoringRunning:     false,
	}
}

// StartServing starts the liveness probe http server.
// After StartServing is called, the probe starts reporting its status to Kubelet
// on the endpoint `address`/`path`.
func (hc *HealthCheck) StartServing(address, path string) {
	mux := http.NewServeMux()
	hc.httpServer = http.Server{Addr: address, Handler: mux}
	mux.HandleFunc(path, hc.serveHTTP)

	go func() {
		err := hc.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			klog.Fatalf("Failed to start server: %v", err)
		}
	}()
}

// StartMonitoring starts controller health checks.
// After StartMonitoring is called, the probe starts monitoring the healthiness
// of the controller and may report that it is dead. Monitoring runs every
// healthCheckInterval until Stop() is called.
func (hc *HealthCheck) StartMonitoring() {
	hc.mutex.Lock()
	hc.syncMonitoringEnabled = true
	hc.monitoringRunning = true
	hc.mutex.Unlock()

	go func() {
		for {
			hc.mutex.RLock()
			if !hc.monitoringRunning {
				hc.mutex.RUnlock()
				break
			}
			hc.mutex.RUnlock()

			select {
			case <-time.After(hc.healthCheckInterval):
				alive := !hc.checkAll()
				hc.mutex.Lock()
				hc.alive = alive
				hc.mutex.Unlock()
			}
		}
	}()
}

// Stop stops controller health checks and the liveness probe http server.
func (hc *HealthCheck) Stop() error {
	hc.mutex.Lock()
	hc.monitoringRunning = false
	hc.mutex.Unlock()
	if err := hc.httpServer.Shutdown(context.Background()); err != nil {
		klog.Errorf("Failed to stop server: %v", err)
		return err
	}
	return nil
}

// serveHTTP implements http.Handler interface to provide a health-check endpoint.
// If health-check is enabled, it runs all health checks.
// If health-check is not enabled, it always responds that the controller is alive.
func (hc *HealthCheck) serveHTTP(w http.ResponseWriter, r *http.Request) {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	if !hc.syncMonitoringEnabled || hc.alive {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(500)
	}
}

// checkAll runs all the health checks and returns true if there is an error,
// in any of them.
func (hc *HealthCheck) checkAll() bool {
	syncAllErr := hc.checkSyncAllTimeOut()
	ingressQueueErr := hc.checkIngressQueueHealth()
	mcrtQueueErr := hc.checkMcrtQueueHealth()

	if syncAllErr != nil && hc.syncAllErr == nil {
		klog.Errorf("Health check: %v", syncAllErr)
	}
	if ingressQueueErr != nil && hc.ingressQueueErr == nil {
		klog.Errorf("Health check: %v", ingressQueueErr)
	}
	if mcrtQueueErr != nil && hc.mcrtQueueErr == nil {
		klog.Errorf("Health check: %v", mcrtQueueErr)
	}
	hc.syncAllErr = syncAllErr
	hc.ingressQueueErr = ingressQueueErr
	hc.mcrtQueueErr = mcrtQueueErr
	return hc.syncAllErr != nil || hc.ingressQueueErr != nil || hc.mcrtQueueErr != nil
}

// checkSyncAllTimeOut checks the last time synchronizeAll was run and the
// last time synchronizeAll was run successfully, and checks if any of them was
// too long ago.
//
// Returns error if any of them timed out, nil otherwise.
func (hc *HealthCheck) checkSyncAllTimeOut() error {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	lastSyncAll := hc.lastActivity[SynchronizeAll]
	lastSuccessSyncAll := hc.lastSuccessSync.runTime
	now := time.Now()
	activityTimedOut := now.After(lastSyncAll.Add(hc.syncActivityTimeout))
	successTimedOut := now.After(lastSuccessSyncAll.Add(hc.syncSuccessTimeout))

	if activityTimedOut {
		timeoutDelta := now.Sub(lastSyncAll) - hc.syncActivityTimeout
		return fmt.Errorf(
			`last synchronizeAll activity %s ago, which exceeds the %s timeout by %s`,
			now.Sub(lastSyncAll), hc.syncActivityTimeout, timeoutDelta)
	} else if successTimedOut {
		timeoutDelta := now.Sub(hc.lastSuccessSync.runTime) - hc.syncSuccessTimeout
		return fmt.Errorf(
			"last synchronizeAll success activity %s ago, which exceeds "+
				"the %s timeout by %s",
			now.Sub(hc.lastSuccessSync.runTime), hc.syncSuccessTimeout, timeoutDelta)
	}
	return nil
}

// checkIngressQueueHealth checks if processNext was called successfully after
// previous synchronizeAll, only in case previous synchronizeAll call had Ingress objects
// added to ingress resync queue.
// i.e if processNext was not run successfully in the time between
// previous synchronizeAll and now, report not healthy.
//
// Returns error if not healthy, nil otherwise.
func (hc *HealthCheck) checkIngressQueueHealth() error {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	if !hc.prevSuccessSync.ingressScheduled {
		return nil
	}
	now := time.Now()
	lastIngressResync := hc.lastActivity[IngressResyncProcess]
	if lastIngressResync.Before(hc.prevSuccessSync.runTime) {
		return fmt.Errorf(
			"previous synchronizeAll added Ingress objects to queue %s ago, "+
				"while last processed Ingress %s ago, %s before",
			now.Sub(hc.prevSuccessSync.runTime),
			now.Sub(lastIngressResync),
			hc.prevSuccessSync.runTime.Sub(lastIngressResync))
	}
	return nil
}

// checkMcrtQueueHealth checks if processNext was called successfully after
// previous synchronizeAll, only in case previous synchronizeAll call had ManagedCertificate
// objects added to ManagedCertificate resync queue.
// i.e if processNext was not run successfully in the time between
// previous synchronizeAll and now, report not healthy.
//
// Returns error if not healthy, nil otherwise.
func (hc *HealthCheck) checkMcrtQueueHealth() error {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()
	if !hc.prevSuccessSync.mcrtScheduled {
		return nil
	}
	now := time.Now()
	lastMcrtResync := hc.lastActivity[McrtResyncProcess]
	if lastMcrtResync.Before(hc.prevSuccessSync.runTime) {
		return fmt.Errorf(
			"previous synchronizeAll added ManagedCertificate objects to queue %s "+
				"ago, while last processed ManagedCertificate %s ago, %s before",
			now.Sub(hc.prevSuccessSync.runTime),
			now.Sub(lastMcrtResync),
			hc.prevSuccessSync.runTime.Sub(lastMcrtResync))
	}
	return nil
}

// UpdateLastActivity updates last time of an activity.
func (hc *HealthCheck) UpdateLastActivity(activityName ActivityName, timestamp time.Time) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	if timestamp.After(hc.lastActivity[activityName]) {
		hc.lastActivity[activityName] = timestamp
	}
}

// UpdateLastSuccessSync updates last time of successful (i.e. not ending
// in error) Controller.synchronizeAll activity.
func (hc *HealthCheck) UpdateLastSuccessSync(timestamp time.Time,
	ingressScheduled, mcrtScheduled bool) {

	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	if timestamp.After(hc.lastSuccessSync.runTime) {
		hc.prevSuccessSync = hc.lastSuccessSync
		hc.lastSuccessSync.runTime = timestamp
		hc.lastSuccessSync.ingressScheduled = ingressScheduled
		hc.lastSuccessSync.mcrtScheduled = mcrtScheduled
	}

	// finishing successful run is also a sign of activity
	if timestamp.After(hc.lastActivity[SynchronizeAll]) {
		hc.lastActivity[SynchronizeAll] = timestamp
	}
}
