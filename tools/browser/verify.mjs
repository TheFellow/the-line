import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import fs from "node:fs/promises";
import { existsSync } from "node:fs";
import http from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright-core";

const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../..",
);
const artifacts = path.join(root, "artifacts/browser");
const selected = process.argv.find((a) => a.startsWith("--case="))?.slice(7);
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
const go=new Go();const params=new URLSearchParams(location.search);go.argv=['studio','--preset','banked','--view',params.get('view')||'2d'];
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
      throw new Error(
        `${label}: ${e.message}\n${JSON.stringify(await state(page))}`,
      );
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
  const p = await coords(page, s, (r[0] + r[2]) / 2, (r[1] + r[3]) / 2);
  await click(page, p.x, p.y);
}
function near(a, b, label, tolerance = 1e-6) {
  assert.ok(Math.abs(a - b) < tolerance, `${label}: ${a} != ${b}`);
}
async function begin(page, index, dx = 4, dy = 3) {
  const s = await state(page);
  const p = s.points[index];
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
    let currentCheck = "startup";
    const started = Date.now();
    try {
      await page.goto(`${base}/?view=${c.view}`);
      await wait(page, (s) => s.updates > 5 && !s.busy, "startup");
      await control(page, "play");
      await wait(page, (s) => !s.playing, "pause animation");
      const original = await state(page);
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
        scrubbed.time / scrubbed.duration,
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
        scrubbed.time / scrubbed.duration,
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
      near(scrubbed.time, scrubbed.duration, "scrub clamps end");
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
      assert.deepEqual(errors, [], "browser errors");
      const final = await state(page);
      await fs.rm(path.join(artifacts, `${c.name}-failure.png`), {
        force: true,
      });
      reports.push({
        ...c,
        checks,
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
        checks,
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
