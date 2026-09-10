# Track authoring

Scenes save as version 2. Version 1 files still load: each horizontal `width`
becomes equal `width_left` and `width_right` halves. This migration does not
change the C2 sampled geometry or invalidate equivalent saved manual/pinned
studies. Names, cars and speed caps remain outside road identity; asymmetric
widths, kerb geometry/grip and the track-limit rule are included.

All distances are metres. Side widths are horizontal distances from the reference
centreline, not distances measured along the bank. Positive bank (degrees)
raises the left edge. Each side needs at least 1.5 m and their sum cannot exceed
80 m. Each kerb may add 0–8 m beyond its side:

```json
{
  "version": 2,
  "name": "Asymmetric kerb study",
  "vehicle": "road",
  "entry_speed_cap": 30,
  "exit_speed_cap": 50,
  "kerbs_count_as_road": true,
  "points": [
    {"x": 0, "y": 0, "z": 0, "width_left": 4, "width_right": 7,
     "bank": 0, "surface": "asphalt",
     "kerb_left": {"width": 2, "surface": "wet", "grip": 0.8}},
    {"x": 150, "y": 0, "z": 0, "width_left": 4, "width_right": 7,
     "bank": 0, "surface": "asphalt",
     "kerb_left": {"width": 2, "surface": "wet", "grip": 0.8}}
  ]
}
```

A positive kerb `grip` overrides its named surface; zero uses that surface.
An unspecified kerb surface has illustrative grip 0.8. The existing surface
names are asphalt, wet, gravel, dirt and ice. Widths interpolate smoothly;
surface identities and grip overrides belong to the outgoing control interval.
Kerbs are flat extensions of the banked ribbon, not simulated bumps. The
red/white strips use their actual configured widths and the same triangle
heights as the evaluated trajectory.

With `kerbs_count_as_road: false` (the default), the circular vehicle clearance
must remain inside the asphalt widths and kerb grip cannot change the evaluated
line. When enabled, legal side boundaries include kerbs. Any overlap of the
clearance footprint with a kerb uses the lower of asphalt and kerb grip,
conservatively including the cell endpoints. This is a circular footprint
approximation, not a four-wheel contact/suspension model.

In the studio, click **Road limits** above the selected point to adjust left and
right widths, each kerb's width/grip and the scene-wide track-limit rule. Edits
are validated, solved and undoable; **Geometry** returns to bank, height and
surface controls. File Save writes v2. The first five presets retain their
cycle order. Six fictional studies follow: decreasing-radius, banked-bowl,
compression, chicane, long-double-apex and blind-crest. They are not surveyed
tracks. Vertical studies illustrate geometry and grade; they do not model
suspension compression or crest unloading.

## CSV centreline import

CSV requires `x,y,z,width_left,width_right` headers in any order. Optional
`bank` (degrees) and `surface` columns are accepted. Unknown/duplicate headers,
nonfinite values, malformed rows, over 1,000 controls, files over 2 MiB and
invalid ribbons are rejected before replacing a scene.

```sh
./bin/the-line import --csv examples/centreline.csv --out artifacts/imported.json   --name "Fictional imported sweep"
./bin/the-line render --scene artifacts/imported.json --view 3d   --out artifacts/imported.png
```

**Import centreline CSV** in the Road limits sidebar opens a normal file chooser
in the browser build. In the native editor it reads the displayed scene path
when it ends in `.csv`, otherwise that path plus `.csv`. Import starts a
new road-car study with 30/50 m/s caps and remains one undoable replacement.

The included CSV is fictional. For a real dataset, obtain its licence, retain
source attribution and distinguish surveyed geometry from this application's
illustrative vehicles. Importing a centreline does not calibrate the car model.

Road identities include `track.RoadInterpolantVersion`, which must change when
geometry interpolation changes. Zero-width kerb surface/grip settings do not
stale a study. Studies from the pre-salt C2 release migrate only when their exact
old digest matches their source road; stale manual hypotheses remain stale.
Both native and browser loads apply this compatibility migration.

A kerb taper inherits the material of its present end; absent kerbs cannot lower
friction. Between two present materials, surface changes remain categorical at
the source knot, with conservative adjacent-cell grip checks. Circular clearance
and kerb contact both check neighboring side-edge segments through a shared
spatial index; open entry and exit cross sections remain unwalled.

CSV headers accept a UTF-8 BOM and case-insensitive column names, including
spreadsheet-style `Width_Left`. Geometry failures retain the import context.
Saved JSON omits empty kerb objects.
