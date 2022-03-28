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

package liveness

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/liveness/testhelpers"
)

const (
	healthCheckInterval = 10 * time.Millisecond
	timeout             = 5 * time.Second
	healthCheckPath     = "/test-health-check"
)

var addressManager = testhelpers.NewAddressManager()

func TestHTTPServer(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description          string
		lastSyncDelta        time.Duration
		lastSuccessSyncDelta time.Duration
		enabled              bool
		wantResponseCode     int
	}{
		{
			description:          "Serve http OK",
			lastSyncDelta:        0,
			lastSuccessSyncDelta: 0,
			enabled:              false,
			wantResponseCode:     200,
		},
		{
			description:          "Last synchronizeAll timed-out",
			lastSyncDelta:        -2 * timeout,
			lastSuccessSyncDelta: 0,
			enabled:              true,
			wantResponseCode:     500,
		},
		{
			description:          "Last success synchronizeAll timed-out",
			lastSyncDelta:        0,
			lastSuccessSyncDelta: -2 * timeout,
			enabled:              true,
			wantResponseCode:     500,
		},
		{
			description:          "Both synchronizeAll last success and activity timed-out",
			lastSyncDelta:        -2 * timeout,
			lastSuccessSyncDelta: -2 * timeout,
			enabled:              true,
			wantResponseCode:     500,
		},
		{
			description:          "Health-check disabled",
			lastSyncDelta:        0,
			lastSuccessSyncDelta: 0,
			enabled:              false,
			wantResponseCode:     200,
		},
		{
			description:          "Last synchronizeAll timed-out, healthCheck disabled",
			lastSyncDelta:        -2 * timeout,
			lastSuccessSyncDelta: -2 * timeout,
			enabled:              false,
			wantResponseCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
			now := time.Now()
			lastSync := now.Add(tc.lastSyncDelta)
			lastSuccessSync := now.Add(tc.lastSuccessSyncDelta)
			healthCheck.lastActivity[SynchronizeAll] = lastSync
			healthCheck.lastSuccessSync.runTime = lastSuccessSync

			if tc.enabled {
				address := addressManager.GetNextAddress()
				healthCheck.StartServing(address, healthCheckPath)
				healthCheck.StartMonitoring()
				t.Cleanup(func() { healthCheck.Stop() })
			}
			assertServeHttpResponseCode(t, healthCheck, tc.wantResponseCode)
		})
	}
}

func TestCheckSyncAllTimeOut(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description          string
		lastSyncDelta        time.Duration
		lastSuccessSyncDelta time.Duration
		wantError            bool
	}{
		{
			description:          "Both now",
			lastSyncDelta:        0,
			lastSuccessSyncDelta: 0,
			wantError:            false,
		},
		{
			description:          "Both in future",
			lastSyncDelta:        timeout,
			lastSuccessSyncDelta: timeout,
			wantError:            false,
		},
		{
			description:          "Both before, no timeout",
			lastSyncDelta:        -timeout / 2,
			lastSuccessSyncDelta: -timeout / 2,
			wantError:            false,
		},
		{
			description:          "Last SynchronizeAll timed-out",
			lastSyncDelta:        -timeout * 2,
			lastSuccessSyncDelta: -timeout / 2,
			wantError:            true,
		},
		{
			description:          "Last success SynchronizeAll timed-out",
			lastSyncDelta:        -timeout / 2,
			lastSuccessSyncDelta: -timeout * 2,
			wantError:            true,
		},
		{
			description:          "Both timed-out",
			lastSyncDelta:        -timeout * 2,
			lastSuccessSyncDelta: -timeout * 2,
			wantError:            true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
			now := time.Now()
			lastSync := now.Add(tc.lastSyncDelta)
			lastSuccessSync := now.Add(tc.lastSuccessSyncDelta)
			healthCheck.lastActivity[SynchronizeAll] = lastSync
			healthCheck.lastSuccessSync.runTime = lastSuccessSync

			err := healthCheck.checkSyncAllTimeOut()
			if tc.wantError && err == nil {
				t.Fatalf("checkSyncAllTimeOut() returned nil, want error")
			} else if !tc.wantError && err != nil {
				t.Fatalf("checkSyncAllTimeOut() returned error, want nil")
			}
		})
	}
}

