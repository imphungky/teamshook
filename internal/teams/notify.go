// Package teams builds and delivers Adaptive Card notifications to a Microsoft
// Teams channel via an Incoming Webhook URL.
//
// Phase 1 scope: turn a parsed GitHub push event into one Adaptive Card and
// POST it. The card layout follows the agreed plan:
//
//   - Primary:   "@pusher pushed to <branch>" title + "View commit on GitHub" button
//   - Secondary: head-commit message, repo, short SHA
//   - Tertiary:  timestamp, small and subtle in the footer
//
// Only fields already parsed in internal/webhook/github.go are used.
package teams

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/imphungky/teamshook/internal/webhook"
)

// --- Adaptive Card wire format ------------------------------------------------
//
// Teams Incoming Webhooks accept a "message" envelope whose attachments carry an
// Adaptive Card. The card itself is an untyped tree of elements, so we model the
// pieces we actually emit rather than the whole schema. Every field is tagged
// omitempty so zero values disappear from the JSON instead of sending nulls that
// some Adaptive Card hosts reject.

// payload is the top-level body POSTed to the Teams webhook URL.
type payload struct {
	Type        string       `json:"type"` // always "message"
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	ContentType string `json:"contentType"` // the adaptive-card MIME type
	Content     card   `json:"content"`
}

// card is the AdaptiveCard root. Body holds the stacked content elements;
// Actions holds the buttons rendered at the bottom.
type card struct {
	Type    string    `json:"type"`    // "AdaptiveCard"
	Schema  string    `json:"$schema"` // the adaptivecards.io schema URL
	Version string    `json:"version"` // "1.4"
	Body    []element `json:"body"`
	Actions []action  `json:"actions,omitempty"`
}

// element is one node in the card body. Because Adaptive Cards mix several node
// shapes (TextBlock, ColumnSet, FactSet, ...) into a single list, we use one
// permissive struct and let omitempty drop the fields a given node doesn't use.
type element struct {
	Type    string    `json:"type"`
	Text    string    `json:"text,omitempty"`
	Weight  string    `json:"weight,omitempty"`  // "Bolder" for the title
	Size    string    `json:"size,omitempty"`    // "Medium", "Small", ...
	Spacing string    `json:"spacing,omitempty"` // "None", "Small", "Medium"
	Wrap    bool      `json:"wrap,omitempty"`
	Subtle  bool      `json:"isSubtle,omitempty"`
	Columns []column  `json:"columns,omitempty"` // for type "ColumnSet"
	Facts   []fact    `json:"facts,omitempty"`   // for type "FactSet"
	Items   []element `json:"items,omitempty"`   // for nested containers/columns
}

type column struct {
	Type  string    `json:"type"`  // "Column"
	Width string    `json:"width"` // "auto" | "stretch" | pixel value
	Items []element `json:"items"`
}

type fact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

// action is an Adaptive Card action; we only use Action.OpenUrl (opens a link).
type action struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

const (
	cardContentType = "application/vnd.microsoft.card.adaptive"
	cardSchema      = "http://adaptivecards.io/schemas/adaptive-card.json"
	cardVersion     = "1.4"
)

