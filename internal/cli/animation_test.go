package cli

import (
	"bytes"
	"image/gif"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestAnimationFrameBudgetsAndSequenceBounds(t *testing.T) {
	for _, test := range []struct {
		name     string
		closed   bool
		start    float64
		duration float64
		width    int
		want     int
		err      string
	}{
		{name: "open remaining", start: 3, want: 70},
		{name: "open clamps duration", start: 3, duration: 20, want: 70},
		{name: "open cannot start later", start: 20, duration: 1, err: "outside the sequence"},
		{name: "open default at finish", start: 10, err: "past the end"},
		{name: "closed default full lap", closed: true, start: 3, want: 100},
		{name: "closed default later lap", closed: true, start: 53, want: 100},
		{name: "closed multiple laps", closed: true, start: 53, duration: 21, want: 210},
		{name: "closed frame budget", closed: true, duration: 181, err: "memory budget"},
		{name: "closed pixel budget", closed: true, duration: 30, width: 3840, err: "memory budget"},
		{name: "closed floating overflow", closed: true, duration: math.MaxFloat64, err: "memory budget"},
	} {
		t.Run(test.name, func(t *testing.T) {
			width := test.width
			if width == 0 {
				width = 800
			}
			got, err := animationFrames(arguments{at: test.start, duration: test.duration, fps: 10, width: width, height: 600}, 10, test.closed)
			if test.err != "" {
				if err == nil || !strings.Contains(err.Error(), test.err) {
					t.Fatalf("error %v, want %q", err, test.err)
				}
			} else if err != nil || got != test.want {
				t.Fatalf("frames %d, error %v; want %d", got, err, test.want)
			}
		})
	}
}

func TestClosedAnimationCLITraversesSeamAndLaterLaps(t *testing.T) {
	scene, err := track.Preset("club-loop")
	if err != nil {
		t.Fatal(err)
	}
	car, _ := vehicle.Preset(scene.Vehicle)
	run, err := solver.Evaluate(scene, car, nil, solver.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	slower := car
	slower.Grip *= .65
	reference, err := solver.Evaluate(scene, slower, nil, solver.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	scene.Study = &track.Study{Version: 1,
		Manual:    &track.ManualLine{Spacing: run.Spacing, RoadDigest: track.RoadDigest(scene), Offsets: run.Offsets},
		Reference: render.StoreReference("Slower ghost", scene, reference, slower),
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "lap.json")
	if err := track.Save(path, scene); err != nil {
		t.Fatal(err)
	}
	export := func(name string, start, duration, fps float64) *gif.GIF {
		t.Helper()
		out := filepath.Join(dir, name+".gif")
		number := func(v float64) string { return strconv.FormatFloat(v, 'g', 17, 64) }
		args := []string{"animate", "--scene", path, "--time", number(start), "--duration", number(duration), "--fps", number(fps), "--width", "800", "--height", "600", "--out", out}
		if err := Run(args, io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		animation, err := gif.DecodeAll(f)
		if err != nil {
			t.Fatal(err)
		}
		return animation
	}
	seam := export("seam", run.Duration-.2, .4, 10)
	if len(seam.Image) != 4 {
		t.Fatalf("seam export was truncated to the first lap: %d frames", len(seam.Image))
	}
	for _, delay := range seam.Delay {
		if delay != 10 {
			t.Fatalf("incorrect GIF clock: delay %d hundredths", delay)
		}
	}
	if bytes.Equal(seam.Image[1].Pix, seam.Image[3].Pix) {
		t.Fatal("seam-crossing animation did not move")
	}
	later := export("later", 2*run.Duration+.3, .2, 10)
	early := export("early", .3, .2, 10)
	// Both current cars are at the same phase of their lap. The slower ghost
	// has a different phase: reducing the entire export clock modulo A's lap
	// would incorrectly put both cars back at their early-lap positions.
	if bytes.Equal(later.Image[0].Pix, early.Image[0].Pix) && bytes.Equal(later.Image[1].Pix, early.Image[1].Pix) {
		t.Fatal("later lap lost the independent ghost clock")
	}
	full := export("default-later", 2*run.Duration+.3, 0, 1)
	if want := int(math.Ceil(run.Duration)); len(full.Image) != want {
		t.Fatalf("default later-lap export has %d frames, want one full lap (%d)", len(full.Image), want)
	}
}
