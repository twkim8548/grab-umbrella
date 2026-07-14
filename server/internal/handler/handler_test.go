package handler

import (
	"encoding/json"
	"testing"
)

func TestNotificationsEnabledBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "omitted by old client defaults to enabled",
			body: `{}`,
			want: true,
		},
		{
			name: "explicit true enables notifications",
			body: `{"notifications_enabled":true}`,
			want: true,
		},
		{
			name: "explicit false disables notifications",
			body: `{"notifications_enabled":false}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req syncRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if got := notificationsEnabled(req.NotificationsEnabled); got != tt.want {
				t.Fatalf("notificationsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
