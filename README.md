# The Line

A Go racing-line studio for designing corner sequences, comparing vehicles, and watching a computed line unfold. It uses Ebitengine, following the adjacent `fluid` project, with a display-independent geometry, dynamics, optimization and rendering core.

The solver searches for a faster line across the complete sequence, with a speed profile constrained by tyre grip, power, mass, fixed front/rear drive distribution, braking, drag, banking, grade and surface changes. Results are **heuristically optimized estimates**, not a proof of the global optimum or a calibrated vehicle simulation.

## Run

Requires Go 1.24 or later. The live studio requires a graphical desktop; the CLI and its PNG/GIF exports do not.

```sh
make build
./bin/the-line-studio
```

Or run directly:

```sh
go run ./main/gui
go run ./main/cli presets
```

## Live editor

Hold the left mouse button on a numbered circular handle, drag it, and release to recompute the line. Handles highlight under the pointer; amber road edges preview the change while dragging. Press Escape to cancel a drag. Switching views, losing focus, or leaving the canvas also cancels an unfinished gesture. The road surface itself is not a drag target. Use the sidebar to change width, bank, elevation and surface, add or delete controls, switch vehicles and cycle the built-in sequences. Playback continues while a background solve runs. A rejected edit restores the last valid scene and its undo/redo history. Dragging changes the ground-plane position while preserving elevation; use the elevation controls to change height. Right-drag or Alt+left-drag to orbit the elevated camera; middle-drag or Shift+left-drag pans, and the wheel zooms. Fit / Reset restores the framing. Camera gestures start in the road viewport and preserve geometry and undo history. Tab switches between plan and elevated views.

| Control | Action |
| --- | --- |
| Space / playback button | Pause or resume |
| Timeline | Scrub shared elapsed time |
| Speed chart | Inspect the cars at the same road station; drag to seek and pause |
| Right-drag / Alt+left-drag | Orbit the elevated camera |
| Middle-drag / Shift+left-drag | Pan in either view |
| Mouse wheel / Fit button | Zoom / reset camera |
| Ghost button | Toggle the centreline reference and its longer playback timeline |
| Speed button | Cycle 1×, 2×, 0.25× and 0.5× playback |
| Tab / view button | Switch plan and elevated views |
| Home | Restart the sequence |
| `[` / `]` | Select the previous / next control |
| Arrow keys | Move the selected control by one metre |
| N / Delete | Insert / delete a control |
| Cmd/Ctrl+N | Start a new sequence |
| Cmd/Ctrl+S / Cmd/Ctrl+O | Save / load the displayed file path |
| Cmd/Ctrl+Z / Cmd/Ctrl+Shift+Z | Undo / redo |
| Escape | Cancel an active geometry, camera or scrubbing gesture; otherwise close the studio |

Click the file path to edit it, use Cmd/Ctrl+A to clear it, and Enter to confirm. Vehicle JSON examples and editable corner fixtures are in [examples](examples/). Custom vehicle files can be used with CLI solving/rendering; the live vehicle button cycles the built-in presets.

The cyan outlined ghost is the verified centreline reference at the **same elapsed time**. The speed chart compares both trajectories on a shared road-distance and speed scale. Its signed delta is optimized time minus reference time at the **same road station**: negative means the optimized line is ahead. Click or drag the chart to investigate an apex, braking approach or exit; use slow motion to watch the two cars separate.

With the ghost enabled, playback continues until both cars finish; the faster car stays at its open-sequence endpoint and the timeline marks its finish. Disabling the ghost restores the optimized-only duration. The sidebar shows power-to-weight, grip multiplier and requested versus realized endpoint speeds. Equal speed caps can produce different actual entry speeds, so the comparison is not necessarily an equal-start race. These remain illustrative vehicles and a quasi-static model.

JSON now includes both trajectories and a shared `station` field; CSV appends `station_m` while preserving existing columns. [Trajectory queries and comparison exports](docs/TRAJECTORIES.md) explains distance, interpolation and delta conventions. PNG/GIF exports include the same comparison composition; complete GIFs run through the reference finish.

## CLI

