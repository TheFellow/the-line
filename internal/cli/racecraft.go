package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/TheFellow/the-line/pkg/racecraft"
	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func runRace(args []string, stdout, stderr io.Writer) error {
	f := flag.NewFlagSet("race", flag.ContinueOnError)
	f.SetOutput(stderr)
	var scenario, file, scene, car, format, save string
	a := arguments{}
	var gap, overspeed, separation, clearance float64
	f.StringVar(&scenario, "scenario", "over-under", "over-under, pass-repass, defend, esses-duel")
	f.StringVar(&file, "experiment", "", "load complete experiment JSON")
	f.StringVar(&save, "save-experiment", "", "save effective experiment inputs as JSON")
	f.StringVar(&scene, "scene", "", "override road with scene JSON (open only)")
	f.StringVar(&car, "vehicle", "", "override shared vehicle preset")
	f.StringVar(&format, "format", "json", "json, csv, png, or gif")
	f.StringVar(&a.out, "out", "", "output path (JSON/CSV otherwise use stdout)")
	f.StringVar(&a.view, "view", "2d", "2d or 3d")
	f.Float64Var(&gap, "gap", 0, "override starting station gap in metres")
	f.Float64Var(&overspeed, "overspeed", 0, "override B minus A station-zero speed cap in m/s (realized start speeds differ)")
	f.Float64Var(&separation, "separation", 0, "override lateral placement separation in metres")
	f.Float64Var(&clearance, "clearance", 0, "override extra body clearance in metres")
	f.IntVar(&a.width, "width", 960, "image width")
	f.IntVar(&a.height, "height", 600, "image height")
	f.Float64Var(&a.at, "time", 0, "start time in seconds")
	f.Float64Var(&a.duration, "duration", 0, "animation seconds; zero runs to first finish")
	f.Float64Var(&a.fps, "fps", 20, "animation frames per second")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() > 0 {
		return fmt.Errorf("unexpected race arguments")
	}
	if format != "json" && format != "csv" && format != "png" && format != "gif" {
		return fmt.Errorf("race format must be json, csv, png, or gif")
	}
	if a.view != "2d" && a.view != "3d" {
		return fmt.Errorf("race view must be 2d or 3d")
	}
	if !finite(a.at) || a.at < 0 || !finite(a.duration) || a.duration < 0 || !finite(a.fps) || a.fps < 1 || a.fps > 50 {
		return fmt.Errorf("invalid race time, duration or fps (1–50)")
	}
	if a.width < 800 || a.width > 3840 || a.height < 600 || a.height > 2160 {
		return fmt.Errorf("image size must be 800–3840 by 600–2160")
	}
	if (format == "png" || format == "gif") && a.out == "" {
		return fmt.Errorf("race images require --out")
	}
	if err := validateRaceFiles(a.out, save, file, scene); err != nil {
		return err
	}
	e, err := racecraft.Example(scenario)
	if err != nil {
		return err
	}
	if file != "" {
		e, err = racecraft.Load(file)
		if err != nil {
			return err
		}
	}
	explicit := map[string]bool{}
	f.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if file != "" && explicit["scenario"] {
		return fmt.Errorf("choose --experiment or --scenario")
	}
	if explicit["gap"] {
		e.Config.Gap = gap
	}
	if explicit["overspeed"] {
		e.Config.Overspeed = overspeed
	}
	if explicit["separation"] {
		e.Config.Separation = separation
	}
	if explicit["clearance"] {
		e.Config.Clearance = clearance
	}
	if scene != "" {
		e.Scene, err = track.Load(scene)
		if err != nil {
			return err
		}
	}
	if car != "" {
		e.Vehicle, err = vehicle.Preset(car)
		if err != nil {
			return err
		}
	}
	if err := e.Validate(); err != nil {
		return err
	}
	result, err := racecraft.Plan(context.Background(), e.Scene, e.Vehicle, e.Config)
	if err != nil {
		return err
	}
	// Each destination is replaced atomically, but the pair is not a transaction.
	// Finish export validation and encoding before replacing saved inputs.
	if err := writeRaceExport(a, format, e, result, stdout); err != nil {
		return err
	}
	if save != "" {
		// A newly created export can reveal an alias that did not exist during
		// preflight, such as differently cased names on a case-insensitive volume.
		if err := validateRaceFiles(a.out, save, file, scene); err != nil {
			return err
		}
		if err := racecraft.Save(save, e); err != nil {
			return err
		}
	}
	start := result.At(0)
	fmt.Fprintf(stderr, "%s: %.3f s; %d completed passes; certified body clearance >= %.3f m; realized start speeds A %.3f, B %.3f m/s; requested B-A station-zero cap delta %.3f m/s\n", e.Config.Scenario, result.Duration, len(result.Events), result.MinClearance, start[0].Speed, start[1].Speed, e.Config.Overspeed)
	return nil
}

