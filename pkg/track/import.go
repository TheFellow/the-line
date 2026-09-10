package track

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ImportCSV reads a centreline in SI metres. Required columns are x,y,z,
// width_left,width_right; optional columns are bank (degrees) and surface.
// Headers may be reordered. Unknown/duplicate columns and missing values fail.
// The returned v2 road is validated before the caller can replace an editor scene.
func ImportCSV(r io.Reader, name string) (Scene, error) {
	data, err := io.ReadAll(io.LimitReader(r, (2<<20)+1))
	if err != nil {
		return Scene{}, fmt.Errorf("import centreline: %w", err)
	}
	if len(data) > 2<<20 {
		return Scene{}, fmt.Errorf("import centreline: CSV exceeds 2 MiB")
	}
	reader := csv.NewReader(bytes.NewReader(data))
	header, err := reader.Read()
	if err != nil {
		return Scene{}, fmt.Errorf("import centreline header: %w", err)
	}
	columns := map[string]int{}
	for i, h := range header {
		h = strings.TrimSpace(h)
		if _, exists := columns[h]; exists {
			return Scene{}, fmt.Errorf("import centreline: duplicate column %q", h)
		}
		switch h {
		case "x", "y", "z", "width_left", "width_right", "bank", "surface":
		default:
			return Scene{}, fmt.Errorf("import centreline: unknown column %q", h)
		}
		columns[h] = i
	}
	for _, h := range []string{"x", "y", "z", "width_left", "width_right"} {
		if _, ok := columns[h]; !ok {
			return Scene{}, fmt.Errorf("import centreline: missing column %q", h)
		}
	}
	scene := Scene{Version: Version, Name: name, Vehicle: "road", EntrySpeed: 30, ExitSpeed: 50}
	for row := 2; ; row++ {
		record, e := reader.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return Scene{}, fmt.Errorf("import centreline row %d: %w", row, e)
		}
		p := Point{Surface: "asphalt"}
		for key, target := range map[string]*float64{"x": &p.X, "y": &p.Y, "z": &p.Z, "width_left": &p.WidthLeft, "width_right": &p.WidthRight, "bank": &p.Bank} {
			if index, ok := columns[key]; ok {
				value, e := strconv.ParseFloat(strings.TrimSpace(record[index]), 64)
				if e != nil || !finite(value) {
					return Scene{}, fmt.Errorf("import centreline row %d: invalid %s", row, key)
				}
				*target = value
			}
		}
		if index, ok := columns["surface"]; ok {
			p.Surface = strings.TrimSpace(record[index])
		}
		scene.Points = append(scene.Points, p)
		if len(scene.Points) > 1000 {
			return Scene{}, fmt.Errorf("import centreline: at most 1000 control points")
		}
	}
	if _, err := SampleRoad(scene, 2); err != nil {
		return Scene{}, err
	}
	return scene, nil
}
