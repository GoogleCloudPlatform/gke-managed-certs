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
)

func getTestResponse(start time.Time, activityTimeout, successTimeout time.Duration, enabled bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/health-check", nil)
	w := httptest.NewRecorder()
	healthCheck := NewHealthCheck(activityTimeout, successTimeout)
	if enabled {
		healthCheck.StartMonitoring()
	}
	healthCheck.lastActivity = start
	healthCheck.lastSuccessfulRun = start
	healthCheck.ServeHTTP(w, req)
	return w
}

func TestOkServeHTTP(t *testing.T) {
	w := getTestResponse(time.Now(), time.Second, time.Second, true)
	if w.Code != 200 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}
}

func TestFailTimeoutServeHTTP(t *testing.T) {
	w := getTestResponse(time.Now().Add(time.Second*-2), time.Second, time.Second, true)
	if w.Code != 500 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}
}

func TestMonitoringOffAfterTimeout(t *testing.T) {
	w := getTestResponse(time.Now().Add(time.Second*-2), time.Second, time.Second, false)
	if w.Code != 200 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}
}

func TestMonitoringOffBeforeTimeout(t *testing.T) {
	w := getTestResponse(time.Now().Add(time.Second*2), time.Second, time.Second, false)
	if w.Code != 200 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}
}

func TestUpdateLastSync(t *testing.T) {
	timeout := time.Second
	start := time.Now().Add(timeout * -2)
	// to make sure it doesn't cause health check failure
	lastSuccess := time.Now().Add(timeout * 10)

	req := httptest.NewRequest("GET", "/health-check", nil)
	healthCheck := NewHealthCheck(timeout, timeout)
	healthCheck.StartMonitoring()
	healthCheck.lastActivity = start
	healthCheck.lastSuccessfulRun = lastSuccess

	w := httptest.NewRecorder()
	healthCheck.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}

	w = httptest.NewRecorder()
	healthCheck.UpdateLastSync(time.Now())
	healthCheck.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}
}

func TestUpdateActivityAtUpdateLastSuccessfulSync(t *testing.T) {
	timeout := time.Second
	start := time.Now().Add(timeout * -2)
	// to make sure it doesn't cause health check failure
	lastSuccess := time.Now().Add(timeout * 10)

	req := httptest.NewRequest("GET", "/health-check", nil)
	healthCheck := NewHealthCheck(timeout, timeout)
	healthCheck.StartMonitoring()
	healthCheck.lastActivity = start
	healthCheck.lastSuccessfulRun = lastSuccess

	w := httptest.NewRecorder()
	healthCheck.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}

	w = httptest.NewRecorder()
	healthCheck.UpdateLastSuccessfulSync(time.Now())
	healthCheck.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}

	// verify last successful run from the future wasn't overwritten
	result := healthCheck.lastSuccessfulRun.After(healthCheck.lastActivity)
	if !result {
		t.Fatalf(`lastSuccessfulRun %v not after lastActivity %v,
			lastSuccessfulRun decreased`, healthCheck.lastSuccessfulRun,
			healthCheck.lastActivity)
	}
}

func TestUpdateLastSuccessfulRun(t *testing.T) {
	timeout := time.Second
	start := time.Now().Add(timeout * -2)
	// to make sure it doesn't cause health check failure
	lastActivity := time.Now().Add(timeout * 10)

	req := httptest.NewRequest("GET", "/health-check", nil)
	healthCheck := NewHealthCheck(timeout, timeout)
	healthCheck.StartMonitoring()
	healthCheck.lastActivity = lastActivity
	healthCheck.lastSuccessfulRun = start

	w := httptest.NewRecorder()
	healthCheck.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}

	w = httptest.NewRecorder()
	healthCheck.UpdateLastSuccessfulSync(time.Now())
	healthCheck.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("Diff response code %d, expected 200", w.Code)
	}

	// verify last activity timestamp from the future wasn't overwritten
	result := healthCheck.lastActivity.After(healthCheck.lastSuccessfulRun)
	if !result {
		t.Fatalf(`lastActivity %v not after lastSuccessfulRun %v,
			lastActivity decreased`, healthCheck.lastActivity, healthCheck.lastSuccessfulRun)
	}
}
