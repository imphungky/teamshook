package teams

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/imphungky/teamshook/internal/webhook"
)

func TestBuildPushCard(t *testing.T) {
	e := webhook.GithubPushEvent{
		Ref:     "refs/heads/main",
		Compare: "https://github.com/imphungky/teamshook/compare/abc...def",
		Pusher:  webhook.PusherPayload{Username: "imphungky", Name: "Michael"},
		Repository: webhook.Repository{
			FullName: "imphungky/teamshook",
		},
		Commits: []webhook.Commit{{ID: "def5678"}},
		HeadCommit: webhook.Commit{
			ID:        "def5678abcdef",
			Message:   "add tests\n\nlonger body here",
			Timestamp: "2026-06-29T10:00:00Z",
			URL:       "https://github.com/imphungky/teamshook/commit/def5678",
		},
	}
	expected_status := true
	card, ok := BuildPushCard(e)
	if expected_status != ok {
		t.Errorf("Card failed to build")
	}
	assertContains(t, card, "@imphungky pushed to main")
}

func TestBuildPushCardDelete(t *testing.T) {
	e := webhook.GithubPushEvent{
		Ref:     "refs/heads/main",
		Compare: "https://github.com/imphungky/teamshook/compare/abc...def",
		Pusher:  webhook.PusherPayload{Username: "imphungky", Name: "Michael"},
		Repository: webhook.Repository{
			FullName: "imphungky/teamshook",
		},
		Commits: []webhook.Commit{{ID: "def5678"}},
	}
	expected := false
	expectedPayloadType := ""
	card, ok := BuildPushCard(e)
	if expected != ok {
		t.Errorf("Card should not build for deletes")
	}
	if card.Type != expectedPayloadType {
		t.Errorf("Card should be empty for deletes")
	}
}

func TestBuildPushCardViewDiffShows(t *testing.T) {
	e := webhook.GithubPushEvent{
		Ref:     "refs/heads/main",
		Compare: "https://github.com/imphungky/teamshook/compare/abc...def",
		Pusher:  webhook.PusherPayload{Username: "imphungky", Name: "Michael"},
		Repository: webhook.Repository{
			FullName: "imphungky/teamshook",
		},
		Commits: []webhook.Commit{{ID: "def5678"}},
		HeadCommit: webhook.Commit{
			ID:        "def5678abcdef",
			Message:   "add tests\n\nlonger body here",
			Timestamp: "2026-06-29T10:00:00Z",
			URL:       "https://github.com/imphungky/teamshook/commit/def5678",
		},
	}
	card, _ := BuildPushCard(e)
	assertContains(t, card, "View diff")
}

func assertContains(t *testing.T, result payload, substr string) {
	j, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Card failed to marshal: %v", err)
	}
	if !strings.Contains(string(j), substr) {
		t.Errorf("Card should contain %s", substr)
	}
}
