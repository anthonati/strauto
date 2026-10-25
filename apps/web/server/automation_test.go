package strauto

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, activity, update, outcome          string
		fetchStatus, updateStatus, historyStatus int
		checked, wantUpdate, wantError           bool
	}{
		{"nonmatching", `{"id":2,"sport_type":"Run","athlete":{"id":1}}`, "", "skipped", 200, 200, 200, true, false, false},
		{"already muted", `{"id":2,"sport_type":"WeightTraining","hide_from_home":true,"athlete":{"id":1}}`, "", "skipped", 200, 200, 200, true, false, false},
		{"wrong owner", `{"id":2,"sport_type":"WeightTraining","athlete":{"id":3}}`, "", "failed", 200, 200, 200, false, false, true},
		{"wrong activity", `{"id":3,"sport_type":"WeightTraining","athlete":{"id":1}}`, "", "failed", 200, 200, 200, false, false, true},
		{"fetch failed", `{}`, "", "failed", 429, 200, 200, false, false, true},
		{"update failed", `{"id":2,"sport_type":"WeightTraining","athlete":{"id":1}}`, `{}`, "failed", 200, 500, 200, true, true, true},
		{"update unconfirmed", `{"id":2,"sport_type":"WeightTraining","athlete":{"id":1}}`, `{"id":2,"hide_from_home":false,"athlete":{"id":1}}`, "failed", 200, 200, 200, true, true, true},
		{"updated", `{"id":2,"sport_type":"WeightTraining","athlete":{"id":1}}`, `{"id":2,"hide_from_home":true,"athlete":{"id":1}}`, "applied", 200, 200, 200, true, true, false},
		{"history unavailable", `{"id":2,"sport_type":"Run","athlete":{"id":1}}`, "", "skipped", 200, 200, 503, true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testConfig(t)
			c, _ := readConfig()
			previous := httpClient
			t.Cleanup(func() { httpClient = previous })
			updates, records := 0, 0
			httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				body, status := "null", 200
				switch {
				case req.URL.Path == "/rest/v1/athletes":
					body = `[{"id":1,"access_token":"access","expires_at":9999999999,"automation_subscriptions":[{"id":11,"rule_type":"mute_weight_training","enabled":true}]}]`
				case req.URL.Path == "/rest/v1/rpc/record_automation_run":
					records++
					var run map[string]any
					if err := json.NewDecoder(req.Body).Decode(&run); err != nil {
						t.Fatal(err)
					}
					if run["p_owner_id"] != float64(1) || run["p_subscription_id"] != float64(11) || run["p_activity_id"] != float64(2) || run["p_checked"] != tc.checked || run["p_outcome"] != tc.outcome {
						t.Errorf("unexpected run: %v", run)
					}
					status = tc.historyStatus
				case req.URL.Host == "www.strava.com" && req.Method == http.MethodGet:
					body, status = tc.activity, tc.fetchStatus
				case req.URL.Host == "www.strava.com" && req.Method == http.MethodPut:
					updates++
					body, status = tc.update, tc.updateStatus
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			err := processEvent(httptest.NewRequest(http.MethodPost, "/api/process_events", nil), c, queuedEvent{OwnerID: 1, ActivityID: 2})
			if (err != nil) != tc.wantError || (updates == 1) != tc.wantUpdate || records != 1 {
				t.Fatalf("error=%v updates=%d history records=%d", err, updates, records)
			}
		})
	}
}

func TestAutomationDashboardUsesSessionOwner(t *testing.T) {
	testConfig(t)
	c, _ := readConfig()
	previous := httpClient
	t.Cleanup(func() { httpClient = previous })
	calls := 0
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body := ""
		switch req.URL.Path {
		case "/rest/v1/athletes":
			if req.URL.Query().Get("id") != "eq.42" {
				t.Fatalf("incorrect athlete filter: %s", req.URL)
			}
			body = `[{"id":42,"access_token":"do-not-expose","automation_subscriptions":[{"id":11,"rule_type":"mute_weight_training","enabled":true}]}]`
		case "/rest/v1/rpc/automation_dashboard":
			var params map[string]int64
			if err := json.NewDecoder(req.Body).Decode(&params); err != nil || params["p_owner_id"] != 42 {
				t.Fatalf("incorrect owner: %v (%v)", params, err)
			}
			body = `[{"id":11,"rule_type":"mute_weight_training","enabled":true,"activities_checked":4,"activities_updated":2,"failed_activities":1,"last_run_at":"2026-09-27T10:00:00Z","last_outcome":"failed"}]`
		default:
			t.Fatalf("dashboard made an unexpected request: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	unauth := httptest.NewRecorder()
	Automation(unauth, httptest.NewRequest(http.MethodGet, "/api/automation", nil))
	if unauth.Code != 401 || calls != 0 {
		t.Fatal("unauthenticated request reached the database")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/automation?owner_id=99", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: newSession(c, 42)})
	response := httptest.NewRecorder()
	Automation(response, req)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"activities_updated":2`) || strings.Contains(response.Body.String(), "do-not-expose") || calls != 2 {
		t.Fatalf("unexpected dashboard: %d %s (calls=%d)", response.Code, response.Body.String(), calls)
	}
}
