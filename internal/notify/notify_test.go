package notify

import (
	"net/url"
	"strings"
	"testing"
)

type fakeClient struct {
	called  bool
	url     string
	message string
	err     error
}

func (f *fakeClient) Send(url, message string) error {
	f.called = true
	f.url = url
	f.message = message
	return f.err
}

func TestEventSummaryMessage(t *testing.T) {
	summary := EventSummary{
		Created: []string{"api", "api", "worker"},
		Updated: []string{"frontend"},
		Deleted: []string{"old"},
		Error:   "up failed",
	}

	message := summary.Message()

	for _, expected := range []string{
		"**Service status changes:**",
		"* 🆕 api (created)",
		"* 🆕 worker (created)",
		"* 🔃 frontend (updated)",
		"* 🔽 old (deleted)",
		"**Error:**",
		"* 🚨 up failed",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message missing %q: %q", expected, message)
		}
	}
}

func TestServiceSendSkipsWhenDisabledOrEmpty(t *testing.T) {
	fake := &fakeClient{}

	service := NewWithClient("", fake)
	if err := service.Send("Composed update", EventSummary{Created: []string{"api"}}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if fake.called {
		t.Fatalf("expected disabled notifier to skip send")
	}

	service = NewWithClient("ntfy://example", fake)
	if err := service.Send("Composed update", EventSummary{}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if fake.called {
		t.Fatalf("expected empty summary to skip send")
	}
}

func TestServiceSendDeliversMessage(t *testing.T) {
	fake := &fakeClient{}
	service := NewWithClient("ntfy://example/topic", fake)

	summary := EventSummary{Updated: []string{"api"}}
	if err := service.Send("Composed update", summary); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !fake.called {
		t.Fatalf("expected Send() to call client")
	}

	notificationURL, err := url.Parse(fake.url)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", fake.url, err)
	}
	if notificationURL.Scheme != "ntfy" || notificationURL.Host != "example" || notificationURL.Path != "/topic" {
		t.Fatalf("url = %q, want ntfy://example/topic with defaults", fake.url)
	}
	query := notificationURL.Query()
	for key, expected := range map[string]string{
		"markdown": "yes",
		"priority": "urgent",
		"tags":     "party_face,tada",
		"title":    "Composed update",
	} {
		if query.Get(key) != expected {
			t.Fatalf("query %q = %q, want %q in url %q", key, query.Get(key), expected, fake.url)
		}
	}
	if strings.Contains(fake.message, "Composed update") || !strings.Contains(fake.message, "**Service status changes:**") {
		t.Fatalf("unexpected message content: %q", fake.message)
	}
}

func TestServiceSendForcesMarkdownAndPreservesNtfyURLOverrides(t *testing.T) {
	fake := &fakeClient{}
	service := NewWithClient("ntfy://example/topic?markdown=no&priority=4&tags=info&title=Custom", fake)

	if err := service.Send("Composed update", EventSummary{Updated: []string{"api"}}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	notificationURL, err := url.Parse(fake.url)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", fake.url, err)
	}
	query := notificationURL.Query()
	for key, expected := range map[string]string{
		"markdown": "yes",
		"priority": "4",
		"tags":     "info",
		"title":    "Custom",
	} {
		if query.Get(key) != expected {
			t.Fatalf("query %q = %q, want %q in url %q", key, query.Get(key), expected, fake.url)
		}
	}
}

func TestServiceSendLeavesNonNtfyURLAlone(t *testing.T) {
	fake := &fakeClient{}
	service := NewWithClient("discord://token@channel", fake)

	if err := service.Send("Composed update", EventSummary{Updated: []string{"api"}}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if fake.url != "discord://token@channel" {
		t.Fatalf("url = %q, want unchanged discord URL", fake.url)
	}
}
