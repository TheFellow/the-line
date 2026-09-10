# Trajectory queries and comparison exports

`solver.Result.Nodes` contains the selected, densely verified trajectory. `CenterNodes` contains the centreline trajectory verified on the same final road, with the same vehicle model, surfaces, circular clearance and requested endpoint speed caps. It is retained directly from the baseline evaluation, without another search. `CenterDuration`, `CenterEntrySpeed` and `CenterExitSpeed` describe those nodes. Search disabled (`--iterations 0`) selects that same baseline.

Each node carries two distances, both in metres:

- `S` (`s` in JSON, `s_m` in CSV) measures actual three-dimensional distance along that particular trajectory.
- `Station` (`station` in JSON, `station_m` in CSV) measures reference-road distance along the sampled three-dimensional centreline. Both trajectories share this coordinate's start and end. Road sampling anchors carry their reference distance; inserted authoritative triangle crossings interpolate station across their road cell. Between trajectory nodes, station varies linearly with actual segment distance.

Compare trajectories at the same **road station**, rather than their separate path distances, node indices or percentage complete. `Result.AtStation(station)` and `Result.CenterAtStation(station)` return `(Node, error)`. They clamp finite out-of-range queries to the open endpoints and reject NaN, infinity or an empty trajectory. Position varies linearly on the represented straight surface segment. Speed follows squared-speed kinematics and time follows constant acceleration; these queries preserve launch and stop behavior at zero speed.

For a shared station, the displayed delta is **optimized elapsed time minus centreline elapsed time**. A negative delta means the optimized line is ahead. At the exit it equals `Duration - CenterDuration`. Equal requested caps can produce different realized entry speeds, so this compares the same cap-constrained optimization problem, not necessarily identical initial states.

```go
optimized, err := result.AtStation(120) // reference-road metres
if err != nil {
    return err
}
reference, err := result.CenterAtStation(120)
if err != nil {
    return err
}
deltaSeconds := optimized.Time - reference.Time
```

For a same-time ghost, query `Result.At(seconds)` and `Result.CenterAt(seconds)` independently. They clamp to each trajectory's endpoints, including infinite times; NaN selects the entry and an empty trajectory returns a zero node. They never wrap an open road. The viewer decides when to restart after the longer run finishes.

JSON solve exports include `result.nodes` and `result.center_nodes`, each with the shared `station` field and `tyre_forces`. CSV exports the selected trajectory. The first ten columns keep their positions; instrumentation columns are appended:

```text
s_m,time_s,x_m,y_m,z_m,speed_mps,curvature_per_m,offset_m,acceleration_mps2,station_m,bank_deg,grade,grip,tyre_lateral_mps2,tyre_longitudinal_mps2,tyre_capacity_mps2,tyre_utilization,phase,limit,forces_available
```

Export both trajectories and a selected-path spreadsheet using:

```sh
bin/the-line solve --preset esses --out artifacts/esses-comparison.json
bin/the-line solve --preset esses --format csv --out artifacts/esses-selected.csv
```

These additions expose the existing quasi-static heuristic's verified output. They do not change force assumptions, search decisions or integrated durations, and the illustrative vehicle presets remain uncalibrated.

## Fixed-line evaluation and setup sweeps

Results additionally export `offsets`, one horizontal lateral offset per final
road sample (not per trajectory node: triangle crossings add trajectory nodes).
Call `solver.Evaluate(scene, car, result.Offsets, Options{Spacing: result.Spacing,
Margin: originalMargin})` to reproduce the path profile exactly. `Options.Seed`
contains offsets at the requested search spacing. `SolveContext` and
`EvaluateContext` support cancellation. See [the setup workbench](SETUP.md) for
fixed-path sensitivity semantics and CLI examples.

## Model tyre-force channels

Each node carries `bank_deg`, actual candidate-segment `grade` (rise/run), and the conservative segment `grip`. `tyre_forces` contains signed `lateral_mps2`, signed `longitudinal_mps2`, `capacity_mps2`, combined `utilization`, `phase`, and `limit`, together with the model envelope's drive/braking capacities. These are forces divided by vehicle mass, not measured chassis accelerations or pedal positions. Displayed g channels divide these specific forces by 9.80665 m/s².

Longitudinal tyre demand is the outgoing segment's net acceleration **plus** drag and signed grade resistance. A car can need positive tyre force while maintaining constant speed uphill, and can decelerate while coasting. Lateral demand includes bank gravity; it can be nonzero on a banked straight. Total utilization is `hypot(longitudinal, lateral) / capacity`. It is not clamped to hide numerical residuals. A driven axle can saturate while total utilization is below one.

Interior nodes use the outgoing segment's acceleration, grade and minimum adjacent grip. The terminal node has no outgoing segment, so its acceleration and forces describe the final incoming segment at its endpoint speed. Bank and curvature are evaluated at the node. Live `At` and `AtStation` queries recalculate force channels from the retained model at the interpolated speed and geometry; they do not interpolate forces from different outgoing accelerations. A result decoded from JSON without its runtime model can only interpolate the archived channels approximately; load its scene and car and reevaluate the line for exact model queries.

`phase` is `drive`, `brake` or `coast`, using a ±0.05 m/s² dead band on longitudinal **tyre** demand. `limit` identifies a nearby active bound: `grip`, `power`, `speed_cap`, or `brake`. `none` means no local bound is active; another segment can constrain the profile. `available: false` and `unknown` labels explicitly cover replacement models that do not expose physical force channels. CSV uses `forces_available` and preserves float64 values with 17 significant digits.

The **Model tyre forces** widget shows the force dot inside the same model's axle- and power-limited envelope. Its horizontal axis is signed lateral force; vertical up is drive force and down is braking. The outlined dot is the reference car at the same elapsed time as the ghost. Chart comparison remains at the same road station. Both use the displayed trajectory clock.

The **COLOR** control cycles fixed absolute scales: speed 0–300 km/h, utilization 0–1, lateral −2–+2 g, and longitudinal −1.5–+1.5 g. Values outside these ranges use the endpoint color; the legend states this clipping. **PLOT** selects the corresponding chart channel without changing the line or starting a solve. Equal physical values use equal colors across cars. CLI render/animation exports expose the same controls:

```sh
bin/the-line render --preset hairpin --vehicle gt --color utilization --channel utilization --out artifacts/grip.png
bin/the-line render --preset banked --view 3d --color lateral_g --channel longitudinal_g --out artifacts/forces.png
```

Road and chart ticks identify **B** (sustained braking after a drive run), **A** (a prominent local speed minimum in a corner), and **T** (sustained positive tyre force after that minimum). T is model drive resumption, not a measured throttle application. Detection uses a fixed 0.25 m station grid, a 2 m speed window and 2 m sustained force phases. It suppresses tiny numerical extrema and draws no corner markers on a straight. Multiple genuine profile minima can produce multiple apex ticks in a compound corner. These remain heuristic study aids; their positions are not driving instructions.

See [instrumentation validation](INSTRUMENTATION.md) for measured checks and limitations.
