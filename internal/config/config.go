// Package config reads the app settings from config.yaml.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"

	"todo/internal/auth"
)

// Config holds the settings from config.yaml.
type Config struct {
	Password Password `yaml:"password"`
}

// Password holds the password rules.
type Password struct {
	// MinLength is the shortest allowed new password, in characters.
	MinLength int `yaml:"min_length"`
}

// Default returns the settings used when config.yaml sets nothing.
func Default() Config {
	return Config{Password: Password{MinLength: auth.DefaultMinPasswordChars}}
}

// Load reads the settings from path. Keys that config.yaml leaves out keep their
// defaults. If allowMissing is true, a missing file gives the defaults.
func Load(path string, allowMissing bool) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && allowMissing {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // a typo in a key is an error, not a silent default
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	if n := cfg.Password.MinLength; n < 1 || n > auth.MaxPasswordChars {
		return cfg, fmt.Errorf("config %s: password.min_length must be between 1 and %d, got %d", path, auth.MaxPasswordChars, n)
	}
	return cfg, nil
}
