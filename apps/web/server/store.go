package strauto

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type athlete struct {
	ID                 int64                    `json:"id"`
	FirstName          string                   `json:"first_name"`
	LastName           string                   `json:"last_name"`
	AccessToken        string                   `json:"access_token"`
	RefreshToken       string                   `json:"refresh_token"`
	ExpiresAt          int64                    `json:"expires_at"`
	GrantedScopes      string                   `json:"granted_scopes"`
	MuteWeightTraining bool                     `json:"mute_weight_training"`
	Subscriptions      []automationSubscription `json:"automation_subscriptions"`
}

type automationSubscription struct {
	ID       int64  `json:"id"`
	RuleType string `json:"rule_type"`
	Enabled  bool   `json:"enabled"`
}

type automationSummary struct {
	automationSubscription
	ActivitiesChecked int64   `json:"activities_checked"`
	ActivitiesUpdated int64   `json:"activities_updated"`
	FailedActivities  int64   `json:"failed_activities"`
	LastRunAt         *string `json:"last_run_at"`
	LastOutcome       *string `json:"last_outcome"`
}

func (a athlete) muteSubscription() automationSubscription {
	for _, subscription := range a.Subscriptions {
		if subscription.RuleType == "mute_weight_training" {
			return subscription
		}
	}
	return automationSubscription{}
}

type queuedEvent struct {
	ID         int64 `json:"id"`
	OwnerID    int64 `json:"owner_id"`
	ActivityID int64 `json:"activity_id"`
	Attempts   int   `json:"attempts"`
}

var errAthleteNotFound = errors.New("athlete not found")

func dbRequest(ctx context.Context, c config, method, path string, body any, headers map[string]string, result any) (int, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.supabaseURL+"/rest/v1/"+path, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	req.Header.Set("apikey", c.supabaseKey)
	if !strings.HasPrefix(c.supabaseKey, "sb_secret_") {
		// Legacy service-role keys are JWTs; current secret keys are not.
		req.Header.Set("Authorization", "Bearer "+c.supabaseKey)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("database request returned %d", resp.StatusCode)
	}
	if result != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func getAthlete(ctx context.Context, c config, id int64) (athlete, error) {
	var rows []athlete
	path := "athletes?id=eq." + strconv.FormatInt(id, 10) + "&select=*,automation_subscriptions(*)&limit=1"
	_, err := dbRequest(ctx, c, http.MethodGet, path, nil, nil, &rows)
	if err != nil {
		return athlete{}, err
	}
	if len(rows) == 0 {
		return athlete{}, errAthleteNotFound
	}
	a := rows[0]
	a.MuteWeightTraining = a.muteSubscription().Enabled
	return a, nil
}

func automationDashboard(ctx context.Context, c config, ownerID int64) ([]automationSummary, error) {
	rows := []automationSummary{}
	_, err := dbRequest(ctx, c, http.MethodPost, "rpc/automation_dashboard", map[string]any{"p_owner_id": ownerID}, nil, &rows)
	return rows, err
}

func recordAutomationRun(ctx context.Context, c config, ownerID, subscriptionID, activityID int64, checked bool, outcome string) error {
	_, err := dbRequest(ctx, c, http.MethodPost, "rpc/record_automation_run", map[string]any{
		"p_owner_id": ownerID, "p_subscription_id": subscriptionID,
		"p_activity_id": activityID, "p_checked": checked, "p_outcome": outcome,
	}, nil, nil)
	return err
}

func saveAthlete(ctx context.Context, c config, token stravaToken, scopes string) error {
	row := map[string]any{
		"id": token.Athlete.ID, "first_name": token.Athlete.FirstName,
		"last_name": token.Athlete.LastName, "access_token": token.AccessToken,
		"refresh_token": token.RefreshToken, "expires_at": token.ExpiresAt,
		"granted_scopes": scopes,
	}
	// Omit the subscription flag so reconnecting does not turn a rule off.
	_, err := dbRequest(ctx, c, http.MethodPost, "athletes?on_conflict=id", []any{row}, map[string]string{"Prefer": "resolution=merge-duplicates,return=minimal"}, nil)
	return err
}

func updateTokens(ctx context.Context, c config, id int64, token stravaToken) error {
	path := "athletes?id=eq." + strconv.FormatInt(id, 10)
	_, err := dbRequest(ctx, c, http.MethodPatch, path, map[string]any{
		"access_token": token.AccessToken, "refresh_token": token.RefreshToken, "expires_at": token.ExpiresAt,
	}, nil, nil)
	return err
}

func setMuteRule(ctx context.Context, c config, id int64, enabled bool) error {
	path := "athletes?id=eq." + strconv.FormatInt(id, 10)
	_, err := dbRequest(ctx, c, http.MethodPatch, path, map[string]any{"mute_weight_training": enabled}, nil, nil)
	return err
}

func deleteAthlete(ctx context.Context, c config, id int64) error {
	path := "athletes?id=eq." + strconv.FormatInt(id, 10)
	_, err := dbRequest(ctx, c, http.MethodDelete, path, nil, nil, nil)
	return err
}

func enqueueEvent(ctx context.Context, c config, ownerID, activityID, subscriptionID, eventTime int64) error {
	row := map[string]any{"p_owner_id": ownerID, "p_activity_id": activityID, "p_subscription_id": subscriptionID, "p_event_time": eventTime}
	_, err := dbRequest(ctx, c, http.MethodPost, "rpc/enqueue_activity_event", row, nil, nil)
	return err
}

func claimEvents(ctx context.Context, c config) ([]queuedEvent, error) {
	var events []queuedEvent
	_, err := dbRequest(ctx, c, http.MethodPost, "rpc/claim_activity_events", map[string]any{"batch_size": 3}, nil, &events)
	return events, err
}

func finishEvent(ctx context.Context, c config, event queuedEvent, processErr error) error {
	status := "done"
	nextAttempt := time.Now().UTC()
	if processErr != nil {
		status = "pending"
		if event.Attempts >= 5 {
			status = "failed"
		}
		// Each claim increments attempts. The delay bounds retries during an outage.
		nextAttempt = nextAttempt.Add(time.Duration(1<<min(event.Attempts, 5)) * time.Minute)
	}
	path := "activity_events?id=eq." + strconv.FormatInt(event.ID, 10)
	_, err := dbRequest(ctx, c, http.MethodPatch, path, map[string]any{
		"status": status, "next_attempt_at": nextAttempt.Format(time.RFC3339),
	}, nil, nil)
	return err
}

func revokeToken(ctx context.Context, c config, refreshToken string) error {
	form := url.Values{"token": {refreshToken}, "token_type_hint": {"refresh_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.strava.com/oauth/revoke", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Strava revoke returned %d", resp.StatusCode)
	}
	return nil
}
