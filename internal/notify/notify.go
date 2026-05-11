package notify

import (
	"fmt"
	"sort"
	"strings"

	"github.com/containrrr/shoutrrr"
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
		lines = append(lines, "Created stacks:")
		for _, name := range sortedUnique(s.Created) {
			lines = append(lines, fmt.Sprintf("- %s", name))
		}
	}

	if len(s.Updated) > 0 {
		lines = append(lines, "Updated stacks:")
		for _, name := range sortedUnique(s.Updated) {
			lines = append(lines, fmt.Sprintf("- %s", name))
		}
	}

	if len(s.Deleted) > 0 {
		lines = append(lines, "Deleted stacks:")
		for _, name := range sortedUnique(s.Deleted) {
			lines = append(lines, fmt.Sprintf("- %s", name))
		}
	}

	if s.Error != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("Error: %s", s.Error))
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
	if title != "" {
		message = fmt.Sprintf("%s\n\n%s", title, message)
	}

	return s.client.Send(s.url, message)
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
