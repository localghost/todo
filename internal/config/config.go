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

// Load reads the settings from path and reports whether the file was read.
// Keys that config.yaml leaves out keep their defaults. If allowMissing is
// true, a missing file gives the defaults.
func Load(path string, allowMissing bool) (Config, bool, error) {
	cfg, err := load(path, allowMissing)
	if errors.Is(err, errMissing) {
		return cfg, false, nil
	}
	return cfg, err == nil, err
}

// errMissing marks an allowed missing file inside load.
var errMissing = errors.New("missing")

func load(path string, allowMissing bool) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && allowMissing {
		return cfg, errMissing
	}
	if err != nil {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // a typo in a key is an error, not a silent default
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	// A second document would be ignored without notice.
	if err := dec.Decode(new(yaml.Node)); err == nil {
		return cfg, fmt.Errorf("config %s: only one YAML document is allowed", path)
	} else if !errors.Is(err, io.EOF) {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	// The decoder keeps the default for a null value and cuts 8.5 to 8, so
	// check the value in the YAML tree.
	if err := checkMinLength(data); err != nil {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	if n := cfg.Password.MinLength; n < 1 || n > auth.MaxPasswordChars {
		return cfg, fmt.Errorf("config %s: password.min_length must be between 1 and %d, got %d", path, auth.MaxPasswordChars, n)
	}
	return cfg, nil
}

// checkMinLength reports a present password section or min_length that has no
// value, and a min_length that is not a whole number.
func checkMinLength(data []byte) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return err
	}
	password := mapValue(doc.Content[0], "password")
	if password == nil {
		return nil
	}
	if password.Tag == "!!null" {
		return fmt.Errorf("line %d: password has no value", password.Line)
	}
	v := mapValue(password, "min_length")
	switch {
	case v == nil || v.Tag == "!!int":
		return nil
	case v.Tag == "!!null":
		return fmt.Errorf("line %d: password.min_length has no value", v.Line)
	default:
		return fmt.Errorf("line %d: password.min_length must be a whole number, got %q", v.Line, v.Value)
	}
}

// mapValue returns the value of key in the mapping node m, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m.Kind == yaml.AliasNode {
		m = m.Alias
	}
	if m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			v := m.Content[i+1]
			if v.Kind == yaml.AliasNode {
				v = v.Alias
			}
			return v
		}
	}
	return nil
}
