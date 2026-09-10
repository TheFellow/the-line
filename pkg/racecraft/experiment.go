package racecraft

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// Experiment stores the complete reproducible inputs, not computed trajectories.
type Experiment struct {
	Config  Config         `json:"racecraft"`
	Scene   track.Scene    `json:"scene"`
	Vehicle vehicle.Config `json:"vehicle"`
}

func Example(name string) (Experiment, error) {
	c := DefaultConfig(name)
	if err := c.Validate(); err != nil {
		return Experiment{}, err
	}
	s, err := Scene(name)
	if err != nil {
		return Experiment{}, err
	}
	v, _ := vehicle.Preset("road")
	return Experiment{Config: c, Scene: s, Vehicle: v}, nil
}
func (e Experiment) Validate() error {
	if err := e.Config.Validate(); err != nil {
		return err
	}
	if err := e.Vehicle.Validate(); err != nil {
		return err
	}
	if e.Scene.Closed {
		return fmt.Errorf("racecraft requires an open corner sequence")
	}
	_, err := track.SampleRoad(e.Scene, 2)
	return err
}
func Decode(r io.Reader) (Experiment, error) {
	var e Experiment
	dec := json.NewDecoder(io.LimitReader(r, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return e, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return e, fmt.Errorf("expected exactly one experiment object")
	}
	return e, e.Validate()
}
