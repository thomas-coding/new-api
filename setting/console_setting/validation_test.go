package console_setting

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestValidateAnnouncementsCountsCharactersInsteadOfBytes(t *testing.T) {
	announcements := []map[string]interface{}{
		{
			"content":     strings.Repeat("\u6d4b", 1000),
			"publishDate": time.Date(2026, 3, 21, 17, 48, 0, 0, time.UTC).Format(time.RFC3339),
			"type":        "default",
			"extra":       "",
		},
	}

	payload, err := json.Marshal(announcements)
	if err != nil {
		t.Fatalf("marshal announcements: %v", err)
	}

	if err := validateAnnouncements(string(payload)); err != nil {
		t.Fatalf("expected 1000 Chinese characters to pass, got error: %v", err)
	}
}

func TestValidateAnnouncementsRejectsContentOverOneThousandCharacters(t *testing.T) {
	announcements := []map[string]interface{}{
		{
			"content":     strings.Repeat("\u6d4b", 1001),
			"publishDate": time.Date(2026, 3, 21, 17, 48, 0, 0, time.UTC).Format(time.RFC3339),
			"type":        "default",
			"extra":       "",
		},
	}

	payload, err := json.Marshal(announcements)
	if err != nil {
		t.Fatalf("marshal announcements: %v", err)
	}

	if err := validateAnnouncements(string(payload)); err == nil {
		t.Fatal("expected content over 1000 characters to fail")
	}
}
