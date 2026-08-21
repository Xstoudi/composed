package notify

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/nicholas-fedor/shoutrrr"
)

type EventSummary struct {
	Created []string
	Updated []string
	Deleted []string
	Error   string
}

func (s EventSummary) HasContent() bool {
	return len(s.Created) > 0 || len(s.Updated) > 0 || len(s.Deleted) > 0 || s.Error != ""
}

func (s EventSummary) Message() string {
	lines := make([]string, 0)

	if len(s.Created) > 0 {
		lines = appendSectionHeader(lines)
		for _, name := range sortedUnique(s.Created) {
			lines = append(lines, fmt.Sprintf("* 🆕 %s (created)", name))
		}
	}

	if len(s.Updated) > 0 {
		lines = appendSectionHeader(lines)
		for _, name := range sortedUnique(s.Updated) {
			lines = append(lines, fmt.Sprintf("* 🔃 %s (updated)", name))
		}
	}

	if len(s.Deleted) > 0 {
		lines = appendSectionHeader(lines)
		for _, name := range sortedUnique(s.Deleted) {
			lines = append(lines, fmt.Sprintf("* 🔽 %s (deleted)", name))
		}
	}

	if s.Error != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "**Error:**", fmt.Sprintf("* 🚨 %s", s.Error))
	}

	return strings.Join(lines, "\n")
}

type Client interface {
	Send(url, message string) error
}

type shoutrrrClient struct{}

func (shoutrrrClient) Send(url, message string) error {
	return shoutrrr.Send(url, message)
}

type Service struct {
	url    string
	client Client
}

func New(url string) *Service {
	return &Service{url: strings.TrimSpace(url), client: shoutrrrClient{}}
}

func NewWithClient(url string, client Client) *Service {
	if client == nil {
		client = shoutrrrClient{}
	}

	return &Service{url: strings.TrimSpace(url), client: client}
}

func (s *Service) Enabled() bool {
	return s != nil && s.url != ""
}

func (s *Service) Send(title string, summary EventSummary) error {
	if s == nil || !s.Enabled() || !summary.HasContent() {
		return nil
	}

	message := strings.TrimSpace(summary.Message())
	notificationURL := withNotificationDefaults(s.url, title, summary.Error != "")

	return s.client.Send(notificationURL, message)
}

func appendSectionHeader(lines []string) []string {
	if len(lines) == 0 {
		return append(lines, "**Service status changes:**")
	}

	return lines
}

func withNotificationDefaults(rawURL, title string, hasError bool) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil || serviceScheme(parsedURL.Scheme) != "ntfy" {
		return rawURL
	}

	query := parsedURL.Query()
	query.Set("markdown", "yes")
	if title != "" && query.Get("title") == "" {
		query.Set("title", title)
	}
	if query.Get("priority") == "" {
		query.Set("priority", "urgent")
	}
	if query.Get("tags") == "" {
		tags := "party_face,tada"
		if hasError {
			tags = "rotating_light,warning"
		}
		query.Set("tags", tags)
	}
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

func serviceScheme(scheme string) string {
	if before, _, ok := strings.Cut(scheme, "+"); ok {
		return before
	}

	return scheme
}

func sortedUnique(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}

	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}

	sort.Strings(result)
	return result
}
