# Watching and studying a run

The **Watch** button opens a third, genuine perspective view. It uses a fixed
trackside camera, automatically composed along the road's long axis. The road,
kerbs, racing line and cars use the same world coordinates as the edit views;
a near plane and reciprocal-depth buffer handle visibility. The car body follows
local bank and grade. **Edit view** returns to elevated orthographic editing.
Geometry dragging, orbit, pan and geometry keyboard edits are unavailable while
watching. Transport controls and charts remain active.

**Sectors** replaces the lower sidebar with splits at the original geometry
control points. Each row reports current time, reference time and current minus
reference. Negative green numbers mean time gained. The highlighted row follows
the current road station; long lists scroll around that row. A closed road also
includes the sector from the last control back to the seam. A stale reference
cannot produce sectors.

The telemetry strip's delta bar uses the same-station time difference, with a
fixed scale of −2 to +2 seconds. The numeric label remains exact when the bar
saturates. It does not compare cars' positions at shared elapsed time. The ghost
still does that independently.

| Control | Result |
| --- | --- |
| Comma / period, or **‹ Frame / Frame ›** | Pause and step by exactly 1/60 second |
| Shift + comma / period, or **‹ 5 m / 5 m ›** | Pause and step by five metres of road station |
| Drag the chart | Pause and inspect a road station |
| Drag the timeline | Pause and inspect elapsed time |

These presentation gestures create no scene edits and trigger no solve. Open
sequence transport clamps at its endpoints. Closed-lap playback continues with
each car following its own lap clock.

## Comparison export

```sh
./bin/the-line solve --preset esses --vehicle gt --format comparison-csv --out artifacts/comparison.csv
./bin/the-line render --preset banked --vehicle gt --view perspective --time 4 --out artifacts/trackside.png
```

`comparison-csv` writes both trajectories at every current-line **road station**.
Columns have `current_` and `reference_` prefixes; each retains its own time,
travelled path distance, position, speed, lateral offset and model tyre-force
channels. `delta_s` is current time minus reference time. Missing force telemetry
is empty, not invented zero. A saved pinned reference supplies the reference;
otherwise it is the current car's verified centreline. A saved reference on a
different physical road is rejected. Floating point fields retain 17 significant
digits. Existing `--format csv` remains the single-trajectory export.

## Verification

`pkg/render/study_test.go` checks analytical constant-speed sector times and the
closing sector. `pkg/render/perspective_test.go` checks inverse-distance apparent
size, visibility independent of triangle submission order, near-plane clipping
and frame isolation. `internal/cli/comparison_test.go` uses unequal path distances
to prove CSV alignment uses road station. The real-input browser helper
`tools/browser/presentation.mjs` covers stepping, sector sums and watch-only input.
