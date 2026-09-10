//go:build !js

package vehicle

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Load reads one strict vehicle JSON object and validates every field.
func Load(path string) (Config, error) {
	var c Config
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, fmt.Errorf("vehicle: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return c, fmt.Errorf("vehicle: expected exactly one JSON object")
	}
	return c, c.Validate()
}

// Save atomically replaces a vehicle file after validation.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".the-line-car-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(c); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
