// Command review renders a reproducible, display-independent visual matrix.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
	xdraw "golang.org/x/image/draw"
)

type measurement struct {
	Preset        string   `json:"preset"`
	Duration      float64  `json:"duration_s"`
	Reference     float64  `json:"reference_s"`
	ForceResidual float64  `json:"force_residual_mps2"`
	SolveMillis   int64    `json:"solve_ms"`
	Images        []string `json:"images"`
}

func run() error {
	out := flag.String("out", "artifacts/roadmap-review", "directory for PNGs and measurements")
	car := flag.String("vehicle", "gt", "illustrative vehicle preset")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		return err
	}
	v, err := vehicle.Preset(*car)
	if err != nil {
		return err
	}
	names := track.Presets()
	gallery := image.NewRGBA(image.Rect(0, 0, 2160, 450*len(names)))
	var report []measurement
	for row, name := range names {
		s, err := track.Preset(name)
		if err != nil {
			return err
		}
		started := time.Now()
		r, err := solver.Solve(s, v, solver.DefaultOptions())
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		m := measurement{Preset: name, Duration: r.Duration, Reference: r.CenterDuration, ForceResidual: r.MaxForceResidual, SolveMillis: time.Since(started).Milliseconds()}
		for col, view := range []string{"2d", "3d", "perspective"} {
			renderer, err := render.New(s, r, v, render.Options{Width: 1440, Height: 900, View: view})
			if err != nil {
				return err
			}
			im := renderer.Frame(r.Duration * .52)
			file := name + "-" + view + ".png"
			if err := save(filepath.Join(*out, file), im); err != nil {
				return err
			}
			m.Images = append(m.Images, file)
			xdraw.ApproxBiLinear.Scale(gallery, image.Rect(col*720, row*450, (col+1)*720, (row+1)*450), im, im.Bounds(), xdraw.Src, nil)
		}
		report = append(report, m)
		fmt.Printf("%s: %.3fs / %.3fs reference, %dms solve\n", name, m.Duration, m.Reference, m.SolveMillis)
	}
	if err := save(filepath.Join(*out, "gallery.png"), gallery); err != nil {
		return err
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(*out, "report.json"), append(b, '\n'), 0644)
}

func save(path string, im image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, im)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
