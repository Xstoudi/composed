package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetConfigState() {
	once = sync.Once{}
	cached = nil
	initErr = nil
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()

	if cfg.StacksFolder != "stacks" {
		t.Fatalf("StacksFolder = %q, want %q", cfg.StacksFolder, "stacks")
	}
	if cfg.LockFile != "/tmp/.composed-lock" {
		t.Fatalf("LockFile = %q, want %q", cfg.LockFile, "/tmp/.composed-lock")
	}
	if cfg.NotifyURL != "" {
		t.Fatalf("NotifyURL = %q, want empty", cfg.NotifyURL)
	}
}

func TestLoadConfigReturnsDefaultsWhenNoFileExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.StacksFolder != "stacks" {
		t.Fatalf("StacksFolder = %q, want default", cfg.StacksFolder)
	}
	if cfg.LockFile != "/tmp/.composed-lock" {
		t.Fatalf("LockFile = %q, want default", cfg.LockFile)
	}
	if cfg.NotifyURL != "" {
		t.Fatalf("NotifyURL = %q, want default empty", cfg.NotifyURL)
	}
}

func TestLoadConfigReadsTOMLAndOverridesSpecifiedFieldsOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	content := `stacksFolder = "examples"
lockFile = "/var/run/composed.lock"
notifyURL = "ntfy://token@ntfy.example/topic"
`
	if err := os.WriteFile(filepath.Join(dir, "composed.toml"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.StacksFolder != "examples" {
		t.Fatalf("StacksFolder = %q, want default", cfg.StacksFolder)
	}
	if cfg.LockFile != "/var/run/composed.lock" {
		t.Fatalf("LockFile = %q, want %q", cfg.LockFile, "/var/run/composed.lock")
	}
	if cfg.NotifyURL != "ntfy://token@ntfy.example/topic" {
		t.Fatalf("NotifyURL = %q, want %q", cfg.NotifyURL, "ntfy://token@ntfy.example/topic")
	}
}

func TestLoadConfigReadsYAMLAndOverridesSpecifiedFieldsOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	content := `stacksFolder: "examples"
lockFile: "/tmp/custom.lock"
notifyURL: "discord://token@channel"
`
	if err := os.WriteFile(filepath.Join(dir, "composed.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.StacksFolder != "examples" {
		t.Fatalf("StacksFolder = %q, want default", cfg.StacksFolder)
	}
	if cfg.LockFile != "/tmp/custom.lock" {
		t.Fatalf("LockFile = %q, want %q", cfg.LockFile, "/tmp/custom.lock")
	}
	if cfg.NotifyURL != "discord://token@channel" {
		t.Fatalf("NotifyURL = %q, want %q", cfg.NotifyURL, "discord://token@channel")
	}
}

func TestLoadConfigPrefersTOMLOverYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "composed.toml"), []byte(`stacksFolder = "examples"`), 0o600); err != nil {
		t.Fatalf("WriteFile(toml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "composed.yaml"), []byte(`stacksFolder: "examples"`), 0o600); err != nil {
		t.Fatalf("WriteFile(yaml) error = %v", err)
	}

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.StacksFolder != "examples" {
		t.Fatalf("StacksFolder = %q, want default", cfg.StacksFolder)
	}
}

func TestLoadConfigReturnsParseErrorForInvalidTOML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "composed.toml"), []byte(`projectsFolder = `), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := loadConfig(dir); err == nil {
		t.Fatalf("loadConfig() error = nil, want non-nil")
	}
}

func TestInitAndGetCacheConfig(t *testing.T) {
	resetConfigState()
	t.Cleanup(resetConfigState)

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "composed.toml"), []byte(`stacksFolder = "examples"`), 0o600); err != nil {
		t.Fatalf("WriteFile(first) error = %v", err)
	}

	if err := Init(dir); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	first := Get()
	if first.StacksFolder != "examples" {
		t.Fatalf("Get().StacksFolder = %q, want %q", first.StacksFolder, "examples")
	}

	if err := os.WriteFile(filepath.Join(dir, "composed.toml"), []byte(`git = "second"`), 0o600); err != nil {
		t.Fatalf("WriteFile(second) error = %v", err)
	}

	if err := Init(dir); err != nil {
		t.Fatalf("second Init() error = %v", err)
	}

	second := Get()
	if second.StacksFolder != "examples" {
		t.Fatalf("cached Get().StacksFolder = %q, want %q", second.StacksFolder, "examples")
	}
	if first != second {
		t.Fatalf("Get() returned different pointers, want cached pointer")
	}
}
