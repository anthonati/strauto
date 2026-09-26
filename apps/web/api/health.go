package handler

import (
	"net/http"

	"strauto/server"
)

func Health(w http.ResponseWriter, r *http.Request) {
	strauto.Health(w, r)
}
