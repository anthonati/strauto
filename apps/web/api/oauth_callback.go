package handler

import (
	"net/http"

	"strauto/server"
)

func OauthCallback(w http.ResponseWriter, r *http.Request) {
	strauto.OauthCallback(w, r)
}
