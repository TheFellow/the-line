# Search quality and performance

The search optimizes complete traversal time under identical road, car and
boundary conditions. A coarse coordinate search supplies a starting line after
comparing its shortlist and the centreline at 0.5 m or finer. It then **continues
optimizing at that final resolution**, using the same geometry, force checks,
speed profile and time integral as an authored line. Coarse candidate rankings
cannot veto improvements in this final stage.

Each requested iteration adds two final-resolution rounds (eight by default).
The first rounds try both lateral directions at 16 positions around the
sequence, followed by local changes at 32 positions. The default uses three
broad rounds and five local rounds, with periodic support across a closed lap's
seam. Perturbations have
continuous first and second derivatives at their support boundaries. The search
also combines successful directions and evaluates four amplitudes of the
combined change, allowing coordinated entry/apex/exit and chicane adjustments.
The local poll amplitude decreases from 0.2 m as rounds proceed; width bounds
are applied through smooth latent coordinates. Only improvements passing the
full evaluator survive. Zero iterations still means no search, unless optional
polishing is requested.

The GUI initially displays a verified centreline and searches in the background,
so painting and playback do not wait for optimization to finish. Setup changes
retain their existing provisional-line and cancellation behavior. CLI solves
wait for the complete result and still require no display.

This is a bounded deterministic search, not a proof of a global optimum. Its
quality is checked against independent physical-offset edits and the authoring
cubic, as well as centreline and previous-search baselines. Same-line refinement
and independently reconstructed forces check numerical consistency. All times
remain estimates from the documented quasi-static vehicle model.

`solver.Options.Workers` controls independent shortlist and final-search evaluations:
zero uses up to four native workers, one is sequential, and values through 32
are accepted. Results are consumed in their original fixed order. Coordinate
search updates remain sequential because the next candidate depends on the
last accepted candidate. Final-search polls use a common starting line, consume
results deterministically, and retain a bounded window of completed trajectories
instead of an entire poll's dense results. A work queue keeps native workers
occupied when some candidates reject quickly and others require full profiles.
Browser/WASM execution always uses one worker and
retains cooperative yields to the browser event loop. Replacement `vehicle.Model`
implementations remain sequential because their interface does not promise
thread safety. Cancellation stops scheduling and waits for active workers;
workers do not outlive a solve.

`Options.Polish` adds zero through three bounded sweeps of 34 small smooth
lateral-offset perturbations **after** the final-resolution search. Its default
is zero. Every extra sweep retains the best verified line, so increasing polish
cannot discard the unpolished result. A supplied feasible seed and the verified
centreline also remain fallbacks.

JSON results expose `candidates`, `termination`, `coarse_duration`,
`selected_coarse_duration`, `refinement_difference_percent`, `fine_candidates`,
`search_workers`, `refine_candidates`, `before_refine_duration`, and
`polish_candidates`. `coarse_duration` is the coarse search winner.
`selected_coarse_duration` is the coarse evaluation of the starting line selected
on the fine road. `before_refine_duration` is that starting line's fine time.
The signed refinement difference is
`100 * (before_refine_duration - selected_coarse_duration) / before_refine_duration`.
This measures the **starting line's coarse-to-fine agreement**, not optimization
gain, statistical confidence, or the final line's accuracy. The final-search gain
is `before_refine_duration - duration`, including any optional polish, and is
shown in the analysis panel. `refine_candidates` counts final-resolution search
evaluations; `fine_candidates` retains its shortlist-count meaning. Direct
evaluation sets all duration diagnostics to its verified time, with zero
refinement difference and zero search candidates.

The built-in car caches immutable road-frame projections and returns compact
acceleration/braking bounds during profile work. Lateral speed ceilings avoid
solving longitudinal axle limits they never use. A strictly feasible power or
hardware brake cap avoids an unnecessary tyre-boundary root. Optional tyre load
sensitivity uses the same positive-base power law through `Exp/Log`; comparison
against the general library power and the independent axle oracles bound the
floating-point difference. Exact historical coarse-stage fixture times remain
covered by golden tests; final times intentionally improve with the new search.

Every nonconstant segment starts with five envelope samples and is checked at
65 points before acceptance; failures raise that segment's working resolution
to 65 and recompute the speed profile. A constant speed/curvature/bank segment
has an identical envelope at every sample and evaluates it once. The final
convergence gate already holds the 65-point bounds at the exact final speeds,
so export reuses those values. This avoids duplicate work without reducing the
final verification density. Independent tests reconstruct forces separately,
including 129 samples on legacy trajectories and axle checks for richer models.

Reproduce profiling with:

```sh
go test ./pkg/solver -run '^$' -bench BenchmarkSolveEsses -benchtime=1x -cpuprofile artifacts/search.cpu
go tool pprof -top artifacts/search.cpu
go test ./pkg/solver -run '^$' -bench 'Benchmark(Solve|Evaluate).*Esses' -benchtime=1x
```

Run benchmarks without other solves or graphical checks for comparable timing.
Go's profile command may also write `solver.test` into the current directory;
remove that generated binary afterwards. The measured results and remaining
interactive latency limits are recorded in [VALIDATION.md](VALIDATION.md).

`candidates` counts actual path-profile evaluations, including supplied-line and
baseline work during seed verification. A verified seed is retained directly,
rather than evaluated again as a fine shortlist entry. A `solver.SeedError`
identifies a rejected warm start; the GUI retries only that failure without the
seed. Other failures return immediately. When a provisional line survives a
search, its displayed iteration and candidate counts describe the search that
was actually completed.
