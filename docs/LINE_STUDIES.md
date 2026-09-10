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
caps and realized entry speeds can affect comparisons; these are cap-constrained
model estimates, not equal-entry-state lap claims.

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
version, sampled manual offsets, and a complete pinned source scene, car, speed
caps and offsets. Load reevaluates the exact saved pin under its original car and
caps, so its name never stands in for a different run. Native builds save atomic
files. Browser builds use the same strict JSON in origin-local browser storage,
keyed by the visible path; that storage is separate from native files. The example
[`manual-study.json`](../examples/manual-study.json) contains an authored centreline
and its named reference on a short fictional sequence.

Manual interpolation uses the solver's bounded latent C2 cubic. Vehicle clearance
is the existing circular centre margin; evaluation retains the model's force and
surface checks. The authoring tool does not add steering dynamics, slip, or a
calibrated driver model. These remain open-sequence, heuristic model estimates.
