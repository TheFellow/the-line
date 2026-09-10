# Setup workbench

Click **Setup** above the vehicle name to switch the sidebar to nine SI-backed
steppers: mass, power, braking capacity, top speed, tyre grip, drag area, front
weight fraction, fixed front drive fraction, and vehicle width. The displayed
power is kW and speed is km/h; JSON and CLI parameters remain SI. Front weight
and drive fractions set the static load and fixed torque split. The second page
adds brake bias, downforce balance, longitudinal load transfer and load sensitivity;
see [vehicle assumptions](VEHICLE_MODEL.md).

Each accepted setup change shares the geometry undo/redo history. Validation
reports the field that failed and leaves the previous setup and histories intact.
The most recently edited **Custom** car is retained in the vehicle cycle after
Road / GT / Rally. Reset restores the scene's named preset.

**Save** writes the custom config inline as optional `vehicle_config` in the
scene JSON. **Load**, GUI `--scene`, and CLI `--scene` restore that setup. A CLI
`--vehicle` or `--vehicle-file` explicitly overrides it. Existing version 1
scenes without the optional field retain their preset. **Save car / Load car**
exchange a standalone vehicle JSON at the editable scene path plus `.car.json`;
that file is independent of the saved study. Scene and car writes are atomic.

A setup edit first evaluates the existing path under the new model asynchronously.
The amber **Provisional** message identifies this verified fixed-path result;
then a seeded heuristic search can replace it with a faster verified path. A
slower search result keeps the provisional path. New edits cancel pending work,
and generation IDs reject stale replies. If the old line no longer fits a wider
car, a fresh search starts without it. Only a rejected warm-start seed triggers
an unseeded retry; unrelated failures are not repeated.

Each committed edit owns its rollback checkpoint. If a second edit fails while
superseding an unfinished first edit, only the second edit is discarded and the
first solve resumes. Its setup and history survive; accepted replies carry the
scene that was actually solved. A rejected edit cannot reappear through redo.

Click **Analyze** for one-sided finite differences on this same line: +50 kg,
+25 kW, +0.05 tyre grip, and +0.1 m² drag area. These are fixed-path effects, not
predictions of separately optimized runs. Analysis refreshes lazily after a setup
solve while the workbench is open.

```sh
bin/the-line sweep --preset esses --vehicle gt --parameter grip \
  --from 1.2 --to 1.6 --steps 9 --out artifacts/grip-sweep.csv
```

The sweep searches one initial line, then evaluates all requested parameter
values on it. Its CSV records SI values, duration, and changed-minus-original
time. Invalid inputs fail before the initial solve; `--out` replaces output only
when the complete sweep succeeds.

## Numerical API and measured latency

`solver.Evaluate` and `EvaluateContext` take offsets at
`track.SampleRoad(scene, opts.Spacing)` stations. Nil offsets select the centreline.
Final evaluation uses at most 0.5 m sampling, the same geometry, clearance,
force residual and speed-cap checks as the optimizer. `Result.Offsets` contains
independent offsets at `Result.Road`; evaluating them with
`Spacing: result.Spacing` exactly reproduces the exported duration and nodes.
`Options.Seed` uses the requested search spacing. It must be feasible there and
on the final verification road. A positive-budget seeded search retains both
seed and centreline fallbacks; zero iterations evaluates the seed unchanged.

`SolveContext` and `EvaluateContext` check cancellation during bounded profile
work units. `Result.OffsetsAt` samples a result for a new search grid; it does not
claim exact geometry preservation across different road discretizations.

Measured native esses fixed-line evaluation in focused tests: **114–125 ms**,
including fresh road sampling, optimized and centreline profiles, and final
force checks. This does **not** achieve the roadmap's one-frame-result target.
The UI remains asynchronous; no force verification is skipped to report a
misleading latency. Browser interaction and visual acceptance are recorded in
`docs/VALIDATION.md` after full integration.
