//go:build !js

package track

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Load reads a strict versioned scene and checks its interpolated road.
func Load(path string) (Scene, error) {
	f, err := os.Open(path)
	if err != nil {
		return Scene{}, fmt.Errorf("load scene: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 2<<20))
	dec.DisallowUnknownFields()
	var scene Scene
	if err := dec.Decode(&scene); err != nil {
		return Scene{}, fmt.Errorf("load scene: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Scene{}, fmt.Errorf("load scene: trailing data")
	}
	upgradeStudyDigests(&scene)
	if _, err := SampleRoad(scene, 2); err != nil {
		return Scene{}, err
	}
	return Migrate(scene), nil
}

// Save atomically replaces path only after scene validation succeeds.
func Save(path string, scene Scene) error {
	if _, err := SampleRoad(scene, 2); err != nil {
		return err
	}
	scene = Migrate(scene)
	data, err := json.MarshalIndent(scene, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), ".the-line-*.json")
	if err != nil {
		return fmt.Errorf("save scene: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return fmt.Errorf("save scene: %w", err)
	}
	return nil
}
