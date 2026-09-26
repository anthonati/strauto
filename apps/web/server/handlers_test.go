package strauto

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testConfig(t *testing.T) {
	t.Helper()
	t.Setenv("APP_URL", "http://localhost:5173")
	t.Setenv("STRAVA_CLIENT_ID", "123")
	t.Setenv("STRAVA_CLIENT_SECRET", "test-secret")
	t.Setenv("SUPABASE_URL", "https://example.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "test-key")
	t.Setenv("SESSION_SECRET", strings.Repeat("x", 32))
	t.Setenv("STRAVA_WEBHOOK_VERIFY_TOKEN", "verify-token")
	t.Setenv("STRAVA_WEBHOOK_SECRET", "callback-secret")
	t.Setenv("STRAVA_WEBHOOK_SUBSCRIPTION_ID", "9")
}

func TestOAuthStartAndStateRejection(t *testing.T) {
	testConfig(t)
	start := httptest.NewRecorder()
	AuthStart(start, httptest.NewRequest(http.MethodGet, "/api/auth_start", nil))
	if start.Code != http.StatusFound {
		t.Fatalf("start status = %d", start.Code)
	}
	location, err := url.Parse(start.Header().Get("Location"))
	if err != nil || location.Host != "www.strava.com" || location.Query().Get("scope") != "activity:read,activity:write" {
		t.Fatalf("unexpected authorization URL: %s", start.Header().Get("Location"))
	}
	if len(start.Result().Cookies()) != 1 || start.Result().Cookies()[0].Value != location.Query().Get("state") {
		t.Fatal("OAuth state cookie did not match redirect state")
	}
	callback := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth_callback?state=wrong&code=secret-code", nil)
	req.AddCookie(start.Result().Cookies()[0])
	OauthCallback(callback, req)
	if callback.Code != http.StatusBadRequest || strings.Contains(callback.Body.String(), "secret-code") {
		t.Fatalf("invalid state response: %d %s", callback.Code, callback.Body.String())
	}
}

func TestSessionRejectsTampering(t *testing.T) {
	testConfig(t)
	c, err := readConfig()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: newSession(c, 42)})
	if id, ok := sessionID(req, c); !ok || id != 42 {
		t.Fatal("valid session rejected")
	}
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: newSession(c, 42) + "x"})
	if _, ok := sessionID(req, c); ok {
		t.Fatal("tampered session accepted")
	}
}

func TestWebhookVerificationRequiresSecret(t *testing.T) {
	testConfig(t)
	bad := httptest.NewRecorder()
	Webhook(bad, httptest.NewRequest(http.MethodGet, "/api/webhook?hub.mode=subscribe&hub.challenge=abc&hub.verify_token=verify-token", nil))
	if bad.Code != http.StatusForbidden {
		t.Fatalf("webhook without secret: %d", bad.Code)
	}
	good := httptest.NewRecorder()
	Webhook(good, httptest.NewRequest(http.MethodGet, "/api/webhook?key=callback-secret&hub.mode=subscribe&hub.challenge=abc&hub.verify_token=verify-token", nil))
	if good.Code != http.StatusOK || !strings.Contains(good.Body.String(), `"hub.challenge":"abc"`) {
		t.Fatalf("webhook verification: %d %s", good.Code, good.Body.String())
	}
}

func TestMuteRuleOnlyTargetsOwnedWeightTraining(t *testing.T) {
	event := queuedEvent{OwnerID: 1, ActivityID: 2}
	a := activity{ID: 2, SportType: "WeightTraining"}
	a.Athlete.ID = 1
	if !shouldMute(a, event) {
		t.Fatal("expected weight training upload to be muted")
	}
	a.HideFromHome = true
	if shouldMute(a, event) {
		t.Fatal("already muted activity selected")
	}
	a.HideFromHome = false
	a.SportType = "Run"
	if shouldMute(a, event) {
		t.Fatal("run selected")
	}
	a.SportType = "WeightTraining"
	a.Athlete.ID = 3
	if shouldMute(a, event) {
		t.Fatal("another athlete's activity selected")
	}
}

func TestProcessEventMutesMatchingStravaActivity(t *testing.T) {
	testConfig(t)
	c, err := readConfig()
	if err != nil {
		t.Fatal(err)
	}
	previous := httpClient
	t.Cleanup(func() { httpClient = previous })
	muted := false
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := ""
		status := http.StatusOK
		switch {
		case req.URL.Host == "example.supabase.co" && req.Method == http.MethodGet:
			body = `[{"id":1,"access_token":"access","refresh_token":"refresh","expires_at":9999999999,"mute_weight_training":true}]`
		case req.URL.Host == "www.strava.com" && req.Method == http.MethodGet:
			body = `{"id":2,"sport_type":"WeightTraining","hide_from_home":false,"athlete":{"id":1}}`
		case req.URL.Host == "www.strava.com" && req.Method == http.MethodPut:
			payload, readErr := io.ReadAll(req.Body)
			if readErr != nil || string(payload) != `{"hide_from_home":true}` || req.Header.Get("Authorization") != "Bearer access" {
				t.Errorf("incorrect Strava update: %s", payload)
			}
			muted = true
			body = `{}`
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	if err := processEvent(httptest.NewRequest(http.MethodPost, "/api/process_events", nil), c, queuedEvent{OwnerID: 1, ActivityID: 2}); err != nil {
		t.Fatal(err)
	}
	if !muted {
		t.Fatal("matching activity was not muted")
	}
}
