package handler

import (
	"net/http"

	"strauto/server"
)

func AuthStart(w http.ResponseWriter, r *http.Request) { strauto.AuthStart(w, r) }
