# Two-car racecraft

## Design and acceptance contract

Qualifying studies minimize estimated traversal time on an empty road. Racecraft
studies instead compare two cars sharing an open corner sequence. Car A starts
ahead; car B attacks. Both retain their identity when the order changes.

The first implementation is a deterministic, bounded tactical planner. Named
examples prescribe intentions (inside defence, over-under, pass and repass, or
side-by-side corner sequence), not a winning car or an animation timestamp.
Smooth lateral paths are evaluated by the existing vehicle/road solver. A finite
set of alternative placements is checked against the opponent's entire timed
trajectory. Unsafe pairs are rejected; no car is teleported through another.
If no safe pair exists, the experiment reports that explicitly and the GUI keeps
the last valid experiment. This is an offline teaching experiment, not an online
AI driver, a game-theoretic optimum, or a calibrated prediction of a real race.

Controls use SI units: initial centre-to-centre road-station gap, attacker entry
speed-cap advantage, lateral placement separation, and extra car-to-car clearance.
Requested entry caps and realized starting speeds are reported separately: the
solver can brake upstream and reduce an infeasible requested arrival speed.
Vehicle performance is shared, so tactical differences arise from placement and
boundary conditions. Changing a control can change the outcome or make a plan
infeasible; example titles do not force passes.

Each car has a 4.4 m long rectangular visual body and its configured width. A
circumscribed horizontal disc encloses that entire body, regardless of heading.
Road clearance uses that radius plus the existing margin. Collision validation
uses the sum of the two radii plus requested clearance, conservatively in XY
(elevation never permits one car to pass through another). An interval bound on
relative travel certifies clearance between samples; checking GIF frames alone is
insufficient. Ambiguous intervals are subdivided and rejected at the finite
resolution limit (1 microsecond) or 250,000 interval checks. The experiment ends at the first finish so neither car parks
in the other's path. Replay resets the complete experiment together.

A pass event requires the new leader to gain a body length in reference-road
station, with hysteresis; becoming momentarily nose-ahead is shown separately.
Telemetry includes both cars' speed, station, signed gap, minimum certified
clearance, and pass/repass events. A qualifying ghost remains a separate concept.

## Incremental delivery

1. Document this contract and acceptance criteria.
2. Add graphics-independent experiment types, tactical line evaluation,
   continuous collision certification, event detection, examples and tests.
3. Add display-independent rendering/exports and GUI mode/scenario/parameter
   controls with transactional rejection of unsafe edits.
4. Exercise real browser input and both rendered views; update the README and
   reproducible animated preview; record validation and open a PR.

Acceptance checks cover determinism, finite input validation, initial separation,
swept collision rejection (including between frames), road/body clearance,
independent force and kinematic consistency, over-under and pass/repass outcomes,
control sensitivity, identical-corner alternatives, qualifying regressions,
CLI round trips, GUI controls and both views. Exported examples and the public
preview must use the same planner and renderer as live playback.

## Using the lab

Press **R** or **Racecraft mode** in the studio. The qualifying study, camera and
analysis display are retained for your return. Racecraft has its own open-road
examples; its controls do not edit the qualifying track. **Next** cycles the four
examples. The initial three use the identical Switchback hairpin geometry.

The four pairs of buttons change gap by 1 m, entry-cap advantage by 1 m/s,
placement separation by 0.25 m, and extra clearance by 0.1 m. The initial car A
station is the requested gap, while B begins at station zero. Positive lateral
offsets are left of the road centreline. Placement separation is the nominal
distance between the two lane intentions, not a guaranteed constant lateral gap
through the crossover. The actual initial speeds and positions are visible in
the playback telemetry and exported trajectories.

Planning first tries the requested placements. Two bounded give-room alternatives
widen B's placement by 0.25 / 0.5 m, delay its crossover by 2 / 4 percent of road
length, and lower its entry speed cap by 2 / 4 m/s. Every alternative is evaluated
again through the vehicle solver, then collision-checked against A. The first
safe pair is selected, without optimizing or prescribing the winning car. The
intent label adds “give room” when an alternative is selected; JSON includes the
candidate count and each car's effective `entry_speed_cap`. There is no online
reaction to the opponent after this offline plan is accepted.

