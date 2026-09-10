//go:build js && wasm

package vehicle

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"syscall/js"
)

// Browser car presets use the same validated JSON in origin-local storage.
// Native builds persist atomic files instead.
func Save(path string, config Config) (err error) {
	if err := config.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("save browser car: %v", recovered)
		}
	}()
	js.Global().Get("localStorage").Call("setItem", "the-line:car:"+path, string(data))
	return nil
}

func Load(path string) (config Config, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("load browser car: %v", recovered)
		}
	}()
	value := js.Global().Get("localStorage").Call("getItem", "the-line:car:"+path)
	if value.IsNull() {
		return config, fmt.Errorf("browser car %q does not exist", path)
	}
	data := value.String()
	if len(data) > 1<<20 {
		return config, fmt.Errorf("browser car exceeds 1 MiB")
	}
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("load browser car: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return config, fmt.Errorf("load browser car: expected exactly one JSON object")
	}
	return config, config.Validate()
}
