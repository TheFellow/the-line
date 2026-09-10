// Package cli provides the display-independent command interface.
package cli

import (
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
	"strings"

	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

const help = `The Line — racing-line studio

Usage: the-line <command> [flags]

  presets    List tracks, vehicles and surfaces
  new        Create a versioned track JSON file from a preset
  import     Import a CSV centreline with asymmetric road widths
  validate   Validate a track and its vehicle
  solve      Optimize a sequence; export JSON or CSV telemetry
  race       Plan two solid cars; export JSON, CSV, PNG or GIF
  sweep      Sweep a setup parameter on one fixed optimized line
  render     Render the studio to a PNG without a display
  animate    Render a timed animated GIF without a display

Common flags:
  --preset esses       Built-in corner sequence
  --scene track.json   Load an existing scene instead
  --vehicle gt         Override the scene vehicle preset
  --vehicle-file car.json  Use custom vehicle parameters
  --spacing 3          Search sample spacing in metres
  --iterations 4       Search iterations
  --out path           Output file (solve prints JSON if omitted)

Run a command with --help for its flags.
Launch the live editor with: go run ./main/gui
Results are heuristic time estimates for the configured vehicle model.
`

type arguments struct {
	line                                                   string
	workers, polish                                        int
	lineColor, channel                                     string
	preset, scene, vehicle, vehicleFile, out, format, view string
	spacing, margin, at, duration, fps                     float64
	iterations, width, height                              int
}

// Run executes a command without initializing a graphics driver.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(stdout, help)
		return err
	}
	command := args[0]
	if command == "race" {
		return runRace(args[1:], stdout, stderr)
	}
	if command == "import" {
		return runImport(args[1:], stdout, stderr)
	}
	if command == "sweep" {
		return runSweep(args[1:], stdout, stderr)
	}
	if command == "presets" {
		if len(args) > 1 {
			return errors.New("presets takes no arguments")
		}
		fmt.Fprintln(stdout, "Tracks:   "+strings.Join(track.Presets(), ", "))
		fmt.Fprintln(stdout, "Vehicles: "+strings.Join(vehicle.Presets(), ", "))
		fmt.Fprintln(stdout, "Surfaces:")
		for _, s := range track.Surfaces() {
			fmt.Fprintf(stdout, "  %-12s grip %.2f\n", s.Name, s.Grip)
		}
		return nil
	}
	switch command {
	case "new", "validate", "solve", "render", "animate":
	default:
		return fmt.Errorf("unknown command %q; run the-line help", command)
	}
	defaults := solver.DefaultOptions()
	a := arguments{}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(stderr)
	f.StringVar(&a.preset, "preset", "esses", "built-in corner sequence")
	f.StringVar(&a.scene, "scene", "", "read a scene JSON file")
	f.StringVar(&a.vehicle, "vehicle", "", "vehicle preset override")
	f.StringVar(&a.vehicleFile, "vehicle-file", "", "custom vehicle JSON file")
	f.StringVar(&a.out, "out", "", "output file")
	f.StringVar(&a.line, "line", "auto", "line: auto (saved active selection), optimized, or manual")
	f.Float64Var(&a.spacing, "spacing", defaults.Spacing, "sample spacing in metres")
	f.IntVar(&a.iterations, "iterations", defaults.Iterations, "bounded optimization iterations")
	f.IntVar(&a.workers, "workers", 0, "fine-candidate workers; zero selects a bounded default")
	f.IntVar(&a.polish, "polish", 0, "additional verified local refinement sweeps (0–3)")
	f.Float64Var(&a.margin, "margin", defaults.Margin, "extra clearance beyond half vehicle width, metres")
	if command == "solve" {
		f.StringVar(&a.format, "format", "json", "output format: json, csv, or comparison-csv")
	}
	if command == "render" || command == "animate" {
		f.StringVar(&a.lineColor, "color", "speed", "line color: speed, utilization, lateral_g, longitudinal_g")
		f.StringVar(&a.channel, "channel", "speed", "chart channel: speed, utilization, lateral_g, longitudinal_g")
		f.StringVar(&a.view, "view", "2d", "view: 2d, 3d, or perspective")
		defaultWidth, defaultHeight := 1440, 900
		if command == "animate" {
			defaultWidth, defaultHeight = 960, 600
		}
		f.IntVar(&a.width, "width", defaultWidth, "image width in pixels")
		f.IntVar(&a.height, "height", defaultHeight, "image height in pixels")
		f.Float64Var(&a.at, "time", 0, "start time in seconds; closed laps accept any nonnegative time")
	}
	if command == "animate" {
		f.Float64Var(&a.duration, "duration", 0, "animation seconds; zero renders the remaining open sequence or one closed lap")
		f.Float64Var(&a.fps, "fps", 20, "animation frames per second (1–50)")
	}
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(f.Args(), " "))
	}
	if !finite(a.margin) || a.margin < 0 {
		return errors.New("--margin must be finite and nonnegative")
	}
	if a.vehicle != "" && a.vehicleFile != "" {
		return errors.New("choose --vehicle or --vehicle-file")
	}
	scene, err := readScene(a)
	if err != nil {
		return err
	}
	config, err := readVehicle(a, scene.Vehicle)
	if a.vehicleFile == "" && a.vehicle == "" && scene.VehicleConfig != nil {
		config = *scene.VehicleConfig
		err = config.Validate()
	}
	if err != nil {
		return err
	}
	if a.vehicle != "" {
		scene.Vehicle = a.vehicle
		scene.VehicleConfig = nil
	}
	if a.vehicleFile != "" {
		scene.VehicleConfig = &config
	}
	if err := config.Validate(); err != nil {
		return err
	}
	if command == "new" {
		if a.out == "" {
			return errors.New("new requires --out track.json")
		}
		if err := track.Save(a.out, scene); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Saved", a.out)
		return nil
	}
	if command == "validate" {
		road, err := track.SampleRoad(scene, a.spacing)
		if err != nil {
			return err
		}
		for i, p := range road {
			if p.LeftLimit() <= config.Width/2+a.margin || p.RightLimit() <= config.Width/2+a.margin {
				return fmt.Errorf("road sample %d is too narrow for this vehicle and clearance", i)
			}
		}
		fmt.Fprintf(stdout, "Valid: %s · %d controls · %d road samples · %s\n", scene.Name, len(scene.Points), len(road), config.Name)
		return nil
	}
	if a.line != "auto" && a.line != "manual" && a.line != "optimized" {
		return errors.New("--line must be auto, optimized, or manual")
	}
	if command == "solve" && a.format != "json" && a.format != "csv" && a.format != "comparison-csv" {
		return errors.New("--format must be json, csv, or comparison-csv")
	}
	if command == "render" || command == "animate" {
		if a.out == "" {
			return fmt.Errorf("%s requires --out", command)
		}
		if a.view != "2d" && a.view != "3d" && a.view != "perspective" {
			return errors.New("--view must be 2d, 3d, or perspective")
		}
		if a.width < 800 || a.height < 600 || a.width > 3840 || a.height > 2160 {
			return errors.New("image size must be 800–3840 by 600–2160 pixels")
		}
		if !finite(a.at) || a.at < 0 {
			return errors.New("--time must be a finite nonnegative number")
		}
	}
	if command == "animate" && (!finite(a.fps) || a.fps < 1 || a.fps > 50 || !finite(a.duration) || a.duration < 0) {
		return errors.New("animation needs finite --fps between 1 and 50 and nonnegative --duration")
	}
	opts := solver.Options{Spacing: a.spacing, Iterations: a.iterations, Margin: a.margin, Workers: a.workers, Polish: a.polish}
	manual := a.line == "manual" || a.line == "auto" && scene.Study.UsesManual()
	var result solver.Result
	if manual {
		if scene.Study == nil || scene.Study.Manual == nil {
			return errors.New("scene has no saved manual line")
		}
		line := scene.Study.Manual
		if line.RoadDigest != track.RoadDigest(scene) {
			return errors.New("manual line stale: different road; use --line optimized")
		}
		opts.Spacing = line.Spacing
		result, err = solver.Evaluate(scene, config, line.Offsets, opts)
	} else {
		result, err = solver.Solve(scene, config, opts)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "%s: %.3f s; centreline %.3f s; improvement %.2f%%; %.1f m\n", scene.Name, result.Duration, result.CenterDuration, 100*(result.CenterDuration-result.Duration)/result.CenterDuration, result.Length)
	switch command {
	case "solve":
		write := func(w io.Writer) error {
			if a.format == "comparison-csv" {
				return writeComparisonCSV(w, scene, result)
			}
			if a.format == "csv" {
				return writeCSV(w, result)
			}
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "  ")
			method := "heuristic minimum-time estimate; open sequence with entry/exit speed caps"
			if scene.Closed {
				method = "heuristic steady-state lap estimate; periodic speed and offsets"
			}
			if manual {
				method = "verified saved manual line; quasi-static time estimate"
			}
			return encoder.Encode(struct {
				Scene   track.Scene    `json:"scene"`
				Vehicle vehicle.Config `json:"vehicle"`
				Result  solver.Result  `json:"result"`
				Method  string         `json:"method"`
			}{scene, config, result, method})
		}
		if a.out == "" {
			return write(stdout)
		}
		return output(a.out, write)
	case "render", "animate":
		renderOpts := render.Options{Width: a.width, Height: a.height, View: a.view, LineColor: render.Channel(a.lineColor), ChartChannel: render.Channel(a.channel), Manual: manual}
		if scene.Study != nil {
			renderOpts.Reference, err = render.RestoreReference(scene.Study.Reference)
			if err != nil {
				return err
			}
		}
		renderer, err := render.New(scene, result, config, renderOpts)
		if err != nil {
			return err
		}
		if command == "render" {
			err = output(a.out, func(w io.Writer) error { return png.Encode(w, renderer.Frame(a.at)) })
		} else {
			err = writeAnimation(a, renderer, renderer.PlaybackDuration(true), scene.Closed)
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Saved", a.out)
	}
	return nil
}