**Space** pauses, **, / .** steps 1/60 s, **Tab** changes 2D/3D, and dragging the
timeline scrubs both cars at the same race time. Scroll zooms, Shift-drag pans,
and right-drag orbits the elevated view. Cars retain their physical drawn size
when zoomed; their A/B labels identify them at wide zoom. Racecraft supports the
plan and elevated views. Trackside perspective remains a qualifying view.

**Save race** and **Load race** store the complete road, vehicle and race inputs
in `racecraft.json`, or the path given by `--race-file`. Native builds use atomic
files; the browser verification build uses origin-local storage, like the other
study types. Unsafe loaded experiments are rejected during planning. Loading or
editing never replaces the displayed experiment until a safe plan is available.
Race edits have save/load; qualifying's undo history belongs to that study.

### Headless exports

```sh
# Replay a checked-in, complete experiment.
go run ./main/gui --race-file examples/racecraft/pass-repass.json

# Override inputs, save the new experiment, and export both complete trajectories.
go run ./main/cli race --experiment examples/racecraft/over-under.json \
  --gap 7 --overspeed 2 --save-experiment my-race.json --out race-result.json

# Shared-time CSV includes both stations, speeds, signed gap and body clearance.
go run ./main/cli race --scenario pass-repass --format csv --out race.csv

# The GUI, still image and GIF share the same renderer.
go run ./main/cli race --scenario pass-repass --format png --time 11 \
  --view 2d --out race.png
go run ./main/cli race --scenario over-under --format gif --fps 20 \
  --view 3d --out race.gif
```

`--scene` and `--vehicle` override the road and shared car in CLI experiments.
The authored normalized placements are designed for the named example geometry;
a different road can yield different behavior or no safe plan. Arbitrary track
corner detection and automatically choosing tactical intentions are future work.

### Observed examples

Default road car, 2 m input sampling, existing solver verification at ≤0.5 m:

| Example | Initial gap | B entry advantage | Observed completed passes | First finish |
| --- | ---: | ---: | --- | ---: |
| Over-under | 6 m | 3 m/s | B at 12.92 s | 13.280 s |
| Pass-repass | 5 m | 8 m/s | B at 6.18 s; A at 12.70 s | 13.239 s |
| Defend | 14 m | 0 m/s | None; A holds | 12.716 s |
| Esses duel | 6 m | 4 m/s | None; nose-ahead advantage trades | 15.687 s |

Events are resolved at 0.02 s with a 4.4 m station hysteresis. Nose-ahead changes
are not completed passes. Increasing the default over-under gap to 7 m prevents
the completed pass before the finish. Setting its gap to 3.25 m and separation to
5 m occupies the preferred line: the second candidate widens and delays B's
crossover and reduces its arrival cap, producing a safe alternative. These are
regression fixtures, not guaranteed outcomes under arbitrary setup changes.

### Validation and limitations

The package tests reconstruct segment kinematics, query the public force envelope,
measure body clearance to all road-side segments, and sample car-to-car clearance
independently of the continuous validator. A high-speed crossing test has safe
endpoints but a collision inside a 10 ms interval; it must be rejected. Other
checks cover deterministic replay, altered outcomes, fallback placement, invalid
inputs, cancellation, strict experiment persistence, CLI exports, and failed
writes preserving earlier files. The standard solver's existing independent
force/refinement checks remain part of `go test ./...`.

The body envelope is deliberately conservative: two enclosing discs can reject
a close rectangle-to-rectangle pass that a richer collision model could allow.
No steering-rate, contact dynamics, drafting, driver reaction delay or sporting
rules are modeled. A circle guarantees body separation but not driver realism.
The baseline reference solver still uses its documented circular-width margin;
racecraft explicitly increases it to enclose the entire 4.4 m body.
