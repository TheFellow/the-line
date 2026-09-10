// Package cli provides the display-independent command interface.
package cli

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/gif"
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
  validate   Validate a track and its vehicle
  solve      Optimize a sequence; export JSON or CSV telemetry
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
	f.Float64Var(&a.spacing, "spacing", defaults.Spacing, "sample spacing in metres")
	f.IntVar(&a.iterations, "iterations", defaults.Iterations, "bounded optimization iterations")
	f.Float64Var(&a.margin, "margin", defaults.Margin, "extra clearance beyond half vehicle width, metres")
	if command == "solve" {
		f.StringVar(&a.format, "format", "json", "output format: json or csv")
	}
	if command == "render" || command == "animate" {
		f.StringVar(&a.view, "view", "2d", "view: 2d or 3d")
		defaultWidth, defaultHeight := 1440, 900
		if command == "animate" {
			defaultWidth, defaultHeight = 960, 600
		}
		f.IntVar(&a.width, "width", defaultWidth, "image width in pixels")
		f.IntVar(&a.height, "height", defaultHeight, "image height in pixels")
		f.Float64Var(&a.at, "time", 0, "start time in seconds")
	}
	if command == "animate" {
		f.Float64Var(&a.duration, "duration", 0, "animation seconds; zero renders the complete sequence")
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
			if p.Width <= config.Width+2*a.margin {
				return fmt.Errorf("road sample %d is too narrow for this vehicle and clearance", i)
			}
		}
		fmt.Fprintf(stdout, "Valid: %s · %d controls · %d road samples · %s\n", scene.Name, len(scene.Points), len(road), config.Name)
		return nil
	}
	if command == "solve" && a.format != "json" && a.format != "csv" {
		return errors.New("--format must be json or csv")
	}
	if command == "render" || command == "animate" {
		if a.out == "" {
			return fmt.Errorf("%s requires --out", command)
		}
		if a.view != "2d" && a.view != "3d" {
			return errors.New("--view must be 2d or 3d")
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
	result, err := solver.Solve(scene, config, solver.Options{Spacing: a.spacing, Iterations: a.iterations, Margin: a.margin})
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "%s: %.3f s; centreline %.3f s; improvement %.2f%%; %.1f m\n", scene.Name, result.Duration, result.CenterDuration, 100*(result.CenterDuration-result.Duration)/result.CenterDuration, result.Length)
	switch command {
	case "solve":
		write := func(w io.Writer) error {
			if a.format == "csv" {
				return writeCSV(w, result)
			}
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "  ")
			return encoder.Encode(struct {
				Scene   track.Scene    `json:"scene"`
				Vehicle vehicle.Config `json:"vehicle"`
				Result  solver.Result  `json:"result"`
				Method  string         `json:"method"`
			}{scene, config, result, "heuristic minimum-time estimate; open sequence with entry/exit speed caps"})
		}
		if a.out == "" {
			return write(stdout)
		}
		return output(a.out, write)
	case "render", "animate":
		renderer, err := render.New(scene, result, config, render.Options{Width: a.width, Height: a.height, View: a.view})
		if err != nil {
			return err
		}
		if command == "render" {
			err = output(a.out, func(w io.Writer) error { return png.Encode(w, renderer.Frame(a.at)) })
		} else {
			err = writeAnimation(a, renderer, math.Max(result.Duration, result.CenterDuration))
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
	if err := c.Write([]string{"s_m", "time_s", "x_m", "y_m", "z_m", "speed_mps", "curvature_per_m", "offset_m", "acceleration_mps2", "station_m"}); err != nil {
		return err
	}
	for _, n := range r.Nodes {
		values := []float64{n.S, n.Time, n.Position.X, n.Position.Y, n.Position.Z, n.Speed, n.Curvature, n.Offset, n.Acceleration, n.Station}
		row := make([]string, len(values))
		for i, v := range values {
			row[i] = strconv.FormatFloat(v, 'f', 6, 64)
		}
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

func writeAnimation(a arguments, r *render.Renderer, total float64) error {
	duration := a.duration
	if duration == 0 {
		duration = total - a.at
	}
	if duration <= 0 {
		return errors.New("animation starts past the end of the sequence")
	}
	if a.at >= total {
		return errors.New("animation start time is outside the sequence")
	}
	duration = math.Min(duration, total-a.at)
	frames := int(math.Ceil(duration * a.fps))
	if frames > 1800 || float64(frames)*float64(a.width)*float64(a.height) > 600e6 {
		return errors.New("animation exceeds memory budget; reduce duration, fps, or image size")
	}
	animation := gif.GIF{LoopCount: 0}
	quantizer := newGIFQuantizer(r.Frame(a.at))
	for i := 0; i < frames; i++ {
		// GIF timing is in hundredths. Distribute rounding to preserve the clock.
		begin := int(math.Round(float64(i) * 100 / a.fps))
		end := int(math.Round(float64(i+1) * 100 / a.fps))
		frame := r.Frame(a.at + float64(begin)/100)
		p := quantizer.frame(frame)
		animation.Image = append(animation.Image, p)
		animation.Delay = append(animation.Delay, max(1, end-begin))
	}
	return output(a.out, func(w io.Writer) error { return gif.EncodeAll(w, &animation) })
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
