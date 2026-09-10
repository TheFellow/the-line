# Compare setups and author a line

Click **Pin current** before changing a car setup. A is the current evaluated run;
B keeps the pinned trajectory, vehicle and original speed caps. The cyan ghost
uses shared elapsed time; the chart and signed delta use shared road station.
The exit delta equals A's duration minus B's duration. **Centreline** returns B
to the current car's centreline baseline.

A geometry, bank, width or surface change marks a pin **stale · different road**.
Its source remains saved, but ghost playback and station comparisons are disabled
until that road is restored or a new pin is chosen. Renaming a scene, changing a
car or changing the speed caps does not change physical-road identity. Different
caps and realized entry speeds can affect open-sequence comparisons. Closed laps
ignore endpoint caps and solve periodic speed profiles; each animated car wraps
at its own lap time. Station comparisons always describe one traversal of the
same physical road.

Click **Author line** to display sparse cyan diamond handles. The first authored
line follows the centreline and has exactly the baseline's evaluated time. Drag a
diamond across the road and release to evaluate its smooth lateral-offset curve.
The drag follows the banked road cross-section, including elevation, under both
plan and rotated elevated cameras. An off-centre grab does not jump. Ordinary
left drags belong exclusively to line handles; right/Option-drag still orbits and
Shift-drag pans. Escape, focus loss, leaving the canvas and switching views cancel
unfinished drags. No geometry or undo history changes until release.

The scene stores verified offsets. A rejected offset or infeasible profile keeps
the previous line and undo/redo history, reports the reason and road station,
and marks that station in red. **Reset line to centre** starts again;
**Optimize from here** seeds a background search with the evaluated line and
retains the faster verified result. **Adopt as reference** pins the authored run.
**Geometry** hides the authoring handles so numbered road controls can be edited.

Save writes the custom vehicle and study in the scene JSON. A study contains a
version, an `active_line` selection (`manual` or `optimized`), sampled manual
offsets, and a complete pinned source scene, car, speed
caps and offsets. Load reevaluates the exact saved pin under its original car and
caps, so its name never stands in for a different run. Native builds save atomic
files. Browser builds use the same strict JSON in origin-local browser storage,
keyed by the visible path; that storage is separate from native files. The example
[`manual-study.json`](../examples/manual-study.json) contains an authored centreline
and its named reference on a short fictional sequence.

CLI exports follow the saved active-line selection automatically. Older studies
without `active_line` select their manual line when present. `--line manual`
requires and evaluates the retained authored line; `--line optimized` runs a
fresh search even when the scene contains one. Optimization keeps the manual
hypothesis without selecting it, so a later setup change can use an optimized
line even when that hypothesis no longer fits a wider car. Geometry edits switch
an invalidated active manual line to optimized in the same undo transaction.
An explicitly selected stale manual line still produces an error, including
legacy files whose road changed outside the editor.
Manual evaluation uses its saved sampling spacing. All three commands share
this selection behavior:

```sh
./bin/the-line solve --scene examples/manual-study.json --line manual --out artifacts/manual.json
./bin/the-line render --scene examples/manual-study.json --view perspective --out artifacts/manual.png
./bin/the-line animate --scene examples/manual-study.json --view 3d --out artifacts/manual.gif
./bin/the-line solve --scene examples/manual-study.json --line optimized --format comparison-csv --out artifacts/comparison.csv
```

`comparison-csv` aligns current and reference times, path distances, positions,
speeds and force telemetry at shared road stations. It restores a saved pin with
its original vehicle and caps, or uses the current car's centreline when no pin
is saved. A stale pin rejects the comparison export and preserves any existing
output file. PNG and GIF exports restore the same pin; a stale pin is labeled
and its ghost is hidden.

For a closed scene, `animate` defaults to one complete current-car lap starting
at `--time`. An explicit `--duration` can span multiple laps, and `--time` can
start on any later lap. Each car retains its own lap clock across the seam. For
example, `--preset club-loop --time 30 --duration 45` exports 45 seconds starting
at elapsed time 30 seconds. Open sequences still stop at the last visible car's
finish and reject start times at or after it. All exports retain the frame and
pixel memory limits; reduce resolution or frame rate for long animations.

Manual interpolation uses the solver's bounded latent C2 cubic. Vehicle clearance
is the existing circular centre margin; evaluation retains the model's force and
surface checks. The authoring tool does not add steering dynamics, slip, or a
calibrated driver model. These remain heuristic quasi-static estimates for open
sequences or steady closed laps; see [closed laps](CLOSED_LAPS.md) for the seam
and periodic-profile assumptions.
