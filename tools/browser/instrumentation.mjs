import assert from "node:assert/strict";
import path from "node:path";

// Real mouse events only; the inspection bridge remains read-only.
export async function instrumentationChecks(page, c, check, h) {
  const { state, control, wait, tick, coords, near, artifacts, startPreset } = h;
  const modes = ["speed", "utilization", "lateral_g", "longitudinal_g"];
  await check("fixed line scales and selectable chart channels preserve the trajectory", async () => {
    let before = await state(page);
    if (before.playing) await control(page, "play");
    before = await state(page);
    for (let i = 0; i < 4; i++) {
      await control(page, "line-color");
      await control(page, "chart-channel");
      const s = await state(page);
      assert.equal(s.lineColor, modes[(i + 1) % 4]);
      assert.equal(s.chartChannel, modes[(i + 1) % 4]);
      assert.equal(s.lineDigest, before.lineDigest, "display channel cannot alter the solution");
      assert.equal(s.solveRequests, before.solveRequests, "display channel cannot start a solve");
      near(s.time, before.time, "channel switch preserves inspection time");
      assert.equal(s.current.tyre_forces.available, true);
      assert.ok(s.current.tyre_forces.utilization <= 1.00002);
      await page.screenshot({ path: path.join(artifacts, `${c.name}-forces-${s.lineColor}.png`) });
    }
  });
  await check("force dot and chart cursor agree at entry, apex and exit", async () => {
    await startPreset(page, c, "hairpin");
    let s = await state(page);
    assert.equal(s.scene.name, "The Switchback");
    const apex = s.markers.find(m => m.kind === "apex");
    assert.ok(apex, "corner fixture has a speed apex");
    const first = s.stationBounds[0], last = s.stationBounds[1];
    const positions = [["entry", first], ["apex", apex.station], ["exit", last]];
    await control(page, "chart-channel"); // utilization
    for (const [name, station] of positions) {
      s = await state(page);
      const [x0, y0, x1, y1] = s.controls.chart;
      const x = x0 + (station - first) / (last - first) * (x1 - x0);
      // Endpoint hit testing is half-open; dragging can reach the exact end.
      const start = await coords(page, s, Math.max(x0 + 1, Math.min(x1 - 1, x)), (y0 + y1) / 2);
      const end = await coords(page, s, x, (y0 + y1) / 2);
      await page.mouse.move(start.x, start.y);
      await page.mouse.down();
      await tick(page);
      await page.mouse.move(end.x, end.y, { steps: 2 });
      await tick(page);
      await page.mouse.up();
      await tick(page);
      s = await state(page);
      const sampledStation = first + Math.max(0, Math.min(1, (s.pointer[0] - x0) / (x1 - x0))) * (last - first);
      near(s.current.station, sampledStation, `${name} sampled station`);
      near(s.current.station, station, `${name} intended station`, 2 * (last - first) / (x1 - x0));
      near(s.time, s.current.time, `${name} animation clock`);
      const force = s.current.tyre_forces;
      const reference = s.referenceCurrent?.tyre_forces || s.centerCurrent.tyre_forces;
      const gravity = 9.80665;
      const envelopeScale = Math.max(2 * gravity, Math.ceil(Math.max(force.capacity_mps2, reference.capacity_mps2) / gravity) * gravity);
      const displayScale = Math.min(s.width / 1440, s.height / 900);
      const ox = (s.width - 1440 * displayScale) / 2, oy = (s.height - 900 * displayScale) / 2;
      near(s.forceCursor[0], ox + (410 + force.lateral_mps2 / envelopeScale * 31) * displayScale, `${name} lateral force dot`);
      near(s.forceCursor[1], oy + (839 - force.longitudinal_mps2 / envelopeScale * 31) * displayScale, `${name} longitudinal force dot`);
      near(s.chartCursor[0], ox + (82 + (sampledStation - first) / (last - first) * 996) * displayScale, `${name} chart station`, .01);
      near(s.chartCursor[1], oy + (722 - Math.max(0, Math.min(1, force.utilization)) * 76) * displayScale, `${name} utilization cursor`);
      await page.screenshot({ path: path.join(artifacts, `${c.name}-forces-${name}.png`) });
    }
    // Restore the default channel for subsequent interaction scenarios.
    for (let i = 0; i < 3; i++) await control(page, "chart-channel");
    await control(page, "restart");
  });
}
