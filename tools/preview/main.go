// Preview rebuilds the README's qualifying and racecraft animation through the
// same CLI renderer used by normal exports. Intermediate clips stay ignored.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"

	"github.com/TheFellow/the-line/internal/cli"
)

func main() {
	out := flag.String("out", "docs/media/club-loop.gif", "combined GIF output")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(out string) error {
	dir := "artifacts/preview"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	clips := [][]string{
		{"animate", "--preset", "club-loop", "--vehicle", "gt", "--view", "3d", "--duration", "6"},
		{"race", "--scenario", "over-under", "--format", "gif", "--view", "2d"},
		{"race", "--scenario", "pass-repass", "--format", "gif", "--view", "3d"},
	}
	combined := gif.GIF{LoopCount: 0}
	for i, args := range clips {
		file := filepath.Join(dir, fmt.Sprintf("%d.gif", i))
		args = append(args, "--fps", "12", "--width", "960", "--height", "600", "--out", file)
		if err := cli.Run(args, os.Stdout, os.Stderr); err != nil {
			return err
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		clip, err := gif.DecodeAll(f)
		f.Close()
		if err != nil {
			return err
		}
		// Each clip begins with a complete opaque frame and retains its own palette.
		var previous []byte
		for j, im := range clip.Image {
			// Reserve one transparent index, remapping its rare opaque colour to the
			// closest remaining entry. Unchanged pixels then compress to transparent
			// runs without relying on an external GIF optimizer.
			palette := append(color.Palette(nil), im.Palette...)
			reserved := len(palette) - 1
			replacement := palette[:reserved].Index(palette[reserved])
			current := append([]byte(nil), im.Pix...)
			for k, p := range current {
				if int(p) == reserved {
					current[k] = uint8(replacement)
				}
			}
			delta := image.NewPaletted(im.Bounds(), palette)
			palette[reserved] = color.RGBA{}
			for k, p := range current {
				delta.Pix[k] = p
				if previous != nil && p == previous[k] {
					delta.Pix[k] = uint8(reserved)
				}
			}
			previous = current
			im = delta
			combined.Image = append(combined.Image, im)
			delay := clip.Delay[j]
			if j == len(clip.Image)-1 {
				delay += 120
			}
			combined.Delay = append(combined.Delay, delay)
			combined.Disposal = append(combined.Disposal, gif.DisposalNone)
		}
	}
	f, err := os.CreateTemp(filepath.Dir(out), ".preview-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := gif.EncodeAll(f, &combined); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), out)
}
