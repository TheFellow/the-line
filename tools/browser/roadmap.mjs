import assert from "node:assert/strict";
import path from "node:path";

// Uses real browser input only. Helpers read the opt-in inspection bridge.
export async function roadmapChecks(page, c, check, h) {
  const { state, wait, control, tick, near, artifacts } = h;
  await check("setup edits publish a verified provisional line and support undo", async () => {
    const before = await state(page);
    await control(page, "setup");
    assert.equal((await state(page)).setupOpen, true);
    await page.evaluate(() => {
      window.setupSamples = [];
      window.setupSampler = setInterval(() => {
        const s = JSON.parse(window.theLineSnapshot());
        window.setupSamples.push({ generation:s.generation, provisional:s.provisional, busy:s.busy, pathDigest:s.pathDigest, grip:s.config.grip });
      }, 30);
    });
    await control(page, "setup:grip+");
    const changed = await wait(page, s => !s.busy && s.config.name.startsWith("Custom"), "setup refinement completes");
    near(changed.config.grip, before.config.grip + .05, "grip step");
    assert.equal(changed.generation, before.generation + 1);
    assert.equal(changed.provisional, false);
    assert.ok(changed.forceResidual < .0005);
    const samples = await page.evaluate(() => { clearInterval(window.setupSampler); return window.setupSamples; });
    const provisional = samples.filter(s => s.generation === changed.generation && s.provisional);
    assert.ok(provisional.length, "observed actual provisional state before full result");
    for (const sample of provisional) assert.equal(sample.pathDigest, before.pathDigest, "provisional preserves current path geometry");
    await wait(page, s => !s.analyzing && s.sensitivity?.length === 4, "same-line sensitivities complete");
    await page.screenshot({path:path.join(artifacts, `${c.name}-setup.png`)});
    await control(page, "undo");
    const undone = await wait(page, s => !s.busy && !s.provisional, "setup undo solves");
    assert.deepEqual(undone.config, before.config);
    assert.equal(undone.lineDigest, before.lineDigest);
    await control(page, "redo");
    const redone = await wait(page, s => !s.busy, "setup redo solves");
    assert.deepEqual(redone.config, changed.config);
    await control(page, "undo");
    await wait(page, s => !s.busy, "restore setup baseline");
  });
  await check("rapid setup changes supersede stale solves", async () => {
    const before = await state(page);
    await control(page, "setup:mass+");
    await control(page, "setup:power+");
    const after = await wait(page, s => !s.busy && !s.provisional, "latest setup solve completes");
    near(after.config.mass, before.config.mass + 50, "last setup retains mass change");
    near(after.config.power, before.config.power + 25000, "last setup retains power change");
    assert.equal(after.generation, before.generation + 2);
    const digest = after.lineDigest;
    await tick(page);
    assert.equal((await state(page)).lineDigest, digest, "stale replies cannot replace latest result");
    await control(page, "undo");
    await wait(page, s => !s.busy, "undo power");
    await control(page, "undo");
    const restored = await wait(page, s => !s.busy, "undo mass");
    assert.deepEqual(restored.config, before.config);
    await control(page, "setup");
    assert.equal((await state(page)).setupOpen, false);
  });
  await check("axle setup page and standalone car persistence", async () => {
    const before = await state(page);
    await control(page, "setup");
    await control(page, "save-car");
    assert.ok((await state(page)).status.startsWith("Saved car"));
    await control(page, "setup-page");
    const paged = await state(page);
    assert.ok(paged.controls["setup:front_brake+"]);
    assert.equal(paged.controls["setup:mass+"], undefined, "hidden setup page cannot capture input");
    await control(page, "setup:front_brake+");
    await control(page, "setup:lift_area+");
    const changed = await wait(page, s => !s.busy && !s.provisional, "axle setup settles");
    near(changed.config.front_brake, .5, "first fixed brake bias is balanced");
    near(changed.config.lift_area, .25, "downforce step");
    assert.ok(changed.forceResidual < .0005);
    await page.screenshot({path:path.join(artifacts, `${c.name}-axle-setup.png`)});
    await control(page, "load-car");
    const restored = await wait(page, s => !s.busy && !s.provisional, "saved car reloads");
    assert.deepEqual(restored.config, before.config, "standalone car restores all setup parameters");
    await control(page, "setup-page");
    await control(page, "setup");
  });
}
