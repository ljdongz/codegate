package session

import (
	"testing"
	"time"
)

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal string", "normal string"},
		{"it's a test", `it'\''s a test`},
		{"", ""},
	}
	for _, tt := range tests {
		got := shellEscape(tt.input)
		if got != tt.want {
			t.Errorf("shellEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	t.Run("valid unix timestamp", func(t *testing.T) {
		got, err := parseTimestamp("1700000000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := time.Unix(1700000000, 0)
		if !got.Equal(want) {
			t.Errorf("parseTimestamp(%q) = %v, want %v", "1700000000", got, want)
		}
	})

	t.Run("invalid string", func(t *testing.T) {
		_, err := parseTimestamp("not-a-number")
		if err == nil {
			t.Error("expected error for invalid timestamp, got nil")
		}
	})
}

func TestNewManager(t *testing.T) {
	allowedUsers := []int64{123, 456}
	m := NewManager("bot-token", allowedUsers, true)

	if m.claudeBotToken != "bot-token" {
		t.Errorf("claudeBotToken = %q, want %q", m.claudeBotToken, "bot-token")
	}
	if len(m.allowedUsers) != 2 || m.allowedUsers[0] != 123 || m.allowedUsers[1] != 456 {
		t.Errorf("allowedUsers = %v, want [123 456]", m.allowedUsers)
	}
	if !m.skipPermissions {
		t.Error("skipPermissions = false, want true")
	}
}
