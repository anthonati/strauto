package handler

import (
	"net/http"

	"strauto/server"
)

func Automation(w http.ResponseWriter, r *http.Request) { strauto.Automation(w, r) }
