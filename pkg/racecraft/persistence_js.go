//go:build js && wasm

package racecraft

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"
)

func Save(path string, e Experiment) (err error) {
	if err = e.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("save browser race: %v", v)
		}
	}()
	js.Global().Get("localStorage").Call("setItem", "the-line:race:"+path, string(data))
	return nil
}
func Load(path string) (e Experiment, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("load browser race: %v", v)
		}
	}()
	value := js.Global().Get("localStorage").Call("getItem", "the-line:race:"+path)
	if value.IsNull() {
		return e, fmt.Errorf("race experiment %q does not exist", path)
	}
	return Decode(strings.NewReader(value.String()))
}
