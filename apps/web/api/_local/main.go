package main

import (
	"fmt"
	"net/http"

	"strauto/server"
)

func main() {
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "ok") })
	http.HandleFunc("/api/auth_start", strauto.AuthStart)
	http.HandleFunc("/api/oauth_callback", strauto.OauthCallback)
	http.HandleFunc("/api/me", strauto.Me)
	http.HandleFunc("/api/automation", strauto.Automation)
	http.HandleFunc("/api/disconnect", strauto.Disconnect)
	http.HandleFunc("/api/webhook", strauto.Webhook)
	http.HandleFunc("/api/process_events", strauto.ProcessEvents)
	fmt.Println("Starting Strauto API on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
