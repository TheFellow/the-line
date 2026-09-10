//go:build !js || !wasm

package racecraft

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func Load(path string) (Experiment, error) {
	f, err := os.Open(path)
	if err != nil {
		return Experiment{}, err
	}
	defer f.Close()
	return Decode(f)
}
func Save(path string, e Experiment) error {
	if err := e.Validate(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".racecraft-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(e); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
