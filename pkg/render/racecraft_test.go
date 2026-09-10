package render

import (
	"bytes"
	"context"
	"github.com/TheFellow/the-line/pkg/racecraft"
	"image"
	"testing"
)

func TestRacePlaybackAndControls(t *testing.T) {
	e, _ := racecraft.Example("pass-repass")
	race, err := racecraft.Plan(context.Background(), e.Scene, e.Vehicle, e.Config)
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"2d", "3d"} {
		r, err := New(e.Scene, race.Cars[0].Path, e.Vehicle, Options{Race: &race, View: view})
		if err != nil {
			t.Fatal(err)
		}
		if r.PlaybackDuration(false) != race.Duration || r.PlaybackDuration(true) != race.Duration {
			t.Fatal("race uses a ghost clock")
		}
		for _, key := range []string{"race-mode", "race-scenario", "race-gap+", "race-overspeed-", "race-separation+", "race-clearance-", "race-save", "race-load", "scrub", "play"} {
			if _, ok := r.Controls()[key]; !ok {
				t.Fatalf("missing %s", key)
			}
		}
		if _, ok := r.Controls()["comparison"]; ok {
			t.Fatal("race exposes ghost toggle")
		}
		// Return buffer reuse is intentional. Check both identities actually move.
		first := append([]byte(nil), r.raceFrame(0, State{}).(*image.RGBA).Pix...)
		last := r.raceFrame(race.Duration, State{}).(*image.RGBA).Pix
		if bytes.Equal(first, last) {
			t.Fatal("static race playback")
		}
	}
}
