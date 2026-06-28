package main

import (
	"net/http"

	"github.com/imphungky/teamshook/internal/webhook"
)

func AddRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/webhook", webhook.HandleGithubPushEvent)
}
