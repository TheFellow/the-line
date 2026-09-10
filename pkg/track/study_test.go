package track

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestStudyPersistenceAndRoadIdentity(t *testing.T) {
	s, _ := Preset("hairpin")
	car, _ := vehicle.Preset("road")
	source := s
	digest := RoadDigest(s)
	s.Study = &Study{Version: 1, Manual: &ManualLine{Spacing: .5, RoadDigest: digest, Offsets: []float64{0, 1, 0}}, Reference: &PinnedLine{Name: "Before grip change", Scene: &source, Vehicle: car, Line: ManualLine{Spacing: .5, RoadDigest: digest, Offsets: []float64{0, 1, 0}}}}
	s.VehicleConfig = &car
	path := filepath.Join(t.TempDir(), "study.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Study.Reference.Name != "Before grip change" || loaded.Study.Manual.Offsets[1] != 1 {
		t.Fatal("study did not survive persistence")
	}
	loaded.Name = "renamed"
	loaded.VehicleConfig.Power *= 1.1
	loaded.EntrySpeed++
	if RoadDigest(loaded) != digest {
		t.Fatal("presentation, car or caps changed physical road identity")
	}
	loaded.Points[1].Bank++
	if RoadDigest(loaded) == digest {
		t.Fatal("physical road change did not stale reference")
	}
	copy := CloneStudy(s.Study)
	copy.Manual.Offsets[0] = 2
	copy.Reference.Line.Offsets[0] = 2
	copy.Reference.Scene.Points[0].X++
	if s.Study.Manual.Offsets[0] != 0 || s.Study.Reference.Line.Offsets[0] != 0 || s.Study.Reference.Scene.Points[0] != source.Points[0] {
		t.Fatal("mutable study aliases original")
	}
}

func TestStudyActiveLineSelectionAndLegacyCompatibility(t *testing.T) {
	if (*Study)(nil).UsesManual() {
		t.Fatal("nil study selected a manual line")
	}
	scene, _ := Preset("hairpin")
	manual := &ManualLine{Spacing: 3, RoadDigest: RoadDigest(scene), Offsets: []float64{0, 1, 0}}
	for _, test := range []struct {
		active  string
		manual  *ManualLine
		want    bool
		invalid bool
	}{
		{active: ""},
		{active: "", manual: manual, want: true},
		{active: LineManual, manual: manual, want: true},
		{active: LineOptimized, manual: manual},
		{active: LineOptimized},
		{active: LineManual, invalid: true},
		{active: "fastest", manual: manual, invalid: true},
	} {
		study := Study{Version: 1, ActiveLine: test.active, Manual: test.manual}
		if err := study.Validate(); (err != nil) != test.invalid {
			t.Fatalf("active %q manual %v: validation %v", test.active, test.manual != nil, err)
		}
		if study.UsesManual() != test.want {
			t.Fatalf("active %q manual %v: selection differs", test.active, test.manual != nil)
		}
		if test.invalid {
			continue
		}
		encoded, err := json.Marshal(study)
		if err != nil {
			t.Fatal(err)
		}
		var decoded Study
		if err := json.Unmarshal(encoded, &decoded); err != nil || !reflect.DeepEqual(study, decoded) {
			t.Fatalf("active selection did not roundtrip: %v", err)
		}
	}
}

func TestLoadingStaleStudyPreservesExplicitSelection(t *testing.T) {
	scene, _ := Preset("hairpin")
	scene.Study = &Study{Version: 1, ActiveLine: LineManual, Manual: &ManualLine{Spacing: 3, RoadDigest: RoadDigest(scene), Offsets: []float64{0, 0}}}
	scene.Points[1].Z++
	path := filepath.Join(t.TempDir(), "stale-study.json")
	if err := Save(path, scene); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Study.UsesManual() || loaded.Study.ActiveLine != LineManual || loaded.Study.Manual.RoadDigest == RoadDigest(loaded) {
		t.Fatal("loading silently rewrote stale manual intent")
	}
}

func TestStudyRejectsNestedReference(t *testing.T) {
	s, _ := Preset("hairpin")
	s.Study = &Study{Version: 1}
	s.Study.Reference = &PinnedLine{Scene: &s, Name: "recursive"}
	if err := s.Study.Validate(); err == nil {
		t.Fatal("nested reference accepted")
	}
}
