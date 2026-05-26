package entities

import (
	"encoding/json"
)

type WebhookConfig struct {
	URL     string            `json:"url"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
}

func (w *WebhookConfig) IsValid() bool {
	return w.URL != ""
}

func (w *WebhookConfig) ToJSON() ([]byte, error) {
	return json.Marshal(w)
}

func WebhookConfigFromJSON(data []byte) (*WebhookConfig, error) {
	var config WebhookConfig
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