```sh
# Save editable corner geometry.
./bin/the-line new --preset esses --out corners.json
./bin/the-line validate --scene corners.json

# Compute the line and time-indexed telemetry.
./bin/the-line solve --scene corners.json --vehicle gt --out line.json
./bin/the-line solve --scene corners.json --vehicle rally --format csv --out line.csv

# The same studio composition, without a window.
./bin/the-line render --scene corners.json --view 2d --out corners.png
./bin/the-line render --preset banked --view 3d --time 5 --out banking.png
./bin/the-line animate --preset rally --view 3d --width 960 --height 600 --fps 20 --out rally.gif
```

`--spacing` controls search discretization; the selected candidates and baseline are re-evaluated at 0.5 m or finer before export. `--iterations` controls the finite search budget. `--margin` adds clearance beyond half the vehicle width. Use `--vehicle-file car.json` to supply a custom vehicle configuration. `--help` on each command lists its options. JSON exports include the scene, vehicle and complete solver result; CSV exports include distance, time, position, speed, curvature, lateral offset and acceleration.

The bundled tracks are `hairpin`, `esses`, `compound`, `banked` and `rally`. Their geometry and vehicle presets are fictional, designed for exploration. The rally sequence transitions from asphalt through gravel to dirt. Surfaces are categorical: the solver anticipates reduced grip when braking instead of simply recolouring the road.

## Geometry and vehicle data

Scenes use versioned JSON with an ordered set of controls. Coordinates and horizontal width are in metres; z points up. Positive bank raises the left edge of the road, so it is adverse banking for a left turn. Both views use the same banking and elevation in the dynamics. The elevated view projects the actual banked road ribbon.

```json
{
  "version": 1,
  "name": "My corner",
  "vehicle": "road",
  "entry_speed_cap": 25,
  "exit_speed_cap": 40,
  "points": [
    {"x": 0, "y": 0, "z": 0, "width": 12, "bank": 0, "surface": "asphalt"},
    {"x": 70, "y": 0, "z": 0, "width": 12, "bank": -5, "surface": "asphalt"},
    {"x": 110, "y": 40, "z": 3, "width": 12, "bank": -5, "surface": "asphalt"},
    {"x": 110, "y": 100, "z": 5, "width": 12, "bank": 0, "surface": "asphalt"}
  ]
}
```

Entry and exit speeds are **caps**, in metres per second. The solver may select a slower actual entry to meet downstream braking constraints. The animation repeats an open sequence by restarting; it does not simulate a periodic closed lap.

Vehicle configuration uses SI units: mass in kg, power in W, braking acceleration in m/s², top speed in m/s, width in m, drag area in m², and dimensionless grip, front weight and front torque fractions. `vehicle.Model` permits replacing the available acceleration/braking envelope without changing geometry or the optimizer. Static load and torque distribution distinguish front-, rear- and all-wheel drive; the first model does not include transient weight transfer, slip angles, drifting, suspension, crest unloading or jumps.

## Development

```sh
make test
make vet
make verify

# Real mouse/keyboard tests in headless Chrome; no desktop windows.
make verify-headless

# Exercise the live editor and capture actual Ebitengine output.
./bin/the-line-studio --demo --frames 240 --file artifacts/edited.json \
  --capture artifacts/editor.png --report artifacts/editor-report.json
```

Headless verification requires Node.js and Chrome (automatically found in its standard macOS location; set `LINE_CHROME_PATH` elsewhere). It builds the same Go editor for WebAssembly with a read-only inspection bridge, sends real browser input through Ebitengine, and writes screenshots and a JSON report under `artifacts/browser/`. For focused iteration, run `node tools/browser/verify.mjs --scope=analysis --case=elevated-retina`; the default runs both the original interaction checks and the camera/comparison checks in both configurations.

Measured solver accuracy, native frame rates, and verification commands are recorded in [docs/VALIDATION.md](docs/VALIDATION.md).

The independent pre-implementation research review lives in [research/CRITIQUE.md](research/CRITIQUE.md), alongside [sources and design rationale](research/README.md). The public packages separate `track`, `vehicle`, `solver` and `render`; `internal/editor` owns transactional editing and `internal/cli` owns commands. Executable wiring is under `main/cli` and `main/gui`.

The [fresh enthusiast review](docs/ENTHUSIAST_REVIEW.md) records the rationale and acceptance criteria for camera control, the centreline ghost and shared-station analysis.
