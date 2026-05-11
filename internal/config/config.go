package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

type Config struct {
	StacksFolder string
	LockFile     string
	NotifyURL    string
	ComposeFiles []string
}

func (c *Config) IsProjectFile(path string) bool {
	if c == nil || path == "" {
		return false
	}

	cleanPath := filepath.Clean(filepath.ToSlash(path))
	projectsFolder := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(c.StacksFolder)), "/")

	return cleanPath == projectsFolder || strings.HasPrefix(cleanPath, projectsFolder+"/")
}

func (c *Config) IsComposeFile(path string) bool {
	if c == nil || path == "" {
		return false
	}
	for _, file := range c.ComposeFiles {
		if strings.HasSuffix(path, file) {
			return true
		}
	}
	return false
}

type partialConfig struct {
	StacksFolder string `toml:"stacksFolder" yaml:"stacksFolder"`
	LockFile     string `toml:"lockFile" yaml:"lockFile"`
	NotifyURL    string `toml:"notifyURL" yaml:"notifyURL"`
	ComposeFiles string `toml:"composeFiles" yaml:"composeFiles"`
}

var (
	once    sync.Once
	cached  *Config
	initErr error
)

func Init(workingDir string) error {
	once.Do(func() {
		cached, initErr = loadConfig(workingDir)
	})
	return initErr
}

func Get() *Config {
	if cached == nil {
		panic("config.Get called before successful config.Init")
	}
	return cached
}

func loadConfig(workingDir string) (*Config, error) {
	cfg := defaultConfig()

	candidates := []string{
		"composed.toml",
		"composed.yaml",
		"composed.yml",
	}

	var found string
	for _, candidate := range candidates {
		path := filepath.Join(workingDir, candidate)
		if _, err := os.Stat(path); err == nil {
			found = path
			break
		}
	}

	if found != "" {
		data, err := os.ReadFile(found)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		var partial partialConfig

		switch filepath.Ext(found) {
		case ".toml":
			if err := toml.Unmarshal(data, &partial); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		case ".yaml", ".yml":
			if err := yaml.Unmarshal(data, &partial); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}

		if partial.StacksFolder != "" {
			cfg.StacksFolder = partial.StacksFolder
		}
		if partial.LockFile != "" {
			cfg.LockFile = partial.LockFile
		}
		if partial.NotifyURL != "" {
			cfg.NotifyURL = partial.NotifyURL
		}
	}
	return &cfg, nil
}

func defaultConfig() Config {
	return Config{
		StacksFolder: "stacks",
		LockFile:     "/tmp/.composed-lock",
		NotifyURL:    "",
		ComposeFiles: []string{
			"docker-compose.yml",
			"docker-compose.yaml",
			"compose.yml",
			"compose.yaml",
		},
	}
}
