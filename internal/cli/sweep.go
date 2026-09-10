package cli

import (
	"encoding/csv"
	"flag"
	"fmt"
	"github.com/TheFellow/the-line/pkg/solver"
	"io"
	"strconv"
)

func runSweep(args []string, stdout, stderr io.Writer) error {
	f := flag.NewFlagSet("sweep", flag.ContinueOnError)
	f.SetOutput(stderr)
	a := arguments{}
	var parameter string
	var from, to float64
	var steps int
	f.StringVar(&a.preset, "preset", "esses", "track preset")
	f.StringVar(&a.scene, "scene", "", "scene JSON")
	f.StringVar(&a.vehicle, "vehicle", "", "vehicle preset")
	f.StringVar(&a.vehicleFile, "vehicle-file", "", "vehicle JSON")
	f.StringVar(&a.out, "out", "", "CSV output; stdout if omitted")
	f.StringVar(&parameter, "parameter", "grip", "setup field in vehicle JSON (SI units)")
	f.Float64Var(&from, "from", .9, "first parameter value in SI units")
	f.Float64Var(&to, "to", 1.3, "last parameter value in SI units")
	f.IntVar(&steps, "steps", 5, "number of evenly spaced values (2–200)")
	f.IntVar(&a.iterations, "iterations", 4, "initial line search iterations")
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("sweep takes only flags")
	}
	if !finite(from) || !finite(to) || steps < 2 || steps > 200 {
		return fmt.Errorf("sweep needs finite endpoints and 2–200 steps")
	}
	if a.vehicle != "" && a.vehicleFile != "" {
		return fmt.Errorf("choose --vehicle or --vehicle-file")
	}
	scene, err := readScene(a)
	if err != nil {
		return err
	}
	car, err := readVehicle(a, scene.Vehicle)
	if a.vehicleFile == "" && a.vehicle == "" && scene.VehicleConfig != nil {
		car = *scene.VehicleConfig
		err = car.Validate()
	}
	if err != nil {
		return err
	}
	if _, err := car.Value(parameter); err != nil {
		return err
	}
	for i := 0; i < steps; i++ {
		if _, err := car.With(parameter, from+(to-from)*float64(i)/float64(steps-1)); err != nil {
			return err
		}
	}
	opts := solver.DefaultOptions()
	opts.Iterations = a.iterations
	line, err := solver.Solve(scene, car, opts)
	if err != nil {
		return err
	}
	opts.Spacing = line.Spacing
	write := func(w io.Writer) error {
		c := csv.NewWriter(w)
		if err := c.Write([]string{"parameter", "value_si", "duration_s", "delta_s", "basis"}); err != nil {
			return err
		}
		for i := 0; i < steps; i++ {
			value := from + (to-from)*float64(i)/float64(steps-1)
			changed, _ := car.With(parameter, value)
			result, err := solver.Evaluate(scene, changed, line.Offsets, opts)
			if err != nil {
				return fmt.Errorf("%s=%g: %w", parameter, value, err)
			}
			row := []string{parameter, strconv.FormatFloat(value, 'g', -1, 64), strconv.FormatFloat(result.Duration, 'f', 9, 64), strconv.FormatFloat(result.Duration-line.Duration, 'f', 9, 64), "fixed optimized line"}
			if err := c.Write(row); err != nil {
				return err
			}
		}
		c.Flush()
		return c.Error()
	}
	if a.out != "" {
		return output(a.out, write)
	}
	return write(stdout)
}