// BuildPushCard renders a push event into a Teams message payload.
//
// It returns (nil, false) for events that should NOT produce a card — currently
// branch deletes, which arrive with an empty head commit. The caller treats the
// false as "skip silently" rather than an error.
func BuildPushCard(e webhook.GithubPushEvent) (payload, bool) {
	// A branch/tag delete has no head commit to link to. There is nothing useful
	// to show, so skip it. (HeadCommit.ID is empty on deletes.)
	if e.HeadCommit.ID == "" {
		return payload{}, false
	}

	verb, refName := describeRef(e.Ref)
	pusher := pusherHandle(e)
	title := fmt.Sprintf("%s %s %s", pusher, verb, refName)

	body := []element{
		// Row 1 — GitHub mark + title/subtitle stack. The title answers
		// "who + where" at a glance; the subtitle gives repo and commit count.
		{
			Type: "ColumnSet",
			Columns: []column{
				{
					Type:  "Column",
					Width: "auto",
					Items: []element{{
						Type: "TextBlock",
						Text: "▶", // ▶  a lightweight stand-in for the GitHub mark
						Size: "Medium",
					}},
				},
				{
					Type:  "Column",
					Width: "stretch",
					Items: []element{
						{Type: "TextBlock", Weight: "Bolder", Size: "Medium", Text: title},
						{
							Type:    "TextBlock",
							Spacing: "None",
							Subtle:  true,
							Text:    fmt.Sprintf("%s · %s", e.Repository.FullName, commitCount(len(e.Commits))),
						},
					},
				},
			},
		},
		// Row 2 — the head-commit message (first line only), the body's anchor.
		{
			Type:    "TextBlock",
			Wrap:    true,
			Spacing: "Medium",
			Text:    firstLine(e.HeadCommit.Message),
		},
		// Row 3 — short SHA as a fact. Calm, scannable, secondary.
		{
			Type:  "FactSet",
			Facts: []fact{{Title: "Commit", Value: shortSHA(e.HeadCommit.ID)}},
		},
		// Row 4 — timestamp, small + subtle. {{DATE/TIME}} are Adaptive Card
		// functions the client renders in each viewer's own locale and timezone.
		{
			Type:    "TextBlock",
			Size:    "Small",
			Subtle:  true,
			Spacing: "Small",
			Text:    formatTimestamp(e.HeadCommit.Timestamp),
		},
	}

	actions := []action{
		{Type: "Action.OpenUrl", Title: "View commit on GitHub", URL: e.HeadCommit.URL},
	}
	// Only offer the diff button when GitHub gave us a compare URL.
	if e.Compare != "" {
		actions = append(actions, action{Type: "Action.OpenUrl", Title: "View diff", URL: e.Compare})
	}

	return payload{
		Type: "message",
		Attachments: []attachment{{
			ContentType: cardContentType,
			Content: card{
				Type:    "AdaptiveCard",
				Schema:  cardSchema,
				Version: cardVersion,
				Body:    body,
				Actions: actions,
			},
		}},
	}, true
}

// Post serializes the payload and delivers it to a Teams Incoming Webhook URL.
// Teams returns 200 with a short body on success; any other status is an error.
func Post(webhookURL string, p payload) error {
	buf, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshaling card: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("posting card to Teams: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Teams webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// --- small pure helpers (easy to unit test) ----------------------------------

// describeRef turns a Git ref into a human action phrase and a display name.
//
//	refs/heads/main    -> ("pushed to", "main")
//	refs/tags/v1.2.0   -> ("pushed tag", "v1.2.0")
//
// One card shape, branching only on the wording — the agreed approach.
func describeRef(ref string) (action, name string) {
	switch {
	case strings.HasPrefix(ref, "refs/tags/"):
		return "pushed tag", strings.TrimPrefix(ref, "refs/tags/")
	case strings.HasPrefix(ref, "refs/heads/"):
		return "pushed to", strings.TrimPrefix(ref, "refs/heads/")
	default:
		// Unknown ref namespace: show it verbatim rather than guessing.
		return "pushed", ref
	}
}

// pusherHandle prefers the @username (it maps to a real GitHub profile) and
// falls back to the display name when GitHub omits the username.
func pusherHandle(e webhook.GithubPushEvent) string {
	if e.Pusher.Username != "" {
		return "@" + e.Pusher.Username
	}
	if e.Pusher.Name != "" {
		return e.Pusher.Name
	}
	return "Someone"
}

// firstLine returns the commit subject — Git separates the subject from the body
// with a blank line, and the card only has room for the subject.
func firstLine(msg string) string {
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return strings.TrimSpace(msg[:i])
	}
	return strings.TrimSpace(msg)
}

// shortSHA mirrors GitHub's 7-character abbreviation, guarding against a SHA
// that is somehow shorter than 7 chars.
func shortSHA(id string) string {
	if len(id) > 7 {
		return id[:7]
	}
	return id
}

// commitCount renders a grammatically correct count for the subtitle.
func commitCount(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}

// formatTimestamp wraps an RFC 3339 timestamp in Adaptive Card DATE/TIME
// functions so each viewer sees it localized. If the timestamp doesn't parse we
// fall back to plain prose rather than emitting a broken function call.
func formatTimestamp(ts string) string {
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		if ts == "" {
			return ""
		}
		return "Pushed " + ts
	}
	return fmt.Sprintf("Pushed {{DATE(%s, SHORT)}} {{TIME(%s)}}", ts, ts)
}
