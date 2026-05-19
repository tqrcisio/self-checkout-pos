package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kardianos/osext"
)

// EnvConfigPath is the environment variable that overrides the default
// config.json lookup (which expects the file next to the binary).
const EnvConfigPath = "BOILERPLATE_CONFIG"

const DefaultPort = 7000

const DefaultAPIKey = "change-me"

type Config struct {
	Port              int    `json:"port"`
	ApiKey            string `json:"api_key"`
	AutoUpdateEnabled *bool  `json:"auto_update_enabled,omitempty"`
}

// AutoUpdateOn returns true unless AutoUpdateEnabled is explicitly false.
// Absence of the field means default-true so fresh config files require no
// migration.
func (c Config) AutoUpdateOn() bool {
	if c.AutoUpdateEnabled == nil {
		return true
	}
	return *c.AutoUpdateEnabled
}

func Default() Config {
	return Config{
		Port:   DefaultPort,
		ApiKey: DefaultAPIKey,
	}
}

func ExecutableDir() (string, error) {
	return osext.ExecutableFolder()
}

func Path() (string, error) {
	if p := os.Getenv(EnvConfigPath); p != "" {
		return p, nil
	}
	dir, err := ExecutableDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Default(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		cfg := Default()
		_ = Save(cfg)
		return cfg, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
