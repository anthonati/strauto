package strauto

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func AuthStart(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	c, ok := configured(w)
	if !ok {
		return
	}
	state, err := randomToken()
	if err != nil {
		safeError(w, err)
		return
	}
	setCookie(w, c, stateCookie, state, "/api/oauth_callback", 600)
	params := url.Values{
		"client_id": {c.clientID}, "redirect_uri": {c.appURL + "/api/oauth_callback"},
		"response_type": {"code"}, "approval_prompt": {"auto"},
		"scope": {"activity:read,activity:write"}, "state": {state},
	}
	http.Redirect(w, r, "https://www.strava.com/oauth/authorize?"+params.Encode(), http.StatusFound)
}

func OauthCallback(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	c, ok := configured(w)
	if !ok {
		return
	}
	state, cookieErr := r.Cookie(stateCookie)
	setCookie(w, c, stateCookie, "", "/api/oauth_callback", -1)
	gotState := r.URL.Query().Get("state")
	if cookieErr != nil || gotState == "" || subtle.ConstantTimeCompare([]byte(state.Value), []byte(gotState)) != 1 {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, c.appURL+"/?error=access_denied", http.StatusFound)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}
	token, err := tokenRequest(r.Context(), c, url.Values{"grant_type": {"authorization_code"}, "code": {code}})
	if err != nil {
		safeError(w, err)
		return
	}
	scopes := r.URL.Query().Get("scope")
	if token.Scope != "" {
		scopes = token.Scope
	}
	if token.Athlete.ID <= 0 || !hasScopes(scopes, "activity:read", "activity:write") {
		_ = revokeToken(r.Context(), c, token.RefreshToken)
		http.Redirect(w, r, c.appURL+"/?error=missing_scopes", http.StatusFound)
		return
	}
	if err := saveAthlete(r.Context(), c, token, scopes); err != nil {
		_ = revokeToken(r.Context(), c, token.RefreshToken)
		safeError(w, err)
		return
	}
	setCookie(w, c, sessionCookie, newSession(c, token.Athlete.ID), "/", 30*24*3600)
	http.Redirect(w, r, c.appURL+"/", http.StatusFound)
}

func hasScopes(granted string, required ...string) bool {
	set := make(map[string]bool)
	for _, scope := range strings.FieldsFunc(granted, func(r rune) bool { return r == ' ' || r == ',' }) {
		set[scope] = true
	}
	for _, scope := range required {
		if !set[scope] {
			return false
		}
	}
	return true
}

func authenticatedAthlete(w http.ResponseWriter, r *http.Request, c config) (athlete, bool) {
	id, ok := sessionID(r, c)
	if !ok {
		http.Error(w, "not connected", http.StatusUnauthorized)
		return athlete{}, false
	}
	a, err := getAthlete(r.Context(), c, id)
	if err != nil {
		if errors.Is(err, errAthleteNotFound) {
			http.Error(w, "not connected", http.StatusUnauthorized)
		} else {
			safeError(w, err)
		}
		return athlete{}, false
	}
	return a, true
}

func Me(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	c, ok := configured(w)
	if !ok {
		return
	}
	a, ok := authenticatedAthlete(w, r, c)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"first_name": a.FirstName, "last_name": a.LastName,
		"mute_weight_training": a.MuteWeightTraining,
	})
}

