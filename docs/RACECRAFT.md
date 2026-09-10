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
resolution limit. The experiment ends at the first finish so neither car parks
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