// validateRaceFiles rejects predictable destination failures and input clobbers.
// Saving back to --experiment is intentional; exports must have their own path.
func validateRaceFiles(out, save, experiment, scene string) error {
	paths := []struct {
		flag, path, resolved string
		info                 os.FileInfo
	}{{flag: "out", path: out}, {flag: "save-experiment", path: save}, {flag: "experiment", path: experiment}, {flag: "scene", path: scene}}
	for i := range paths {
		p := &paths[i]
		if p.path == "" {
			continue
		}
		absolute, err := filepath.Abs(p.path)
		if err != nil {
			return err
		}
		dir, err := filepath.EvalSymlinks(filepath.Dir(absolute))
		if err != nil {
			return fmt.Errorf("--%s: %w", p.flag, err)
		}
		parent, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("--%s: %w", p.flag, err)
		}
		if !parent.IsDir() {
			return fmt.Errorf("--%s parent is not a directory", p.flag)
		}
		p.resolved = filepath.Join(dir, filepath.Base(absolute))
		p.info, err = os.Stat(absolute)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("--%s: %w", p.flag, err)
		}
		if i < 2 && p.info != nil && p.info.IsDir() {
			return fmt.Errorf("--%s destination is a directory", p.flag)
		}
	}
	for _, pair := range [][2]int{{0, 1}, {0, 2}, {0, 3}, {1, 3}} {
		a, b := paths[pair[0]], paths[pair[1]]
		if a.path != "" && b.path != "" && (a.resolved == b.resolved || a.info != nil && b.info != nil && os.SameFile(a.info, b.info)) {
			return fmt.Errorf("--%s and --%s refer to the same file", a.flag, b.flag)
		}
	}
	return nil
}

func writeRaceExport(a arguments, format string, e racecraft.Experiment, result racecraft.Result, stdout io.Writer) error {
	if format == "png" || format == "gif" {
		r, err := render.New(e.Scene, result.Cars[0].Path, e.Vehicle, render.Options{Width: a.width, Height: a.height, View: a.view, Race: &result})
		if err != nil {
			return err
		}
		if format == "gif" {
			return writeAnimation(a, r, result.Duration, false)
		}
		return output(a.out, func(w io.Writer) error { return png.Encode(w, r.Frame(a.at)) })
	}
	write := func(w io.Writer) error {
		if format == "json" {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		csvWriter := csv.NewWriter(w)
		if err := csvWriter.Write([]string{"time_s", "a_station_m", "b_station_m", "a_speed_mps", "b_speed_mps", "a_minus_b_gap_m", "sampled_body_clearance_m"}); err != nil {
			return err
		}
		for t := 0.; t <= result.Duration; t = math.Min(t+.05, result.Duration) {
			n := result.At(t)
			clear := math.Hypot(n[0].Position.X-n[1].Position.X, n[0].Position.Y-n[1].Position.Y) - result.Cars[0].Radius - result.Cars[1].Radius
			values := []float64{t, n[0].Station, n[1].Station, n[0].Speed, n[1].Speed, n[0].Station - n[1].Station, clear}
			row := make([]string, len(values))
			for i, v := range values {
				row[i] = strconv.FormatFloat(v, 'g', 17, 64)
			}
			if err := csvWriter.Write(row); err != nil {
				return err
			}
			if t == result.Duration {
				break
			}
		}
		csvWriter.Flush()
		return csvWriter.Error()
	}
	if a.out == "" {
		return write(stdout)
	}
	return output(a.out, write)
}
