package handler

import (
	"net/http"

	"strauto/server"
)

func Webhook(w http.ResponseWriter, r *http.Request) { strauto.Webhook(w, r) }
