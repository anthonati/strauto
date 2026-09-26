package handler

import (
	"net/http"

	"strauto/server"
)

func Disconnect(w http.ResponseWriter, r *http.Request) { strauto.Disconnect(w, r) }