func TestCheckIngressQueueHealth(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description             string
		prevIngressScheduled    bool
		prevSuccessSyncDelta    time.Duration
		lastIngressProcessDelta time.Duration
		lastSuccessSyncDelta    time.Duration
		wantError               bool
	}{
		{
			description:             "Process in between",
			prevIngressScheduled:    true,
			prevSuccessSyncDelta:    -3 * time.Second,
			lastIngressProcessDelta: -2 * time.Second,
			lastSuccessSyncDelta:    -1 * time.Second,
			wantError:               false,
		},
		{
			description:             "Process before previous SynchronizeAll",
			prevIngressScheduled:    true,
			prevSuccessSyncDelta:    -3 * time.Second,
			lastIngressProcessDelta: -4 * time.Second,
			lastSuccessSyncDelta:    -1 * time.Second,
			wantError:               true,
		},
		{
			description:             "Process in future",
			prevIngressScheduled:    true,
			prevSuccessSyncDelta:    -3 * time.Second,
			lastIngressProcessDelta: 1 * time.Second,
			lastSuccessSyncDelta:    -1 * time.Second,
			wantError:               false,
		},
		{
			description:             "No process",
			prevIngressScheduled:    true,
			prevSuccessSyncDelta:    -3 * time.Second,
			lastIngressProcessDelta: -999999 * time.Second,
			lastSuccessSyncDelta:    -1 * time.Second,
			wantError:               true,
		},
		{
			description:             "Process in between, no Ingress scheduled",
			prevIngressScheduled:    false,
			prevSuccessSyncDelta:    -3 * time.Second,
			lastIngressProcessDelta: -2 * time.Second,
			lastSuccessSyncDelta:    -1 * time.Second,
			wantError:               false,
		},
		{
			description:             "Process before previous SynchronizeAll (or no process), no Ingress scheduled",
			prevIngressScheduled:    false,
			prevSuccessSyncDelta:    -3 * time.Second,
			lastIngressProcessDelta: -4 * time.Second,
			lastSuccessSyncDelta:    -1 * time.Second,
			wantError:               false,
		},
		{
			description:             "Process in future, no Ingress scheduled",
			prevIngressScheduled:    false,
			prevSuccessSyncDelta:    -3 * time.Second,
			lastIngressProcessDelta: 1 * time.Second,
			lastSuccessSyncDelta:    -1 * time.Second,
			wantError:               false,
		},
		{
			description:             "No process, no Ingress scheduled",
			prevIngressScheduled:    false,
			prevSuccessSyncDelta:    -3 * time.Second,
			lastIngressProcessDelta: -999999 * time.Second,
			lastSuccessSyncDelta:    -1 * time.Second,
			wantError:               false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
			now := time.Now()
			healthCheck.prevSuccessSync.ingressScheduled = tc.prevIngressScheduled
			healthCheck.prevSuccessSync.runTime = now.Add(tc.prevSuccessSyncDelta)
			healthCheck.lastActivity[IngressResyncProcess] = now.Add(tc.lastIngressProcessDelta)
			healthCheck.lastSuccessSync.runTime = now.Add(tc.lastSuccessSyncDelta)

			err := healthCheck.checkIngressQueueHealth()
			if tc.wantError && err == nil {
				t.Fatalf("checkIngressQueueHealth() returned nil, want error")
			} else if !tc.wantError && err != nil {
				t.Fatalf("checkIngressQueueHealth() returned error, want nil")
			}
		})
	}
}

