package track

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestVersionOneMigrationPreservesGeometryAndDigest(t *testing.T) {
	for _, name := range Presets()[:5] {
		old, _ := Preset(name)
		old.Version = 1
		// The old digest serialized this exact struct shape and field order.
		type legacyPoint struct {
			X       float64 `json:"x"`
			Y       float64 `json:"y"`
			Z       float64 `json:"z"`
			Width   float64 `json:"width"`
			Bank    float64 `json:"bank"`
			Surface string  `json:"surface"`
		}
		legacy := make([]legacyPoint, len(old.Points))
		for i, p := range old.Points {
			legacy[i] = legacyPoint{p.X, p.Y, p.Z, p.Width, p.Bank, p.Surface}
		}
		data, _ := json.Marshal(legacy)
		digest := fmt.Sprintf("%x", sha256.Sum256(data))
		migrated := Migrate(old)
		if migrated.Version != 2 || RoadDigest(old) != digest || RoadDigest(migrated) != digest {
			t.Fatalf("%s migration digest changed", name)
		}
		a, err := SampleRoad(old, .5)
		if err != nil {
			t.Fatal(err)
		}
		b, err := SampleRoad(migrated, .5)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s migration changed sampled physical road", name)
		}
		migrated.Points[0].X++
		if old.Points[0].X == migrated.Points[0].X {
			t.Fatal("migration aliases source")
		}
	}
}

func TestAsymmetricBankedWidthsAndKerbLimits(t *testing.T) {
	s := Scene{Version: 2, Name: "Banked taper", Points: []Point{
		{WidthLeft: 3, WidthRight: 7, Bank: 10, Surface: "asphalt", KerbLeft: Kerb{Width: 2, Surface: "wet"}},
		{X: 100, WidthLeft: 5, WidthRight: 6, Bank: 10, Surface: "asphalt", KerbLeft: Kerb{Width: 2, Surface: "wet"}},
	}}
	r, err := SampleRoad(s, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range r {
		if p.LeftWidth() < 3 || p.LeftWidth() > 5 || p.RightWidth() < 6 || p.RightWidth() > 7 {
			t.Fatal("width interpolation overshot")
		}
		left := p.AtOffset(p.LeftLimit())
		if math.Abs(left.Y-p.LeftWidth()) > 1e-10 || math.Abs(left.Z-left.Y*math.Tan(math.Pi/18)) > 1e-10 {
			t.Fatal("banked asymmetric edge incorrect")
		}
	}
	s.KerbsCountAsRoad = true
	allowed, err := SampleRoad(s, 2)
	if err != nil {
		t.Fatal(err)
	}
	p := allowed[0]
	if p.LeftLimit() != 5 || p.RightLimit() != 7 || p.GripAcross(3.5, .2) != .65 || p.GripAcross(1, .2) != 1 {
		t.Fatal("kerb limit/grip mismatch")
	}
	without := s
	without.KerbsCountAsRoad = false
	if RoadDigest(s) == RoadDigest(without) {
		t.Fatal("track-limit rule missing from digest")
	}
	without = Migrate(s)
	without.Points[0].KerbLeft.Grip = .5
	if RoadDigest(s) == RoadDigest(without) {
		t.Fatal("kerb grip missing from digest")
	}
	without = Migrate(s)
	without.Points[0].WidthLeft++
	if RoadDigest(s) == RoadDigest(without) {
		t.Fatal("asymmetric width missing from digest")
	}
	s.Points[0].WidthLeft = 1
	if s.Validate() == nil {
		t.Fatal("invalid side width accepted")
	}
}