func Automation(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	c, ok := configured(w)
	if !ok {
		return
	}
	if !sameOrigin(r, c) {
		http.Error(w, "invalid origin", http.StatusForbidden)
		return
	}
	a, ok := authenticatedAthlete(w, r, c)
	if !ok {
		return
	}
	var body struct {
		MuteWeightTraining *bool `json:"mute_weight_training"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body); err != nil || body.MuteWeightTraining == nil {
		http.Error(w, "invalid automation setting", http.StatusBadRequest)
		return
	}
	if *body.MuteWeightTraining && !c.automationConfigured() {
		http.Error(w, "automation is not configured", http.StatusServiceUnavailable)
		return
	}
	if err := setMuteRule(r.Context(), c, a.ID, *body.MuteWeightTraining); err != nil {
		safeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"mute_weight_training": *body.MuteWeightTraining})
}

func Disconnect(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	c, ok := configured(w)
	if !ok {
		return
	}
	if !sameOrigin(r, c) {
		http.Error(w, "invalid origin", http.StatusForbidden)
		return
	}
	a, ok := authenticatedAthlete(w, r, c)
	if !ok {
		return
	}
	if err := revokeToken(r.Context(), c, a.RefreshToken); err != nil {
		safeError(w, err)
		return
	}
	if err := deleteAthlete(r.Context(), c, a.ID); err != nil {
		safeError(w, err)
		return
	}
	setCookie(w, c, sessionCookie, "", "/", -1)
	w.WriteHeader(http.StatusNoContent)
}

type webhookEvent struct {
	ObjectType     string            `json:"object_type"`
	AspectType     string            `json:"aspect_type"`
	ObjectID       int64             `json:"object_id"`
	OwnerID        int64             `json:"owner_id"`
	SubscriptionID int64             `json:"subscription_id"`
	EventTime      int64             `json:"event_time"`
	Updates        map[string]string `json:"updates"`
}

func Webhook(w http.ResponseWriter, r *http.Request) {
	c, ok := configured(w)
	if !ok {
		return
	}
	if c.webhookSecret == "" || c.webhookToken == "" {
		http.Error(w, "webhook is not configured", http.StatusServiceUnavailable)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("key")), []byte(c.webhookSecret)) != 1 {
		http.Error(w, "invalid webhook key", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		if q.Get("hub.mode") != "subscribe" || subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(c.webhookToken)) != 1 || q.Get("hub.challenge") == "" {
			http.Error(w, "invalid verification", http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"hub.challenge": q.Get("hub.challenge")})
		return
	}
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var event webhookEvent
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&event); err != nil || event.OwnerID <= 0 || event.SubscriptionID <= 0 {
		http.Error(w, "invalid event", http.StatusBadRequest)
		return
	}
	if c.subscriptionID == 0 || event.SubscriptionID != c.subscriptionID {
		http.Error(w, "unknown subscription", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()
	if event.ObjectType == "athlete" && event.AspectType == "update" && event.Updates["authorized"] == "false" {
		if err := deleteAthlete(ctx, c, event.OwnerID); err != nil {
			safeError(w, err)
			return
		}
	} else if event.ObjectType == "activity" && event.AspectType == "create" && event.ObjectID > 0 {
		if err := enqueueEvent(ctx, c, event.OwnerID, event.ObjectID, event.SubscriptionID, event.EventTime); err != nil {
			safeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func ProcessEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	c, ok := configured(w)
	if !ok {
		return
	}
	if c.workerSecret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+c.workerSecret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	events, err := claimEvents(r.Context(), c)
	if err != nil {
		safeError(w, err)
		return
	}
	processed, failed := 0, 0
	for _, event := range events {
		err := processEvent(r, c, event)
		if err != nil {
			failed++
		} else {
			processed++
		}
		if saveErr := finishEvent(r.Context(), c, event, err); saveErr != nil {
			safeError(w, saveErr)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]int{"processed": processed, "retrying": failed})
}

func processEvent(r *http.Request, c config, event queuedEvent) error {
	a, err := getAthlete(r.Context(), c, event.OwnerID)
	if err != nil {
		if errors.Is(err, errAthleteNotFound) {
			return nil
		}
		return err
	}
	if !a.MuteWeightTraining {
		return nil
	}
	if a.ExpiresAt <= time.Now().Add(5*time.Minute).Unix() {
		token, err := tokenRequest(r.Context(), c, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {a.RefreshToken}})
		if err != nil {
			return err
		}
		if err := updateTokens(r.Context(), c, a.ID, token); err != nil {
			return err
		}
		a.AccessToken = token.AccessToken
	}
	activity, err := stravaActivity(r.Context(), a.AccessToken, event.ActivityID)
	if err != nil {
		return err
	}
	if !shouldMute(activity, event) {
		return nil
	}
	return muteActivity(r.Context(), a.AccessToken, event.ActivityID)
}

func shouldMute(activity activity, event queuedEvent) bool {
	return activity.ID == event.ActivityID && activity.Athlete.ID == event.OwnerID &&
		activity.SportType == "WeightTraining" && !activity.HideFromHome
}
