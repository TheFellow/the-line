# Independent roadmap correctness review

Reviewed 2026-09-10 during integration of `research/NEXT_STEPS.md`. This was a
fresh, bounded source and public-API review of geometry, asymmetric-width
migration, kerb force assignment, periodic seams, force telemetry, reference
identity and study persistence. It is not a replacement for the final full test
matrix or the root agent's headless visual and interaction acceptance.

## Corrected finding: tapered asphalt can hide kerb contact

The cross-section check `offset + radius > width_left` does not establish that a
circular footprint stays on asphalt. At a taper, the actual sloping asphalt edge
can lie closer than the width measured along the sampled road normal. This is
the same distinction already recognized by the solver's legal-edge clearance
check.

An independent public-API probe used a straight road with controls at x = 0, 10
and 60 m; left asphalt widths of 8, 2 and 2 m; a constant 6 m right width; and a
2 m legal ice kerb. A 2 m wide car followed offsets `left_width - 1.26` with the
default 0.25 m safety margin. Before correction, a node at station 2.892 m
reported asphalt grip 1.0 despite being only 0.994432 m from the asphalt edge,
inside the car's physical 1 m radius.

The corrected geometry evaluator tests each candidate segment's circular
clearance footprint against the actual asphalt-edge segments, including nearby
cells, and charges the least touched kerb grip. A spatial index keeps the check
local on finely sampled roads. Existing conservative adjacent-grip handling is
retained. Decorative kerbs excluded from the legal road bypass this check.

The regression independently reconstructs the straight-road edge equation
`y = m*x + b`, verifies its perpendicular-foot distance, and checks exported
grip wherever the physical car radius crosses that edge. It also compares every
node and the exact duration of an excluded-kerb scene with its bare-asphalt
counterpart. The original probe now finds no missed contact.

Validation: `go test ./pkg/solver -run
'TestTaperedAsphalt|TestAsymmetricClearance|TestClosedCircle' -count=1` passed.

## Other reviewed contracts and remaining limits

- Symmetric v1 roads and their v2 migrations retain identical physical-road
  digests. Digest compatibility deliberately excludes car setup and speed caps;
  it does not imply equal realized entry states.
- A closed trajectory repeats the first physical state at its final station,
  while retaining accumulated distance and time. Periodic queries wrap each
  trajectory using its own duration; station queries remain single-lap queries.
- Live reference snapshots retain the solver's force model, allowing fresh force
  balance at interpolated time/station queries. Archived JSON without a live
  model uses the documented interpolated telemetry approximation.
- Kerbs are flat banked-ribbon extensions with conservative surface grip. This
  model does not describe kerb bumps, suspension response or individual wheels.
  Clearance remains a horizontal circular footprint, not a rectangular swept
  body.
- Optional axle dynamics remain quasi-static, with prescribed lateral force
  allocation. They do not add tyre slip, steering-rate dynamics, lateral load
  transfer or crest unloading. Heuristic search is not a proof of a globally
  optimal line.

No additional reproducible high-impact correctness defect was identified in
this bounded review. Performance changes were still being integrated separately;
their complete numerical regression and rendered acceptance are recorded by
the final integration workflow.