func TestCheckMcrtQueueHealth(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description          string
		prevMcrtScheduled    bool
		prevSuccessSyncDelta time.Duration
		lastMcrtProcessDelta time.Duration
		lastSuccessSyncDelta time.Duration
		wantError            bool
	}{
		{
			description:          "Process in between",
			prevMcrtScheduled:    true,
			prevSuccessSyncDelta: -3 * time.Second,
			lastMcrtProcessDelta: -2 * time.Second,
			lastSuccessSyncDelta: -1 * time.Second,
			wantError:            false,
		},
		{
			description:          "Process before previous SynchronizeAll",
			prevMcrtScheduled:    true,
			prevSuccessSyncDelta: -3 * time.Second,
			lastMcrtProcessDelta: -4 * time.Second,
			lastSuccessSyncDelta: -1 * time.Second,
			wantError:            true,
		},
		{
			description:          "Process in future",
			prevMcrtScheduled:    true,
			prevSuccessSyncDelta: -3 * time.Second,
			lastMcrtProcessDelta: 1 * time.Second,
			lastSuccessSyncDelta: -1 * time.Second,
			wantError:            false,
		},
		{
			description:          "No process",
			prevMcrtScheduled:    true,
			prevSuccessSyncDelta: -3 * time.Second,
			lastMcrtProcessDelta: -999999 * time.Second,
			lastSuccessSyncDelta: -1 * time.Second,
			wantError:            true,
		},
		{
			description:          "Process in between, no ManagedCertificate scheduled",
			prevMcrtScheduled:    false,
			prevSuccessSyncDelta: -3 * time.Second,
			lastMcrtProcessDelta: -2 * time.Second,
			lastSuccessSyncDelta: -1 * time.Second,
			wantError:            false,
		},
		{
			description:          "Process before previous SynchronizeAll (or no process), no ManagedCertificate scheduled",
			prevMcrtScheduled:    false,
			prevSuccessSyncDelta: -3 * time.Second,
			lastMcrtProcessDelta: -4 * time.Second,
			lastSuccessSyncDelta: -1 * time.Second,
			wantError:            false,
		},
		{
			description:          "Process in future, no ManagedCertificate scheduled",
			prevMcrtScheduled:    false,
			prevSuccessSyncDelta: -3 * time.Second,
			lastMcrtProcessDelta: 1 * time.Second,
			lastSuccessSyncDelta: -1 * time.Second,
			wantError:            false,
		},
		{
			description:          "No process, no ManagedCertificate scheduled",
			prevMcrtScheduled:    false,
			prevSuccessSyncDelta: -3 * time.Second,
			lastMcrtProcessDelta: -999999 * time.Second,
			lastSuccessSyncDelta: -1 * time.Second,
			wantError:            false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
			now := time.Now()
			healthCheck.prevSuccessSync.mcrtScheduled = tc.prevMcrtScheduled
			healthCheck.prevSuccessSync.runTime = now.Add(tc.prevSuccessSyncDelta)
			healthCheck.lastActivity[McrtResyncProcess] = now.Add(tc.lastMcrtProcessDelta)
			healthCheck.lastSuccessSync.runTime = now.Add(tc.lastSuccessSyncDelta)

			err := healthCheck.checkMcrtQueueHealth()
			if tc.wantError && err == nil {
				t.Fatalf("checkMcrtQueueHealth() returned nil, want error")
			} else if !tc.wantError && err != nil {
				t.Fatalf("checkMcrtQueueHealth() returned error, want nil")
			}
		})
	}
}

func TestUpdateLastActivity(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description           string
		lastActivityDelta     time.Duration
		newActivityDelta      time.Duration
		wantLastActivityDelta time.Duration
	}{
		{
			description:           "New activity after last activity",
			lastActivityDelta:     -1 * time.Second,
			newActivityDelta:      0,
			wantLastActivityDelta: 0,
		},
		{
			description:           "New activity before last activity",
			lastActivityDelta:     -1 * time.Second,
			newActivityDelta:      -2 * time.Second,
			wantLastActivityDelta: -1 * time.Second,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
			now := time.Now()
			activityName := IngressResyncProcess // The other activities should behave in the same way
			healthCheck.lastActivity[activityName] = now.Add(tc.lastActivityDelta)

			healthCheck.UpdateLastActivity(activityName, now.Add(tc.newActivityDelta))

			lastActivity := healthCheck.lastActivity[activityName]
			wantLastActivity := now.Add(tc.wantLastActivityDelta)
			if lastActivity != wantLastActivity {
				t.Fatalf("healthCheck.lastActivity %s, want %s", lastActivity, wantLastActivity)
			}
		})
	}
}