func readScene(a arguments) (track.Scene, error) {
	if a.scene != "" {
		return track.Load(a.scene)
	}
	return track.Preset(a.preset)
}
func readVehicle(a arguments, name string) (vehicle.Config, error) {

	if a.vehicleFile != "" {
		var v vehicle.Config
		file, err := os.Open(a.vehicleFile)
		if err != nil {
			return v, err
		}
		defer file.Close()
		decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&v); err != nil {
			return v, fmt.Errorf("vehicle file: %w", err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return v, errors.New("vehicle file must contain exactly one JSON object")
		}
		return v, v.Validate()
	}
	if a.vehicle != "" {
		name = a.vehicle
	}
	return vehicle.Preset(name)
}
func writeCSV(w io.Writer, r solver.Result) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"s_m", "time_s", "x_m", "y_m", "z_m", "speed_mps", "curvature_per_m", "offset_m", "acceleration_mps2", "station_m", "bank_deg", "grade", "grip", "tyre_lateral_mps2", "tyre_longitudinal_mps2", "tyre_capacity_mps2", "tyre_utilization", "phase", "limit", "forces_available"}); err != nil {
		return err
	}
	for _, n := range r.Nodes {
		values := []float64{n.S, n.Time, n.Position.X, n.Position.Y, n.Position.Z, n.Speed, n.Curvature, n.Offset, n.Acceleration, n.Station, n.Bank, n.Grade, n.Grip, n.Forces.Lateral, n.Forces.Longitudinal, n.Forces.Capacity, n.Forces.Utilization}
		row := make([]string, len(values))
		for i, v := range values {
			row[i] = strconv.FormatFloat(v, 'g', 17, 64)
		}
		row = append(row, n.Forces.Phase, n.Forces.Limit, strconv.FormatBool(n.Forces.Available))
		if err := c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

// output writes atomically so a failed export cannot truncate a prior artifact.
func output(path string, write func(io.Writer) error) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".the-line-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = write(f); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
