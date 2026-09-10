package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/TheFellow/the-line/pkg/track"
)

func runImport(args []string, stdout, stderr io.Writer) error {
	f := flag.NewFlagSet("import", flag.ContinueOnError)
	f.SetOutput(stderr)
	csvPath := f.String("csv", "", "centreline CSV file, metres")
	out := f.String("out", "", "output scene JSON file")
	name := f.String("name", "Imported centreline", "scene name; credit source in your study")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 || *csvPath == "" || *out == "" {
		return fmt.Errorf("import requires --csv input.csv and --out scene.json")
	}
	file, err := os.Open(*csvPath)
	if err != nil {
		return err
	}
	defer file.Close()
	scene, err := track.ImportCSV(file, *name)
	if err != nil {
		return err
	}
	if err = track.Save(*out, scene); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Imported %d centreline controls to %s\n", len(scene.Points), *out)
	return nil
}
