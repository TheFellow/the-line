package racecraft_test

import (
	"encoding/json"
	"github.com/TheFellow/the-line/pkg/racecraft"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExperimentRoundTrip(t *testing.T) {
	e, _ := racecraft.Example("pass-repass")
	file := filepath.Join(t.TempDir(), "race.json")
	if err := racecraft.Save(file, e); err != nil {
		t.Fatal(err)
	}
	got, err := racecraft.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e, got) {
		t.Fatal("experiment changed")
	}
	data, _ := json.Marshal(e)
	for _, bad := range []string{string(data) + " {}", strings.Replace(string(data), `"gap_m"`, `"typo"`, 1), strings.Replace(string(data), `"version":1`, `"version":999`, 1)} {
		if _, err := racecraft.Decode(strings.NewReader(bad)); err == nil {
			t.Fatal("accepted malformed experiment")
		}
	}
	if _, err := racecraft.Decode(strings.NewReader(string(data) + strings.Repeat(" ", 4<<20))); err == nil {
		t.Fatal("accepted oversized experiment")
	}
	e.Config.Gap = -1
	if err := racecraft.Save(file, e); err == nil {
		t.Fatal("saved invalid experiment")
	}
	after, err := racecraft.Load(file)
	if err != nil || !reflect.DeepEqual(after, got) {
		t.Fatal("invalid save damaged prior file")
	}
}
