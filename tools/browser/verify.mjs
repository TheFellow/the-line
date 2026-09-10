import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import fs from "node:fs/promises";
import { existsSync } from "node:fs";
import http from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";
import { presentationChecks } from "./presentation.mjs";
import { racecraftChecks } from "./racecraft.mjs";
import { closedChecks } from "./closed.mjs";
import { onceChecks } from "./once.mjs";
import { roadmapChecks } from "./roadmap.mjs";
import { authoringChecks } from "./authoring.mjs";
import { manualChecks } from "./manual.mjs";
import { instrumentationChecks } from "./instrumentation.mjs";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const artifactOption = process.argv.find(a => a.startsWith("--artifacts="))?.slice(12);
const artifacts = path.resolve(root, artifactOption || "artifacts/browser");
const selected = process.argv.find((a) => a.startsWith("--case="))?.slice(7);
const scope = process.argv.find((a) => a.startsWith("--scope="))?.slice(8) || "all";
assert.ok(["racecraft", "once", "interaction", "analysis", "setup", "instrumentation", "manual", "authoring", "presentation", "closed", "roadmap", "all"].includes(scope), `Unknown scope: ${scope}`);
const cases = [
  { name: "elevated-retina", view: "3d", width: 1000, height: 700, dpr: 2 },
  { name: "plan", view: "2d", width: 1440, height: 900, dpr: 1 },
].filter((c) => !selected || c.name === selected);
assert.ok(cases.length, `Unknown browser case: ${selected}`);
await fs.mkdir(artifacts, { recursive: true });
if (!process.argv.includes("--skip-build")) {
  execFileSync(
    "go",
    [
      "build",
      "-tags",
      "browsercheck",
      "-o",
      path.join(artifacts, "studio.wasm"),
      "./main/gui",
    ],
    {
      cwd: root,
      env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
      stdio: "inherit",
    },
  );
  const goroot = execFileSync("go", ["env", "GOROOT"], {
    cwd: root,
    encoding: "utf8",
  }).trim();
  await fs.rm(path.join(artifacts, "wasm_exec.js"), { force: true });
  await fs.copyFile(
    path.join(goroot, "lib/wasm/wasm_exec.js"),
    path.join(artifacts, "wasm_exec.js"),
  );
}
const html = `<!doctype html><html><head><meta charset="utf-8"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden;background:#0f151d}</style></head><body><script src="/wasm_exec.js"></script><script>
const go=new Go();const params=new URLSearchParams(location.search);go.argv=['studio','--preset',params.get('preset')||'banked','--view',params.get('view')||'2d'];
for (const flag of ['mode','race-file','vehicle']) { if(params.has(flag)) go.argv.push('--'+flag,params.get(flag)); }
go.exit=(code)=>{window.applicationExit=code};
WebAssembly.instantiateStreaming(fetch('/studio.wasm'),go.importObject).then(r=>go.run(r.instance)).catch(e=>{window.bootError=String(e)});
</script></body></html>`;
const server = http.createServer(async (req, res) => {
  try {
    const pathname = new URL(req.url, "http://localhost").pathname;
    if (pathname === "/favicon.ico") {
      res.writeHead(204).end();
      return;
    }
    if (pathname === "/") {
      res.setHeader("Content-Type", "text/html");
      res.end(html);
      return;
    }
    if (!["/wasm_exec.js", "/studio.wasm"].includes(pathname)) {
      res.writeHead(404).end();
      return;
    }
    res.setHeader(
      "Content-Type",
      pathname.endsWith(".wasm") ? "application/wasm" : "text/javascript",
    );
    res.end(await fs.readFile(path.join(artifacts, pathname.slice(1))));
  } catch (e) {
    res.writeHead(500).end(String(e));
  }
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const chrome =
  process.env.LINE_CHROME_PATH ||
  (process.platform === "darwin"
    ? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    : undefined);
let browser;
try {
  browser = await chromium.launch({
    headless: true,
    ...(chrome && existsSync(chrome) ? { executablePath: chrome } : {}),
    args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader"],
  });
} catch (error) {
  await new Promise((resolve) => server.close(resolve));
  throw error;
}
const base = `http://127.0.0.1:${server.address().port}`;
const reports = [];

async function startPreset(page, c, preset = "banked") {
  await page.goto(`${base}/?view=${c.view}&preset=${preset}`);
  await wait(page, s => s.updates > 5 && !s.busy, "fixture startup");
  if ((await state(page)).playing) await control(page, "play");
  await wait(page, s => !s.playing, "pause fixture");
}
async function state(page) {
  return page.evaluate(() => JSON.parse(window.theLineSnapshot()));
}
async function wait(page, predicate, label) {
  const source = predicate.toString();
  await page
    .waitForFunction(
      (source) => {
        if (!window.theLineSnapshot) return false;
        const text = window.theLineSnapshot();
        return text && eval("(" + source + ")")(JSON.parse(text));
      },
      source,
      { timeout: 120000, polling: 50 },
    )
    .catch(async (e) => {
      const snapshot = await state(page);
      const summary = Object.fromEntries(["view", "status", "busy", "generation", "pointer", "dragging", "manualDragging", "cameraDragging", "updates"].map(key => [key, snapshot[key]]));
      throw new Error(`${label}: ${e.message}\n${JSON.stringify(summary)}`);
    });
  return state(page);
}
async function tick(page) {
  const s = await state(page);
  await page.waitForFunction(
    (n) => JSON.parse(window.theLineSnapshot()).updates > n + 2,
    s.updates,
    { timeout: 120000, polling: 30 },
  );
}
async function key(page, name) {
  await page.keyboard.down(name);
  await tick(page);
  await page.keyboard.up(name);
  await tick(page);
}
async function coords(page, s, x, y) {
  const rect = await page.locator("canvas").boundingBox();
  const scale = Math.min(rect.width / s.width, rect.height / s.height);
  return {
    x: rect.x + (rect.width - s.width * scale) / 2 + x * scale,
    y: rect.y + (rect.height - s.height * scale) / 2 + y * scale,
  };
}
async function click(page, x, y) {
  await page.mouse.move(x, y);
  await page.mouse.down();
  await tick(page);
  await page.mouse.up();
  await tick(page);
}
async function control(page, name) {
  const s = await state(page);
  const r = s.controls[name];
  assert.ok(r, `Control ${name} is unavailable in ${s.view} view`);
  const p = await coords(page, s, (r[0] + r[2]) / 2, (r[1] + r[3]) / 2);
  await click(page, p.x, p.y);
}
function near(a, b, label, tolerance = 1e-6) {
  assert.ok(Math.abs(a - b) < tolerance, `${label}: ${a} != ${b}`);
}
async function begin(page, index, dx = 4, dy = 3) {
  const s = await state(page);
  const p = s.points[index];
  assert.ok(p[0] + 6 >= 0 && p[0] + 6 < s.width && p[1] - 4 >= 0 && p[1] - 4 < s.height, `handle ${index} is outside the visible window: ${JSON.stringify({point:p, camera:s.camera})}`);
  const start = await coords(page, s, p[0] + 6, p[1] - 4);
  const end = await coords(
    page,
    s,
    p[0] +
      6 +
      dx * (s.axisX[0] - s.origin[0]) +
      dy * (s.axisY[0] - s.origin[0]),
    p[1] -
      4 +
      dx * (s.axisX[1] - s.origin[1]) +
      dy * (s.axisY[1] - s.origin[1]),
  );
  await page.mouse.move(start.x, start.y);
  await page.mouse.down();
  const grabbed = await wait(page, (s) => s.dragging, "pointer press");
  await page.mouse.move(end.x, end.y, { steps: 8 });
  await tick(page);
  return { before: s, end, index, dx, dy, grabbed };
}
async function unchanged(page, before) {
  await tick(page);
  const s = await state(page);
  assert.deepEqual(s.scene, before.scene);
  assert.equal(s.lineDigest, before.lineDigest);
  assert.equal(s.busy, false);
  return s;
}

function cameraOnly(before, after) {
  assert.deepEqual(after.scene, before.scene, "camera must preserve geometry");
  assert.equal(after.lineDigest, before.lineDigest);
  assert.equal(after.solveRequests, before.solveRequests, "camera must not queue a solve");
  assert.equal(after.canUndo, before.canUndo);
  assert.equal(after.canRedo, before.canRedo);
  assert.equal(after.selected, before.selected, "camera must not select a handle");
  assert.equal(after.dragging, false);
  assert.equal(after.busy, false);
}
async function cameraGesture(page, { button = "right", modifier, dx = 100, dy = 0 } = {}) {
  const before = await state(page);
  const started = Date.now();
  const r = before.roadViewport;
  const scale = Math.min(before.width / 1440, before.height / 900);
  const x = r[0] + (r[2] - r[0]) * .3;
  const y = r[1] + (r[3] - r[1]) * .45;
  const start = await coords(page, before, x, y);
  const end = await coords(page, before, x + dx * scale, y + dy * scale);
  if (modifier) await page.keyboard.down(modifier);
  await page.mouse.move(start.x, start.y);
  await page.mouse.down({ button });
  await wait(page, (s) => s.cameraDragging, "camera captures press");
  await page.mouse.move(end.x, end.y, { steps: 8 });
  await tick(page);
  const held = await state(page);
  assert.equal(held.dragging, false);
  return { before, held, button, modifier, end, gestureMillis: Date.now() - started, updates: held.updates - before.updates };
}
async function releaseCamera(page, gesture) {
  await page.mouse.up({ button: gesture.button });
  if (gesture.modifier) await page.keyboard.up(gesture.modifier);
  const after = await wait(page, (s) => !s.cameraDragging, "release camera");
  cameraOnly(gesture.before, after);
  return after;
}
function expectedDrop(gesture, held) {
  const a = gesture.before.axisX.map((v, i) => v - gesture.before.origin[i]);
  const b = gesture.before.axisY.map((v, i) => v - gesture.before.origin[i]);
  const delta = held.pointer.map((v, i) => v - gesture.grabbed.pointer[i]);
  const det = a[0] * b[1] - a[1] * b[0];
  const p = gesture.before.scene.points[gesture.index];
  return {
    x: p.x + (delta[0] * b[1] - delta[1] * b[0]) / det,
    y: p.y + (a[0] * delta[1] - a[1] * delta[0]) / det,
    z: p.z,
  };
}
async function seekTimeline(page, fraction) {
  const s = await state(page), r = s.controls.scrub;
  const x = r[0] + fraction * (r[2] - r[0]);
  const p = await coords(page, s, x, (r[1] + r[3]) / 2);
  if (fraction >= 1) {
    // Hit rectangles exclude their right edge. Capture inside, then drag to
    // the exact endpoint just as a user does; DPR rounding must not decide it.
    const start = await coords(page, s, r[2] - 2, (r[1] + r[3]) / 2);
    await page.mouse.move(start.x, start.y);
    await page.mouse.down(); await tick(page);
    await page.mouse.move(p.x, p.y); await tick(page);
    await page.mouse.up(); await tick(page);
  } else {
    await click(page, p.x, p.y);
  }
  return state(page);
}
function comparisonConsistent(s) {
  near(s.current.time, Math.min(s.time, s.duration), "optimized elapsed time");
  near(s.centerCurrent.time, Math.min(s.time, s.centerDuration), "reference elapsed time");
  near(s.sameStationCenter.station, s.current.station, "delta shared station");
  near(s.delta, s.current.time - s.sameStationCenter.time, "signed station delta");
}
async function analysisChecks(page, c, check) {
  const measurements = { cameraGestures: [] };
  await control(page, "fit");
  const initial = await state(page);
  await check("camera orbit, pan, zoom and fit preserve the solved scene", async () => {
    if (initial.view === "3d") {
      let cumulativeYaw = 0;
      for (let i = 0; i < 4; i++) {
        const gesture = await cameraGesture(page, { dx: 210 });
        const moved = await releaseCamera(page, gesture);
        measurements.cameraGestures.push({ kind: "orbit", milliseconds: gesture.gestureMillis, updates: gesture.updates });
        const delta = moved.camera.yaw - gesture.before.camera.yaw;
        cumulativeYaw += Math.atan2(Math.sin(delta), Math.cos(delta));
        near(moved.camera.zoom, gesture.before.camera.zoom, "orbit must not refit zoom");
        if (i === 1) await page.screenshot({ path: path.join(artifacts, `${c.name}-opposite-side.png`) });
      }
      assert.ok(cumulativeYaw > 2 * Math.PI, "right drag must complete a full yaw revolution");
      const alt = await cameraGesture(page, { button: "left", modifier: "Alt", dx: -90, dy: 25 });
      const moved = await releaseCamera(page, alt);
      assert.notEqual(moved.camera.yaw, alt.before.camera.yaw);
      assert.notEqual(moved.camera.elevation, alt.before.camera.elevation);
    }
    for (const input of [{ button: "middle" }, { button: "left", modifier: "Shift" }]) {
      const gesture = await cameraGesture(page, { ...input, dx: 25, dy: -15 });
      const moved = await releaseCamera(page, gesture);
      measurements.cameraGestures.push({ kind: "pan", milliseconds: gesture.gestureMillis, updates: gesture.updates });
      assert.ok(moved.camera.panX > gesture.before.camera.panX);
      assert.ok(moved.camera.panY < gesture.before.camera.panY);
      near(moved.camera.yaw, gesture.before.camera.yaw, "pan preserves yaw");
    }
    const beforeZoom = await state(page), v = beforeZoom.roadViewport;
    const p = await coords(page, beforeZoom, (v[0] + v[2]) / 2, (v[1] + v[3]) / 2);
    await page.mouse.move(p.x, p.y);
    await page.mouse.wheel(0, -160);
    await tick(page);
    const zoomed = await state(page);
    cameraOnly(beforeZoom, zoomed);
    assert.ok(zoomed.camera.zoom > beforeZoom.camera.zoom, "wheel up zooms in");
    assert.ok(zoomed.camera.zoom < 2 * beforeZoom.camera.zoom, "one wheel gesture must not make an abrupt multi-fold zoom jump");
    await page.screenshot({ path: path.join(artifacts, `${c.name}-camera.png`) });
    await control(page, "fit");
    const reset = await state(page);
    cameraOnly(initial, reset);
    assert.deepEqual(reset.camera, initial.camera, "Fit restores default framing");
  });

  await check("camera cancellation and fresh transformed handle drag", async () => {
    for (const cancel of ["Escape", "blur", "Tab", "leave"]) {
      console.log(`${c.name}: cancel camera (${cancel})`);
      const gesture = await cameraGesture(page, { button: "middle", dx: 18, dy: 8 });
      const canvas = page.locator("canvas");
      const style = await canvas.getAttribute("style");
      if (cancel === "blur") await canvas.evaluate((el) => el.blur());
      else if (cancel === "leave") {
        await canvas.evaluate((el) => {
          el.style.setProperty("width", "85vw", "important");
          el.style.setProperty("height", "85vh", "important");
        });
        const bounds = await canvas.boundingBox();
        await page.mouse.move(Math.min(c.width - 2, bounds.x + bounds.width + 20), bounds.y + bounds.height / 2);
      } else await key(page, cancel);
      const cancelled = await wait(page, (s) => !s.cameraDragging, `${cancel} cancels camera`);
      if (cancel !== "leave") {
        await page.mouse.move(gesture.end.x + 10, gesture.end.y + 5);
        await tick(page);
        assert.deepEqual((await state(page)).camera, cancelled.camera, "held movement cannot resume cancelled camera");
      }
      await page.mouse.up({ button: gesture.button });
      if (cancel === "leave") {
        await canvas.evaluate((el, previous) => {
          if (previous === null) el.removeAttribute("style"); else el.setAttribute("style", previous);
        }, style);
      }
      if (cancel === "Tab") await key(page, "Tab");
      await tick(page);
      cameraOnly(gesture.before, await state(page));
    }
    await control(page, "fit");
    if (initial.view === "3d") await releaseCamera(page, await cameraGesture(page, { dx: 65, dy: 12 }));
    await releaseCamera(page, await cameraGesture(page, { button: "middle", dx: 12, dy: 5 }));
    const beforeZoom = await state(page), v = beforeZoom.roadViewport;
    const zoomAt = await coords(page, beforeZoom, (v[0] + v[2]) / 2, (v[1] + v[3]) / 2);
    await page.mouse.move(zoomAt.x, zoomAt.y);
    await page.mouse.wheel(0, -70);
    await tick(page);
    const gesture = await begin(page, 3, 2, 1);
    const held = await state(page), expected = expectedDrop(gesture, held);
    near(held.preview.X, expected.x, "transformed preview x");
    near(held.preview.Y, expected.y, "transformed preview y");
    near(held.preview.Z, expected.z, "transformed preview elevation");
    await page.mouse.up();
    const after = await wait(page, (s) => !s.dragging && !s.busy && s.canUndo, "transformed edit solves");
    near(after.scene.points[3].x, expected.x, "transformed committed x");
    near(after.scene.points[3].y, expected.y, "transformed committed y");
    near(after.scene.points[3].z, expected.z, "transformed committed elevation");
    assert.deepEqual(after.camera, gesture.before.camera, "solve preserves inspection camera");
    assert.equal(after.solveRequests, gesture.before.solveRequests + 1, "one solve per committed drag");
    assert.notEqual(after.lineDigest, gesture.before.lineDigest);
    await page.screenshot({ path: path.join(artifacts, `${c.name}-transformed-edit.png`) });
    await control(page, "undo");
    await wait(page, (s) => !s.busy && !s.canUndo && s.canRedo, "one undo restores transformed edit");
    await unchanged(page, gesture.before);
    await control(page, "fit");
  });

  await check("preset and vehicle changes replace both verified trajectories", async () => {
    for (const name of ["Ridge to River", "Rhythm Section"]) {
      if (name === "Rhythm Section") await startPreset(page, c, "hairpin");
      const before = await state(page);
      await control(page, "preset");
      const next = await wait(page, (s) => !s.busy && s.scene.name === s.solvedScene.name, "preset comparison solves");
      assert.equal(next.scene.name, name);
      assert.deepEqual(next.scene, next.solvedScene);
      assert.notEqual(next.lineDigest, before.lineDigest);
      assert.equal(next.solveRequests, before.solveRequests + 1);
      near(next.endNode.station, next.centerEndNode.station, "new reference coordinate bounds");
      near(next.endNode.time, next.duration, "new optimized duration");
      near(next.centerEndNode.time, next.centerDuration, "new reference duration");
      assert.ok(next.centerDuration >= next.duration);
      comparisonConsistent(next);
    }
    const beforeVehicle = await state(page);
    await control(page, "vehicle");
    const changed = await wait(page, (s) => !s.busy && s.scene.vehicle === "gt", "vehicle comparison solves");
    assert.deepEqual(changed.scene, changed.solvedScene);
    assert.equal(changed.solveRequests, beforeVehicle.solveRequests + 1);
    assert.notEqual(changed.lineDigest, beforeVehicle.lineDigest);
    assert.notEqual(changed.centerDuration, beforeVehicle.centerDuration, "vehicle refreshes reference speed profile");
    assert.deepEqual(changed.camera, beforeVehicle.camera, "vehicle switch preserves camera");
    comparisonConsistent(changed);
    await page.screenshot({ path: path.join(artifacts, `${c.name}-gt-comparison.png`) });
    await control(page, "undo");
    await wait(page, (s) => !s.busy && s.scene.vehicle === "road", "restore road vehicle");
    const restored = await unchanged(page, beforeVehicle);
    near(restored.centerDuration, beforeVehicle.centerDuration, "undo restores reference profile");
  });

  await check("chart seeks shared road station and retains gesture ownership", async () => {
    const before = await state(page), r = before.controls.chart;
    const first = before.stationBounds[0], last = before.stationBounds[1];
    const y = (r[1] + r[3]) / 2;
    for (const fraction of [0, .5, 1]) {
      const p = await coords(page, before, r[0] + 2 + fraction * (r[2] - r[0] - 4), y);
      await click(page, p.x, p.y);
      const sought = await state(page);
      const f = Math.max(0, Math.min(1, (sought.pointer[0] - r[0]) / (r[2] - r[0])));
      near(sought.current.station, first + f * (last - first), "chart station");
      assert.equal(sought.playing, false);
      comparisonConsistent(sought);
      cameraOnly(before, sought);
    }
    const middle = await coords(page, before, (r[0] + r[2]) / 2, y);
    const beforeUnowned = await state(page), blank = await coords(page, before, 20, 20);
    await page.mouse.move(blank.x, blank.y);
    await page.mouse.down();
    await tick(page);
    await page.mouse.move(middle.x, middle.y);
    await tick(page);
    near((await state(page)).time, beforeUnowned.time, "unowned held pointer cannot seek chart");
    assert.equal((await state(page)).charting, false);
    await page.mouse.up();
    await tick(page);
    await page.mouse.down();
    await wait(page, (s) => s.charting, "chart press captures");
    const handle = await coords(page, before, ...before.points[3]);
    await page.mouse.move(handle.x, handle.y, { steps: 8 });
    await tick(page);
    const overHandle = await state(page);
    assert.equal(overHandle.charting, true, "chart owns gesture over a geometry handle");
    cameraOnly(before, overHandle);
    for (const fraction of [.25, -.05, 1.05]) {
      const p = await coords(page, before, r[0] + fraction * (r[2] - r[0]), r[1] - 40);
      await page.mouse.move(p.x, p.y, { steps: 8 });
      await tick(page);
      const sought = await state(page);
      const f = Math.max(0, Math.min(1, (sought.pointer[0] - r[0]) / (r[2] - r[0])));
      near(sought.current.station, first + f * (last - first), "captured chart station");
      assert.equal(sought.dragging, false);
      assert.equal(sought.cameraDragging, false);
      assert.equal(sought.playing, false);
      comparisonConsistent(sought);
      cameraOnly(before, sought);
    }
    const atEnd = await state(page);
    near(atEnd.delta, atEnd.duration - atEnd.centerDuration, "terminal signed delta");
    await page.mouse.up();
    await wait(page, (s) => !s.charting, "chart releases capture");
    await page.mouse.move(middle.x, middle.y);
    await tick(page);
    near((await state(page)).time, atEnd.time, "hover does not seek after chart release");
    const beforeRight = await state(page);
    await page.mouse.down({ button: "right" });
    await tick(page);
    await page.mouse.move(middle.x + 30, middle.y);
    await tick(page);
    assert.equal((await state(page)).charting, false);
    assert.equal((await state(page)).cameraDragging, false);
    await page.mouse.up({ button: "right" });
    await tick(page);
    near((await state(page)).time, beforeRight.time, "right drag cannot scrub chart");
    await page.screenshot({ path: path.join(artifacts, `${c.name}-chart-end.png`) });
  });

  await check("reference ghost uses shared time and both finish before restart", async () => {
    const before = await state(page);
    assert.equal(before.comparison, true);
    near(before.playbackDuration, Math.max(before.duration, before.centerDuration), "comparison duration");
    assert.ok(before.centerDuration > before.duration, "fixture needs a slower centreline reference");
    const fraction = ((before.duration + before.centerDuration) / 2) / before.playbackDuration;
    const heldFinish = await seekTimeline(page, fraction);
    comparisonConsistent(heldFinish);
    assert.ok(heldFinish.time > heldFinish.duration && heldFinish.time < heldFinish.centerDuration);
    assert.deepEqual(heldFinish.current, heldFinish.endNode, "optimized car waits exactly at exit");
    assert.ok(heldFinish.centerCurrent.station < heldFinish.centerEndNode.station, "reference still travels its own centreline");
    near(heldFinish.centerCurrent.offset, 0, "reference carries no optimized offset");
    await page.screenshot({ path: path.join(artifacts, `${c.name}-ghost-finish.png`) });
    await control(page, "comparison");
    const disabled = await state(page);
    assert.equal(disabled.comparison, false);
    near(disabled.playbackDuration, disabled.duration, "comparison disabled duration");
    near(disabled.time, disabled.duration, "comparison disable clamps clock");
    await control(page, "comparison");
    const enabled = await state(page);
    assert.equal(enabled.comparison, true);
    near(enabled.playbackDuration, enabled.centerDuration, "comparison reenabled duration");
    cameraOnly(before, enabled);
    await seekTimeline(page, .995);
    await control(page, "play");
    const restarted = await wait(page, (s) => s.playing && s.time < .6, "open sequence restarts after reference finish");
    comparisonConsistent(restarted);
    await control(page, "play");
    await wait(page, (s) => !s.playing, "pause after comparison loop");
  });

  await check("playback rates advance simulated time per update", async () => {
    for (const rate of [1, 2, .25, .5]) {
      if ((await state(page)).rate !== rate) await control(page, "rate");
      near((await state(page)).rate, rate, "selected playback rate");
      await control(page, "restart");
      await control(page, "play");
      await wait(page, (s) => s.playing, "rate playback starts");
      const a = await state(page);
      await page.waitForFunction((updates) => JSON.parse(window.theLineSnapshot()).updates >= updates + 24, a.updates, { timeout: 120000, polling: 30 });
      const b = await state(page);
      near(b.time - a.time, (b.updates - a.updates) * rate / 60, "rate scales simulation updates", 1e-8);
      comparisonConsistent(b);
      await control(page, "play");
      await wait(page, (s) => !s.playing, "rate playback pauses");
    }
    await control(page, "rate");
    near((await state(page)).rate, 1, "rate cycle returns to realtime");
    await seekTimeline(page, .5);
    await page.screenshot({ path: path.join(artifacts, `${c.name}-comparison.png`) });
  });
  return measurements;
}

try {
  for (const c of cases) {
    const context = await browser.newContext({
      viewport: { width: c.width, height: c.height },
      deviceScaleFactor: c.dpr,
    });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    page.on("console", (msg) => {
      if (msg.type() === "error") errors.push(msg.text());
    });
    const checks = [];
    let measurements;
    let currentCheck = "startup";
    const started = Date.now();
    try {
      await page.goto(`${base}/?view=${c.view}`);
      await wait(page, (s) => s.updates > 5 && !s.busy, "startup");
      await control(page, "play");
      await wait(page, (s) => !s.playing, "pause animation");
      const original = await state(page);
      if (scope === "interaction" || scope === "all") {
        currentCheck = "click without movement";
        console.log(`${c.name}: ${currentCheck}`);
        const p = await coords(page, original, ...original.points[3]);
        await click(page, p.x, p.y);
        await wait(page, (s) => s.selected === 3 && !s.dragging, "select handle");
        await unchanged(page, original);
        assert.equal((await state(page)).canUndo, false);
        checks.push(currentCheck);
        currentCheck = "drag preview and release";
        console.log(`${c.name}: ${currentCheck}`);
        const gesture = await begin(page, 3);
        let held = await state(page);
        assert.deepEqual(held.scene, gesture.before.scene);
        const a = gesture.before.axisX.map(
            (v, i) => v - gesture.before.origin[i],
          ),
          b = gesture.before.axisY.map((v, i) => v - gesture.before.origin[i]);
        const delta = held.pointer.map((v, i) => v - gesture.grabbed.pointer[i]);
        const det = a[0] * b[1] - a[1] * b[0];
        const expectedX =
            original.scene.points[3].x +
            (delta[0] * b[1] - delta[1] * b[0]) / det,
          expectedY =
            original.scene.points[3].y +
            (a[0] * delta[1] - a[1] * delta[0]) / det;
        near(held.preview.X, expectedX, "preview x");
        near(held.preview.Y, expectedY, "preview y");
        await page.screenshot({
          path: path.join(artifacts, `${c.name}-held.png`),
        });
        await page.mouse.up();
        let after = await wait(
          page,
          (s) => !s.busy && !s.dragging && s.canUndo,
          "solve dropped geometry",
        );
        near(after.scene.points[3].x, expectedX, "released x");
        near(after.scene.points[3].y, expectedY, "released y");
        near(
          after.scene.points[3].z,
          original.scene.points[3].z,
          "fixed elevation",
        );
        assert.deepEqual(after.solvedScene, after.scene);
        assert.notEqual(after.lineDigest, original.lineDigest);
        assert.ok(after.duration > 0 && Number.isFinite(after.duration));
        assert.ok(after.forceResidual < 0.0005);
        await page.screenshot({
          path: path.join(artifacts, `${c.name}-released.png`),
        });
        checks.push(currentCheck);
        currentCheck = "undo and redo";
        console.log(`${c.name}: ${currentCheck}`);
        await control(page, "undo");
        await wait(page, (s) => !s.busy && s.canRedo && !s.canUndo, "undo solve");
        await unchanged(page, original);
        await control(page, "redo");
        await wait(page, (s) => !s.busy && s.canUndo, "redo solve");
        assert.deepEqual((await state(page)).scene, after.scene);
        assert.equal((await state(page)).lineDigest, after.lineDigest);
        await control(page, "undo");
        await wait(
          page,
          (s) => !s.busy && s.canRedo && !s.canUndo,
          "restore initial scene",
        );
        checks.push(currentCheck);
        currentCheck = "Escape cancellation";
        console.log(`${c.name}: ${currentCheck}`);
        const cancelled = await begin(page, 3);
        await key(page, "Escape");
        await wait(page, (s) => !s.dragging, "Escape clears drag");
        await page.mouse.up();
        await unchanged(page, cancelled.before);
        checks.push(currentCheck);
        currentCheck = "focus-loss cancellation";
        console.log(`${c.name}: ${currentCheck}`);
        const blurred = await begin(page, 3);
        await page.locator("canvas").evaluate((canvas) => canvas.blur());
        await tick(page);
        await page.mouse.up();
        await wait(page, (s) => !s.busy && !s.dragging, "blur clears drag");
        await unchanged(page, blurred.before);
        checks.push(currentCheck);
        currentCheck = "Tab cancellation";
        console.log(`${c.name}: ${currentCheck}`);
        const switched = await begin(page, 3);
        await key(page, "Tab");
        await wait(page, (s) => !s.dragging, "Tab clears drag");
        await page.mouse.up();
        const switchedState = await unchanged(page, switched.before);
        assert.notEqual(switchedState.view, switched.before.view);
        await key(page, "Tab");
        await tick(page);
        assert.equal((await state(page)).view, switched.before.view);
        checks.push(currentCheck);

        currentCheck = "canvas leave and outside release cancellation";
        console.log(`${c.name}: ${currentCheck}`);
        const left = await begin(page, 3);
        // Keep the pointer on the canvas while shrinking its CSS bounds, then send
        // an actual mouse movement/release into the exposed page margin.
        const canvas = page.locator("canvas");
        const oldStyle = await canvas.getAttribute("style");
        try {
          await canvas.evaluate((el) => {
            el.style.setProperty("width", "85vw", "important");
            el.style.setProperty("height", "85vh", "important");
          });
          const bounds = await canvas.boundingBox();
          const outside = {
            x: Math.min(c.width - 2, bounds.x + bounds.width + 20),
            y: Math.min(c.height - 2, bounds.y + bounds.height / 2),
          };
          assert.ok(
            outside.x > bounds.x + bounds.width,
            "outside release must leave canvas",
          );
          await page.mouse.move(outside.x, outside.y, { steps: 8 });
          await tick(page);
          await page.mouse.up();
          await wait(
            page,
            (s) => !s.dragging && !s.busy,
            "leaving canvas cancels drag",
          );
          await unchanged(page, left.before);
        } finally {
          await canvas.evaluate((el, style) => {
            if (style === null) el.removeAttribute("style");
            else el.setAttribute("style", style);
          }, oldStyle);
          await tick(page);
        }
        const returnToHandle = await coords(
          page,
          left.before,
          ...left.before.points[3],
        );
        await page.mouse.move(returnToHandle.x, returnToHandle.y);
        await tick(page);
        assert.equal(
          (await state(page)).dragging,
          false,
          "return after outside release must not resume drag",
        );
        checks.push(currentCheck);

        currentCheck = "invalid drop preserves geometry and redo branch";
        console.log(`${c.name}: ${currentCheck}`);
        const beforeInvalid = await state(page);
        assert.equal(
          beforeInvalid.canRedo,
          true,
          "scenario needs existing redo branch",
        );
        const target = beforeInvalid.scene.points[4],
          moving = beforeInvalid.scene.points[3];
        const invalid = await begin(
          page,
          3,
          target.x - moving.x,
          target.y - moving.y,
        );
        const invalidHeld = await state(page);
        assert.ok(
          Math.hypot(
            invalidHeld.preview.X - target.x,
            invalidHeld.preview.Y - target.y,
          ) < 1,
          "quantized drop must violate minimum control separation",
        );
        await page.mouse.up();
        await wait(
          page,
          (s) =>
            !s.dragging && !s.busy && s.status.includes("at least 1 metre apart"),
          "invalid drop rejection",
        );
        const rejected = await unchanged(page, invalid.before);
        assert.equal(rejected.canUndo, invalid.before.canUndo);
        assert.equal(rejected.canRedo, true);
        await control(page, "redo");
        await wait(
          page,
          (s) => !s.busy && s.canUndo && !s.canRedo,
          "redo survives invalid drop",
        );
        assert.deepEqual((await state(page)).scene, after.scene);
        assert.equal((await state(page)).lineDigest, after.lineDigest);
        await control(page, "undo");
        await wait(
          page,
          (s) => !s.busy && !s.canUndo && s.canRedo,
          "restore after invalid drop",
        );
        checks.push(currentCheck);

        currentCheck = "solver rejection restores original redo branch";
        console.log(`${c.name}: ${currentCheck}`);
        const beforeSolverRejection = await state(page);
        assert.equal(beforeSolverRejection.selected, 3);
        assert.equal(beforeSolverRejection.canRedo, true);
        // Asphalt wraps directly to ice. On this banked control, ice cannot support
        // even a stationary vehicle, so geometry validates but the solver rejects.
        await control(page, "surface-");
        await wait(
          page,
          (s) => !s.busy && s.status.startsWith("edit reverted:"),
          "ice edit rejected by solver",
        );
        const solverRejected = await unchanged(page, beforeSolverRejection);
        assert.equal(solverRejected.selected, beforeSolverRejection.selected);
        assert.equal(solverRejected.canUndo, beforeSolverRejection.canUndo);
        assert.equal(solverRejected.canRedo, true);
        await control(page, "redo");
        await wait(
          page,
          (s) => !s.busy && s.canUndo && !s.canRedo,
          "redo survives solver rejection",
        );
        assert.deepEqual((await state(page)).scene, after.scene);
        assert.equal((await state(page)).lineDigest, after.lineDigest);
        await control(page, "undo");
        await wait(
          page,
          (s) => !s.busy && !s.canUndo && s.canRedo,
          "restore after solver rejection",
        );
        checks.push(currentCheck);

        currentCheck = "scrub requires a press on the timeline";
        console.log(`${c.name}: ${currentCheck}`);
        const beforeScrub = await state(page),
          scrub = beforeScrub.controls.scrub;
        const blank = await coords(page, beforeScrub, 20, 20),
          middle = await coords(
            page,
            beforeScrub,
            (scrub[0] + scrub[2]) / 2,
            (scrub[1] + scrub[3]) / 2,
          );
        await page.mouse.move(blank.x, blank.y);
        await page.mouse.down();
        await tick(page);
        await page.mouse.move(middle.x, middle.y, { steps: 8 });
        await tick(page);
        near(
          (await state(page)).time,
          beforeScrub.time,
          "held pointer from outside timeline must not seek",
        );
        await page.mouse.up();
        await tick(page);
        checks.push(currentCheck);

        currentCheck = "scrub retains capture outside strip and clamps endpoints";
        console.log(`${c.name}: ${currentCheck}`);
        await page.mouse.down();
        await tick(page);
        let scrubbed = await state(page);
        near(
          scrubbed.time / scrubbed.playbackDuration,
          (scrubbed.pointer[0] - scrub[0]) / (scrub[2] - scrub[0]),
          "timeline press seeks",
        );
        assert.equal(scrubbed.playing, false);
        const quarter = await coords(
          page,
          beforeScrub,
          scrub[0] + (scrub[2] - scrub[0]) / 4,
          scrub[1] - 40,
        );
        await page.mouse.move(quarter.x, quarter.y, { steps: 8 });
        await tick(page);
        scrubbed = await state(page);
        near(
          scrubbed.time / scrubbed.playbackDuration,
          (scrubbed.pointer[0] - scrub[0]) / (scrub[2] - scrub[0]),
          "scrub continues outside vertical strip",
        );
        const startClamp = await coords(
          page,
          beforeScrub,
          Math.max(1, scrub[0] - 20),
          scrub[1] - 40,
        );
        await page.mouse.move(startClamp.x, startClamp.y, { steps: 8 });
        await tick(page);
        near((await state(page)).time, 0, "scrub clamps start");
        const endClamp = await coords(
          page,
          beforeScrub,
          scrub[2] + 20,
          scrub[1] - 40,
        );
        await page.mouse.move(endClamp.x, endClamp.y, { steps: 8 });
        await tick(page);
        scrubbed = await state(page);
        near(scrubbed.time, scrubbed.playbackDuration, "scrub clamps end");
        await page.mouse.up();
        await tick(page);
        await page.mouse.move(middle.x, middle.y);
        await tick(page);
        near(
          (await state(page)).time,
          scrubbed.time,
          "release ends scrub capture",
        );
        checks.push(currentCheck);
        currentCheck = "rendered playback advances without changing geometry";
        console.log(`${c.name}: ${currentCheck}`);
        await control(page, "restart");
        const playbackBefore = await state(page);
        await control(page, "play");
        await wait(page, (s) => s.playing, "start playback");
        await page.screenshot({
          path: path.join(artifacts, `${c.name}-animation-a.png`),
        });
        const animationStart = await state(page);
        await page.waitForFunction(
          (n) => JSON.parse(window.theLineSnapshot()).updates > n + 30,
          animationStart.updates,
          { timeout: 120000, polling: 50 },
        );
        await page.screenshot({
          path: path.join(artifacts, `${c.name}-animation-b.png`),
        });
        const animationEnd = await state(page);
        assert.ok(
          animationEnd.time > animationStart.time,
          "playback must advance between rendered frames",
        );
        await control(page, "play");
        await wait(page, (s) => !s.playing, "pause playback");
        await unchanged(page, playbackBefore);
        checks.push(currentCheck);
      }
      if (scope === "analysis" || scope === "all") {
        measurements = await analysisChecks(page, c, async (name, run) => {
          currentCheck = name;
          console.log(`${c.name}: ${name}`);
          await run();
          checks.push(name);
        });
      }
      if (scope === "setup" || scope === "roadmap" || scope === "all") {
        await startPreset(page, c);
        await roadmapChecks(page, c, async (name, run) => {
          currentCheck = name;
          console.log(`${c.name}: ${name}`);
          await run();
          checks.push(name);
        }, { state, wait, control, tick, near, coords, click, key, seekTimeline, artifacts, startPreset });
      }
      if (scope === "once" || scope === "roadmap" || scope === "all") {
        await onceChecks(page, c, async (name, run) => {
          currentCheck = name;
          console.log(`${c.name}: ${name}`);
          await run();
          checks.push(name);
        }, { state, wait, control, tick, near, coords, artifacts, startPreset });
      }
      if (scope === "instrumentation" || scope === "roadmap" || scope === "all") {
        await startPreset(page, c);
        await instrumentationChecks(page, c, async (name, run) => {
          currentCheck = name;
          console.log(`${c.name}: ${name}`);
          await run();
          checks.push(name);
        }, { state, wait, control, tick, near, coords, click, key, seekTimeline, artifacts, startPreset });
      }
      if (scope === "manual" || scope === "roadmap" || scope === "all") {
        await startPreset(page, c);
        await manualChecks(page, c, async (name, run) => {
          currentCheck = name;
          console.log(`${c.name}: ${name}`);
          await run();
          checks.push(name);
        }, { state, wait, control, tick, near, coords, click, key, seekTimeline, artifacts, startPreset });
      }
      if (scope === "authoring" || scope === "roadmap" || scope === "all") {
        await startPreset(page, c);
        await authoringChecks(page, c, async (name, run) => {
          currentCheck = name;
          console.log(`${c.name}: ${name}`);
          await run();
          checks.push(name);
        }, { state, wait, control, tick, near, coords, click, key, seekTimeline, artifacts, startPreset });
      }
      if (scope === "presentation" || scope === "roadmap" || scope === "all") {
        await startPreset(page, c, "banked");
        await presentationChecks(page, c, async (name, run) => {
          currentCheck = name;
          console.log(`${c.name}: ${name}`);
          await run(); checks.push(name);
        }, { state, wait, control, tick, near, coords, click, key, seekTimeline, artifacts, startPreset });
      }
      if (scope === "closed" || scope === "roadmap" || scope === "all") {
        await startPreset(page, c, "club-loop");
        await closedChecks(page, c, async (name, run) => {
          currentCheck = name;
          console.log(`${c.name}: ${name}`);
          await run(); checks.push(name);
        }, { state, wait, control, tick, near, coords, click, key, seekTimeline, artifacts, startPreset });
      }
      if (scope === "racecraft" || scope === "all") {
        await startPreset(page, c, "hairpin");
        await racecraftChecks(page, c, async (name, run) => {
          currentCheck = name; console.log(`${c.name}: ${name}`);
          await run(); checks.push(name);
        }, { state, wait, control, tick, near, key, coords, seekTimeline, artifacts, startPreset });
      }
      assert.deepEqual(errors, [], "browser errors");
      const final = await state(page);
      await fs.rm(path.join(artifacts, `${c.name}-failure.png`), {
        force: true,
      });
      reports.push({
        ...c,
        scope,
        checks,
        measurements,
        seconds: (Date.now() - started) / 1000,
        final,
      });
      console.log(`PASS ${c.name}: ${checks.join(", ")}`);
    } catch (e) {
      await page
        .screenshot({ path: path.join(artifacts, `${c.name}-failure.png`) })
        .catch(() => {});
      reports.push({
        ...c,
        scope,
        checks,
        measurements,
        failed: currentCheck,
        error: String(e),
        browserErrors: errors,
        state: await state(page).catch(() => null),
      });
      throw e;
    } finally {
      await context.close();
    }
  }
} finally {
  await fs.writeFile(
    path.join(artifacts, "report.json"),
    JSON.stringify(reports, null, 2) + "\n",
  );
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
