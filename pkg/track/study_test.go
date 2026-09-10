package track

import (
	"path/filepath"
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

func TestStudyRejectsNestedReference(t *testing.T) {
	s, _ := Preset("hairpin")
	s.Study = &Study{Version: 1}
	s.Study.Reference = &PinnedLine{Scene: &s, Name: "recursive"}
	if err := s.Study.Validate(); err == nil {
		t.Fatal("nested reference accepted")
	}
}