func TestUpdateLastSuccessSync(t *testing.T) {
	t.Parallel()

	now := time.Now()

	testCases := []struct {
		description         string
		lastSuccessSync     syncDetails
		prevSuccessSync     syncDetails
		lastSync            time.Time
		newSuccessSync      syncDetails
		wantLastSuccessSync syncDetails
		wantPrevSuccessSync syncDetails
		wantLastSync        time.Time
	}{
		{
			description:     "Happy path, new activity after last activity",
			prevSuccessSync: syncDetails{runTime: now.Add(-2 * time.Second)},
			lastSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second)},
			lastSync:        now.Add(-1 * time.Second),
			newSuccessSync:  syncDetails{runTime: now},

			wantPrevSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second)},
			wantLastSuccessSync: syncDetails{runTime: now},
			wantLastSync:        now,
		},
		{
			description:     "New activity before last activity",
			prevSuccessSync: syncDetails{runTime: now.Add(-3 * time.Second)},
			lastSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second)},
			lastSync:        now.Add(-1 * time.Second),
			newSuccessSync:  syncDetails{runTime: now.Add(-2 * time.Second)},

			wantPrevSuccessSync: syncDetails{runTime: now.Add(-3 * time.Second)},
			wantLastSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second)},
			wantLastSync:        now.Add(-1 * time.Second),
		},
		{
			description:     "New activity before previous activity",
			prevSuccessSync: syncDetails{runTime: now.Add(-3 * time.Second)},
			lastSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second)},
			lastSync:        now.Add(-1 * time.Second),
			newSuccessSync:  syncDetails{runTime: now.Add(-5 * time.Second)},

			wantPrevSuccessSync: syncDetails{runTime: now.Add(-3 * time.Second)},
			wantLastSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second)},
			wantLastSync:        now.Add(-1 * time.Second),
		},
		{
			description:     "Happy path, new activity after last activity, schedule objects for processing",
			prevSuccessSync: syncDetails{runTime: now.Add(-2 * time.Second), ingressScheduled: true},
			lastSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second), mcrtScheduled: true},
			lastSync:        now.Add(-1 * time.Second),
			newSuccessSync:  syncDetails{runTime: now, ingressScheduled: true},

			wantPrevSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second), mcrtScheduled: true},
			wantLastSuccessSync: syncDetails{runTime: now, ingressScheduled: true},
			wantLastSync:        now,
		},
		{
			description:     "New activity before last activity, schedule objects for processing",
			prevSuccessSync: syncDetails{runTime: now.Add(-3 * time.Second), ingressScheduled: true},
			lastSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second), mcrtScheduled: true},
			lastSync:        now.Add(-1 * time.Second),
			newSuccessSync:  syncDetails{runTime: now.Add(-2 * time.Second), ingressScheduled: true},

			wantPrevSuccessSync: syncDetails{runTime: now.Add(-3 * time.Second), ingressScheduled: true},
			wantLastSuccessSync: syncDetails{runTime: now.Add(-1 * time.Second), mcrtScheduled: true},
			wantLastSync:        now.Add(-1 * time.Second),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
			healthCheck.prevSuccessSync = tc.prevSuccessSync
			healthCheck.lastSuccessSync = tc.lastSuccessSync
			healthCheck.lastActivity[SynchronizeAll] = tc.lastSync

			healthCheck.UpdateLastSuccessSync(
				tc.newSuccessSync.runTime,
				tc.newSuccessSync.ingressScheduled,
				tc.newSuccessSync.mcrtScheduled)

			if healthCheck.prevSuccessSync != tc.wantPrevSuccessSync {
				t.Fatalf("healthCheck.prevSuccessSync %v, want %v",
					healthCheck.prevSuccessSync, tc.wantPrevSuccessSync)
			}
			if healthCheck.lastSuccessSync != tc.wantLastSuccessSync {
				t.Fatalf("healthCheck.lastSuccessSync %v, want %v",
					healthCheck.lastSuccessSync, tc.wantLastSuccessSync)
			}
			if healthCheck.lastActivity[SynchronizeAll] != tc.wantLastSync {
				t.Fatalf("healthCheck.lastSync %v, want %v",
					healthCheck.lastActivity[SynchronizeAll], tc.wantLastSync)
			}
		})
	}
}

// TestUpdateLastSync makes sure that healthCheck.UpdateLastActivity
// successfully updates SynchronizeAll activity.
func TestUpdateLastSyncHttp(t *testing.T) {
	healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
	address := addressManager.GetNextAddress()

	now := time.Now()
	past := now.Add(timeout * -2)
	future := now.Add(timeout * 10)

	healthCheck.lastActivity[SynchronizeAll] = past
	// Make last success sync in the future because we are not testing it now.
	healthCheck.lastSuccessSync.runTime = future

	healthCheck.StartServing(address, healthCheckPath)
	healthCheck.StartMonitoring()
	t.Cleanup(func() { healthCheck.Stop() })

	assertServeHttpResponseCode(t, healthCheck, 500)
	healthCheck.UpdateLastActivity(SynchronizeAll, now)
	assertServeHttpResponseCode(t, healthCheck, 200)
}

// TestUpdateLastSuccessSync makes sure that healthCheck.UpdateLastSuccessSync
// successfully updates last successful SynchronizeAll.
func TestUpdateLastSuccessSyncHttp(t *testing.T) {
	healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
	address := addressManager.GetNextAddress()

	now := time.Now()
	past := now.Add(timeout * -2)
	healthCheck.lastActivity[SynchronizeAll] = past
	healthCheck.lastSuccessSync.runTime = past

	healthCheck.StartServing(address, healthCheckPath)
	healthCheck.StartMonitoring()
	t.Cleanup(func() { healthCheck.Stop() })

	assertServeHttpResponseCode(t, healthCheck, 500)
	healthCheck.UpdateLastSuccessSync(now, false, false)
	assertServeHttpResponseCode(t, healthCheck, 200)
}

