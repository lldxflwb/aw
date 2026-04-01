package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the aw.yml configuration file.
type Config struct {
	Context  []string          `yaml:"context"`
	Branches map[string]string `yaml:"branches,omitempty"`
}

// DefaultConfig returns the default configuration with hardcoded context candidates.
func DefaultConfig() *Config {
	return &Config{
		Context: []string{
			"CLAUDE.md",
			"AGENTS.md",
			"codex.md",
			".claude",
			".codex",
			".cursorrules",
			".cursor",
		},
	}
}

const configFile = "aw.yml"

// defaultConfigYAML is the content written when aw.yml doesn't exist.
const defaultConfigYAML = `# aw configuration
# context: AI context files/directories to symlink into workspaces
context:
  - CLAUDE.md
  - AGENTS.md
  - codex.md
  - .claude
  - .codex
  - .cursorrules
  - .cursor
`

// LoadOrCreateConfig loads aw.yml from dir. If it doesn't exist, creates it with defaults.
func LoadOrCreateConfig(dir string) (*Config, bool, error) {
	path := filepath.Join(dir, configFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, false, fmt.Errorf("read %s: %w", configFile, err)
		}
		// Create default config
		if err := os.WriteFile(path, []byte(defaultConfigYAML), 0644); err != nil {
			return nil, false, fmt.Errorf("write %s: %w", configFile, err)
		}
		return DefaultConfig(), true, nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("parse %s: %w", configFile, err)
	}
	if len(cfg.Context) == 0 {
		cfg.Context = DefaultConfig().Context
	}
	return &cfg, false, nil
}
