# Search quality and performance

The search remains a deterministic heuristic. It accepts feasible improvements
under identical road, car and boundary conditions, then compares a bounded
shortlist and the centreline on a road sampled at 0.5 m or finer. Its timing is
an estimate from the documented quasi-static model, not a global optimum.

`solver.Options.Workers` controls independent final-candidate evaluations:
zero uses up to four native workers, one is sequential, and values through 32
are accepted. Results are consumed in their original fixed order. Coordinate
search updates remain sequential because the next candidate depends on the
last accepted candidate. Browser/WASM execution always uses one worker and
retains cooperative yields to the browser event loop. Replacement `vehicle.Model`
implementations remain sequential because their interface does not promise
thread safety. Cancellation stops scheduling and waits for active workers;
workers do not outlive a solve.

`Options.Polish` adds zero through three bounded sweeps of 34 small smooth
lateral-offset perturbations. The default is zero, retaining the established
fixture results. Polishing preserves the original final shortlist and adds up
to four polish finalists, so extra search cannot discard a previously better
verified line. A supplied feasible seed and the verified centreline also remain
fallbacks.

JSON results expose `candidates`, `termination`, `coarse_duration`,
`selected_coarse_duration`, `refinement_difference_percent`, `fine_candidates`,
`search_workers`, and `polish_candidates`. `coarse_duration` is the coarse search
winner. `selected_coarse_duration` is the coarse evaluation of the line selected
on the fine road. The signed refinement difference is
`100 * (fine_time - selected_coarse_time) / fine_time`. This is **measured
coarse-to-fine agreement**, not statistical confidence or an accuracy guarantee.
Direct evaluation has no separate coarse timing; both duration fields equal its
verified duration and the difference is zero.

The built-in car caches immutable road-frame projections and returns compact
acceleration/braking bounds during profile work. Lateral speed ceilings avoid
solving longitudinal axle limits they never use. A strictly feasible power or
hardware brake cap avoids an unnecessary tyre-boundary root. Optional tyre load
sensitivity uses the same positive-base power law through `Exp/Log`; comparison
against the general library power and the independent axle oracles bound the
floating-point difference. Legacy optional-field defaults retain their exact
recorded trajectories.

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
