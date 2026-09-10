//go:build js && wasm

package track

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"syscall/js"
)

// Browser builds use origin-local storage. Native builds retain atomic files.
// Both paths preserve the same strict, versioned scene document.
func Save(path string, scene Scene) (err error) {
	if _, err = SampleRoad(scene, 2); err != nil {
		return err
	}
	scene = Migrate(scene)
	data, err := json.Marshal(scene)
	if err != nil {
		return err
	}
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("save browser study: %v", value)
		}
	}()
	js.Global().Get("localStorage").Call("setItem", "the-line:scene:"+path, string(data))
	return nil
}
func Load(path string) (scene Scene, err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("load browser study: %v", value)
		}
	}()
	value := js.Global().Get("localStorage").Call("getItem", "the-line:scene:"+path)
	if value.IsNull() {
		return Scene{}, fmt.Errorf("browser study %q does not exist", path)
	}
	dec := json.NewDecoder(io.LimitReader(strings.NewReader(value.String()), 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&scene); err != nil {
		return Scene{}, fmt.Errorf("load browser study: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Scene{}, fmt.Errorf("load browser study: trailing data")
	}
	if _, err := SampleRoad(scene, 2); err != nil {
		return Scene{}, err
	}
	return Migrate(scene), nil
}
