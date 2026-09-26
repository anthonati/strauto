package strauto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const sessionCookie = "strauto_session"
const stateCookie = "strauto_oauth_state"

type config struct {
	appURL         string
	clientID       string
	clientSecret   string
	supabaseURL    string
	supabaseKey    string
	sessionSecret  string
	webhookToken   string
	webhookSecret  string
	subscriptionID int64
	workerSecret   string
}

func readConfig() (config, error) {
	supabaseKey := os.Getenv("SUPABASE_SECRET_KEY")
	if supabaseKey == "" {
		// Existing projects may still use the legacy service-role JWT.
		supabaseKey = os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	}
	c := config{
		appURL:        strings.TrimRight(os.Getenv("APP_URL"), "/"),
		clientID:      os.Getenv("STRAVA_CLIENT_ID"),
		clientSecret:  os.Getenv("STRAVA_CLIENT_SECRET"),
		supabaseURL:   strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		supabaseKey:   supabaseKey,
		sessionSecret: os.Getenv("SESSION_SECRET"),
		webhookToken:  os.Getenv("STRAVA_WEBHOOK_VERIFY_TOKEN"),
		webhookSecret: os.Getenv("STRAVA_WEBHOOK_SECRET"),
		workerSecret:  os.Getenv("WORKER_SECRET"),
	}
	var missing []string
	for name, value := range map[string]string{
		"APP_URL":              c.appURL,
		"STRAVA_CLIENT_ID":     c.clientID,
		"STRAVA_CLIENT_SECRET": c.clientSecret,
		"SUPABASE_URL":         c.supabaseURL,
		"SUPABASE_SECRET_KEY or SUPABASE_SERVICE_ROLE_KEY": c.supabaseKey,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(c.sessionSecret) < 32 {
		missing = append(missing, "SESSION_SECRET (at least 32 characters)")
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		log.Printf("Strauto server configuration is incomplete: %s", strings.Join(missing, ", "))
		return c, errors.New("server configuration is incomplete")
	}
	u, err := url.Parse(c.appURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return c, errors.New("APP_URL must be HTTPS or local HTTP")
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return c, errors.New("APP_URL must be an origin without a path")
	}
	s, err := url.Parse(c.supabaseURL)
	if err != nil || s.Scheme != "https" || s.Host == "" {
		return c, errors.New("SUPABASE_URL must be HTTPS")
	}
	if raw := os.Getenv("STRAVA_WEBHOOK_SUBSCRIPTION_ID"); raw != "" {
		c.subscriptionID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return c, errors.New("invalid STRAVA_WEBHOOK_SUBSCRIPTION_ID")
		}
	}
	return c, nil
}

func configured(w http.ResponseWriter) (config, bool) {
	c, err := readConfig()
	if err != nil {
		log.Printf("Strauto configuration error: %v", err)
		http.Error(w, "server configuration is incomplete", http.StatusServiceUnavailable)
		return c, false
	}
	return c, true
}

// Health reports whether this deployment has the minimum server configuration.
// It does not make network calls to Strava or Supabase.
func Health(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := configured(w); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "configured"})
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func sameOrigin(r *http.Request, c config) bool {
	return r.Header.Get("Origin") == c.appURL
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func newSession(c config, athleteID int64) string {
	payload := fmt.Sprintf("%d:%d", athleteID, time.Now().Add(30*24*time.Hour).Unix())
	mac := hmac.New(sha256.New, []byte(c.sessionSecret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func sessionID(r *http.Request, c config) (int64, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return 0, false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return 0, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, false
	}
	mac := hmac.New(sha256.New, []byte(c.sessionSecret))
	_, _ = mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return 0, false
	}
	fields := strings.Split(string(payload), ":")
	if len(fields) != 2 {
		return 0, false
	}
	id, idErr := strconv.ParseInt(fields[0], 10, 64)
	exp, expErr := strconv.ParseInt(fields[1], 10, 64)
	return id, idErr == nil && expErr == nil && id > 0 && exp > time.Now().Unix()
}

func setCookie(w http.ResponseWriter, c config, name, value, path string, age int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: path, MaxAge: age,
		HttpOnly: true, Secure: strings.HasPrefix(c.appURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
}

func safeError(w http.ResponseWriter, err error) {
	// Never send upstream bodies: OAuth codes, tokens and database details may appear there.
	log.Printf("Strauto request failed: %v", err)
	http.Error(w, "service temporarily unavailable", http.StatusBadGateway)
}
