package strauto

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkerDistinguishesRetriesFromExhaustedFailures(t *testing.T) {
	testConfig(t)
	previous := httpClient
	t.Cleanup(func() { httpClient = previous })
	saved := map[string]string{}
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, status := "null", http.StatusOK
		switch req.URL.Path {
		case "/rest/v1/rpc/claim_activity_events":
			body = `[{"id":101,"owner_id":1,"activity_id":11,"attempts":1},{"id":102,"owner_id":2,"activity_id":12,"attempts":2},{"id":103,"owner_id":3,"activity_id":13,"attempts":5}]`
		case "/rest/v1/athletes":
			if req.URL.Query().Get("id") == "eq.1" {
				body = `[]` // Disconnected athlete: acknowledge without doing work.
			} else {
				status = http.StatusServiceUnavailable
			}
		case "/rest/v1/activity_events":
			if req.Method != http.MethodPatch {
				t.Fatalf("unexpected queue method: %s", req.Method)
			}
			var update struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(req.Body).Decode(&update); err != nil {
				t.Fatal(err)
			}
			saved[req.URL.Query().Get("id")] = update.Status
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	req := httptest.NewRequest(http.MethodPost, "/api/process_events", nil)
	req.Header.Set("Authorization", "Bearer worker-secret")
	response := httptest.NewRecorder()
	ProcessEvents(response, req)
	var result map[string]int
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || result["processed"] != 1 || result["retrying"] != 1 || result["failed"] != 1 {
		t.Fatalf("worker results: %d %s", response.Code, response.Body.String())
	}
	if saved["eq.101"] != "done" || saved["eq.102"] != "pending" || saved["eq.103"] != "failed" || len(saved) != 3 {
		t.Fatalf("queue statuses disagree with worker result: %v", saved)
	}
}

func TestWorkerDoesNotReportSuccessWhenQueueSaveFails(t *testing.T) {
	testConfig(t)
	previous := httpClient
	t.Cleanup(func() { httpClient = previous })
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, status := "null", http.StatusOK
		switch req.URL.Path {
		case "/rest/v1/rpc/claim_activity_events":
			body = `[{"id":101,"owner_id":1,"activity_id":11,"attempts":1}]`
		case "/rest/v1/athletes":
			body = `[]`
		case "/rest/v1/activity_events":
			status = http.StatusServiceUnavailable
		default:
			t.Fatalf("unexpected request: %s", req.URL.Path)
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	req := httptest.NewRequest(http.MethodPost, "/api/process_events", nil)
	req.Header.Set("Authorization", "Bearer worker-secret")
	response := httptest.NewRecorder()
	ProcessEvents(response, req)
	if response.Code < 500 || strings.Contains(response.Body.String(), `"processed"`) {
		t.Fatalf("failed queue write reported success: %d %s", response.Code, response.Body.String())
	}
}
