package cli

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"testing"
)

func TestSweepFixedLineAndValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"sweep", "--preset", "hairpin", "--parameter", "power", "--from", "100000", "--to", "150000", "--steps", "3", "--iterations", "0"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&stdout).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[0][1] != "value_si" {
		t.Fatalf("bad CSV: %v", rows)
	}
	first, _ := strconv.ParseFloat(rows[1][2], 64)
	last, _ := strconv.ParseFloat(rows[3][2], 64)
	if last > first {
		t.Fatalf("extra power slowed same line: %v", rows)
	}
	for _, args := range [][]string{{"sweep", "--steps", "1"}, {"sweep", "--parameter", "bogus"}, {"sweep", "--parameter", "mass", "--from", "-1"}} {
		if err := Run(args, &stdout, &stderr); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