// TestUpdateFutureTime makes sure that SynchronizeAll updates do not replace previous
// times if they were from the future. It should only increase time, not decrease it.
func TestUpdateFutureTime(t *testing.T) {
	healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
	address := addressManager.GetNextAddress()

	now := time.Now()
	future := now.Add(timeout * 10)
	healthCheck.lastActivity[SynchronizeAll] = future
	healthCheck.lastSuccessSync.runTime = future

	healthCheck.StartServing(address, healthCheckPath)
	healthCheck.StartMonitoring()
	t.Cleanup(func() { healthCheck.Stop() })

	assertServeHttpResponseCode(t, healthCheck, 200)
	healthCheck.UpdateLastSuccessSync(now, false, false)
	assertServeHttpResponseCode(t, healthCheck, 200)

	// verify last SynchronizeAll activity timestamp from the future wasn't overwritten.
	changed := healthCheck.lastActivity[SynchronizeAll].Equal(future)
	if !changed {
		t.Fatalf(`lastSyncActivity was %s decreased to %s`,
			future, healthCheck.lastActivity[SynchronizeAll])
	}
	// verify last SynchronizeAll sync from the future wasn't overwritten.
	changed = healthCheck.lastSuccessSync.runTime.Equal(future)
	if !changed {
		t.Fatalf(`lastSyncSuccess was %s decreased to %s`,
			future, healthCheck.lastSuccessSync.runTime)
	}
}

func TestProcessQueueHealthCheckHttp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description      string
		ingressScheduled bool
		mcrtScheduled    bool
		processActivity  []ActivityName
		wantResponseCode int
	}{
		{
			description:      "Ingress queue add and process",
			ingressScheduled: true,
			mcrtScheduled:    false,
			processActivity:  []ActivityName{IngressResyncProcess},
			wantResponseCode: 200,
		},
		{
			description:      "Ingress queue add only",
			ingressScheduled: true,
			mcrtScheduled:    false,
			wantResponseCode: 500,
		},
		{
			description:      "Ingress queue process only",
			ingressScheduled: false,
			mcrtScheduled:    false,
			processActivity:  []ActivityName{IngressResyncProcess},
			wantResponseCode: 200,
		},
		{
			description:      "ManagedCertificate queue add and process",
			ingressScheduled: false,
			mcrtScheduled:    true,
			processActivity:  []ActivityName{McrtResyncProcess},
			wantResponseCode: 200,
		},
		{
			description:      "ManagedCertificate queue add only",
			ingressScheduled: false,
			mcrtScheduled:    true,
			wantResponseCode: 500,
		},
		{
			description:      "ManagedCertificate queue process only",
			ingressScheduled: false,
			mcrtScheduled:    false,
			processActivity:  []ActivityName{IngressResyncProcess},
			wantResponseCode: 200,
		},
		{
			description:      "No add, no process",
			ingressScheduled: false,
			mcrtScheduled:    false,
			wantResponseCode: 200,
		},
		{
			description:      "Add both, process one",
			ingressScheduled: true,
			mcrtScheduled:    true,
			processActivity:  []ActivityName{McrtResyncProcess},
			wantResponseCode: 500,
		},
		{
			description:      "Add both, process both",
			ingressScheduled: true,
			mcrtScheduled:    true,
			processActivity:  []ActivityName{IngressResyncProcess, McrtResyncProcess},
			wantResponseCode: 200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			healthCheck := NewHealthCheck(healthCheckInterval, timeout, timeout)
			address := addressManager.GetNextAddress()

			healthCheck.StartServing(address, healthCheckPath)
			healthCheck.StartMonitoring()
			t.Cleanup(func() { healthCheck.Stop() })

			healthCheck.UpdateLastSuccessSync(now.Add(-30*time.Millisecond), tc.ingressScheduled, tc.mcrtScheduled)
			for _, activityName := range tc.processActivity {
				healthCheck.UpdateLastActivity(activityName, now.Add(-20*time.Millisecond))
			}
			healthCheck.UpdateLastSuccessSync(now.Add(-10*time.Millisecond), false, false)
			assertServeHttpResponseCode(t, healthCheck, tc.wantResponseCode)
		})
	}
}

func assertServeHttpResponseCode(t *testing.T, healthCheck *HealthCheck, wantCode int) {
	t.Helper()
	req := httptest.NewRequest("GET", healthCheckPath, nil)
	w := httptest.NewRecorder()
	time.Sleep(2 * healthCheckInterval) // wait for a health check loop cycle
	healthCheck.serveHTTP(w, req)
	assertResponseCode(t, w.Code, wantCode)
}

func assertResponseCode(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("Response code %d, want %d", got, want)
	}
}
