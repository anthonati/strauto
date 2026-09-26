package handler

import (
	"net/http"

	"strauto/server"
)

func Me(w http.ResponseWriter, r *http.Request) { strauto.Me(w, r) }
