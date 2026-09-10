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

JSON solve exports include `result.nodes` and `result.center_nodes`, each with the new `station` field. CSV continues to export the selected trajectory, appending `station_m` after the existing columns so their positions and meanings stay intact:

```text
s_m,time_s,x_m,y_m,z_m,speed_mps,curvature_per_m,offset_m,acceleration_mps2,station_m
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
