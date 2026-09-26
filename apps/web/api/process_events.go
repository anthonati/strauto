package handler

import (
	"net/http"

	"strauto/server"
)

func ProcessEvents(w http.ResponseWriter, r *http.Request) { strauto.ProcessEvents(w, r) }
