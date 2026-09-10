package track

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/TheFellow/the-line/pkg/vehicle"
)

// Study stores editable line hypotheses. Offsets are horizontal metres at the
// sampled road's stations; zero follows the centreline. Geometry uses the scene schema version.
type Study struct {
	Version   int         `json:"version"`
	Manual    *ManualLine `json:"manual,omitempty"`
	Reference *PinnedLine `json:"reference,omitempty"`
}

type ManualLine struct {
	Spacing    float64   `json:"spacing"`
	RoadDigest string    `json:"road_digest"`
	Offsets    []float64 `json:"offsets"`
}

// PinnedLine preserves the original geometry and speed caps as well as the car,
// so loading a study reconstructs the original reference, even on a stale road.
type PinnedLine struct {
	Name    string         `json:"name"`
	Scene   *Scene         `json:"scene"`
	Vehicle vehicle.Config `json:"vehicle"`
	Line    ManualLine     `json:"line"`
}

func (m ManualLine) Validate() error {
	if !finite(m.Spacing) || m.Spacing < .25 || m.Spacing > 20 {
		return fmt.Errorf("study: line spacing must be between 0.25 and 20 m")
	}
	if len(m.Offsets) < 2 || len(m.Offsets) > 100000 {
		return fmt.Errorf("study: line requires 2 to 100000 offsets")
	}
	for i, offset := range m.Offsets {
		if !finite(offset) || offset < -80 || offset > 80 {
			return fmt.Errorf("study: invalid offset %d", i)
		}
	}
	if len(m.RoadDigest) != 64 {
		return fmt.Errorf("study: invalid road digest")
	}
	return nil
}

func (s Study) Validate() error {
	if s.Version != 1 {
		return fmt.Errorf("study: unsupported version %d", s.Version)
	}
	if s.Manual != nil {
		if err := s.Manual.Validate(); err != nil {
			return err
		}
	}
	if p := s.Reference; p != nil {
		if p.Name == "" || len(p.Name) > 120 {
			return fmt.Errorf("study: reference name requires 1 to 120 bytes")
		}
		if p.Scene == nil || p.Scene.Study != nil {
			return fmt.Errorf("study: reference requires a nonnested source scene")
		}
		if err := p.Scene.Validate(); err != nil {
			return err
		}
		if err := p.Vehicle.Validate(); err != nil {
			return err
		}
		if err := p.Line.Validate(); err != nil {
			return err
		}
		if RoadDigest(*p.Scene) != p.Line.RoadDigest {
			return fmt.Errorf("study: reference road digest differs from its source")
		}
	}
	return nil
}

// RoadDigest identifies physical geometry and surfaces, deliberately excluding
// names, cars, speed caps and study metadata. Setup changes remain comparable.
func RoadDigest(scene Scene) string {
	// Preserve old digests for physically identical symmetric roads, including
	// their v2 migrations, so existing manual studies keep their identity.
	type legacyPoint struct {
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		Z       float64 `json:"z"`
		Width   float64 `json:"width"`
		Bank    float64 `json:"bank"`
		Surface string  `json:"surface"`
	}
	points := make([]legacyPoint, len(scene.Points))
	legacy := !scene.KerbsCountAsRoad && !scene.Closed
	for i, p := range scene.Points {
		points[i] = legacyPoint{p.X, p.Y, p.Z, p.LeftWidth() + p.RightWidth(), p.Bank, p.Surface}
		legacy = legacy && p.LeftWidth() == p.RightWidth() && p.KerbLeft == (Kerb{}) && p.KerbRight == (Kerb{})
	}
	var data []byte
	if legacy {
		data, _ = json.Marshal(points)
	} else {
		canonical := Migrate(Scene{Version: scene.Version, Points: scene.Points})
		data, _ = json.Marshal(struct {
			Points []Point `json:"points"`
			Kerbs  bool    `json:"kerbs_count_as_road"`
			Closed bool    `json:"closed,omitempty"`
		}{canonical.Points, scene.KerbsCountAsRoad, scene.Closed})
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// CloneStudy prevents callers and undo snapshots from sharing mutable offsets.
func CloneStudy(study *Study) *Study {
	if study == nil {
		return nil
	}
	out := *study
	if study.Manual != nil {
		m := *study.Manual
		m.Offsets = append([]float64(nil), m.Offsets...)
		out.Manual = &m
	}
	if study.Reference != nil {
		p := *study.Reference
		p.Line.Offsets = append([]float64(nil), p.Line.Offsets...)
		if p.Scene != nil {
			s := *p.Scene
			s.Points = append([]Point(nil), s.Points...)
			s.Study = nil
			if s.VehicleConfig != nil {
				v := *s.VehicleConfig
				s.VehicleConfig = &v
			}
			p.Scene = &s
		}
		out.Reference = &p
	}
	return &out
}
