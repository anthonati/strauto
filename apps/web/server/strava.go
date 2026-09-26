package strauto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 8 * time.Second}

type stravaToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	Scope        string `json:"scope"`
	Athlete      struct {
		ID        int64  `json:"id"`
		FirstName string `json:"firstname"`
		LastName  string `json:"lastname"`
	} `json:"athlete"`
}

func tokenRequest(ctx context.Context, c config, fields url.Values) (stravaToken, error) {
	fields.Set("client_id", c.clientID)
	fields.Set("client_secret", c.clientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.strava.com/oauth/token", strings.NewReader(fields.Encode()))
	if err != nil {
		return stravaToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return stravaToken{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return stravaToken{}, fmt.Errorf("Strava token request returned %d", resp.StatusCode)
	}
	var token stravaToken
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&token); err != nil {
		return stravaToken{}, err
	}
	if token.AccessToken == "" || token.RefreshToken == "" || token.ExpiresAt == 0 {
		return stravaToken{}, fmt.Errorf("incomplete Strava token response")
	}
	return token, nil
}

func stravaActivity(ctx context.Context, accessToken string, id int64) (activity, error) {
	var a activity
	if err := stravaAPI(ctx, http.MethodGet, accessToken, id, nil, &a); err != nil {
		return a, err
	}
	return a, nil
}

func muteActivity(ctx context.Context, accessToken string, id int64) error {
	return stravaAPI(ctx, http.MethodPut, accessToken, id, []byte(`{"hide_from_home":true}`), nil)
}

func stravaAPI(ctx context.Context, method, accessToken string, id int64, body []byte, result any) error {
	url := "https://www.strava.com/api/v3/activities/" + strconv.FormatInt(id, 10)
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Strava activity request returned %d", resp.StatusCode)
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result)
	}
	return nil
}

type activity struct {
	ID           int64  `json:"id"`
	SportType    string `json:"sport_type"`
	HideFromHome bool   `json:"hide_from_home"`
	Athlete      struct {
		ID int64 `json:"id"`
	} `json:"athlete"`
}
