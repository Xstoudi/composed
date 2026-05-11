package notify

import (
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

	for _, expected := range []string{"Created stacks:", "- api", "- worker", "Updated stacks:", "Deleted stacks:", "Error: up failed"} {
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
	service := NewWithClient("ntfy://example", fake)

	summary := EventSummary{Updated: []string{"api"}}
	if err := service.Send("Composed update", summary); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !fake.called {
		t.Fatalf("expected Send() to call client")
	}
	if fake.url != "ntfy://example" {
		t.Fatalf("url = %q, want %q", fake.url, "ntfy://example")
	}
	if !strings.Contains(fake.message, "Composed update") || !strings.Contains(fake.message, "Updated stacks:") {
		t.Fatalf("unexpected message content: %q", fake.message)
	}
}
