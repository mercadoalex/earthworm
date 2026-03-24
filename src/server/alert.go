package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// AlertDispatcher sends alerts via webhook and WebSocket broadcast.
type AlertDispatcher struct {
	webhookURL  string
	wsBroadcast func(Alert)
	httpClient  *http.Client
}

// NewAlertDispatcher creates a new dispatcher.
func NewAlertDispatcher(webhookURL string, wsBroadcast func(Alert)) *AlertDispatcher {
	return &AlertDispatcher{
		webhookURL:  webhookURL,
		wsBroadcast: wsBroadcast,
		httpClient:  &http.Client{Timeout: 5 * time.Second},
	}
}

// Dispatch sends the alert to the webhook (if configured) and broadcasts to WS clients.
func (d *AlertDispatcher) Dispatch(alert Alert) {
	// Always broadcast to WebSocket clients
	if d.wsBroadcast != nil {
		d.wsBroadcast(alert)
	}

	// Send to webhook if configured
	if d.webhookURL != "" {
		go func() {
			body, err := json.Marshal(alert)
			if err != nil {
				log.Printf("Failed to marshal alert for webhook: %v", err)
				return
			}
			resp, err := d.httpClient.Post(d.webhookURL, "application/json", bytes.NewReader(body))
			if err != nil {
				log.Printf("Failed to send alert to webhook %s: %v", d.webhookURL, err)
				return
			}
			resp.Body.Close()
		}()
	}
}
