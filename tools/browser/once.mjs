import assert from "node:assert/strict";
import path from "node:path";

// Exercise asynchronous recovery through real controls and the CSV chooser.
export async function onceChecks(page, c, check, h) {
  const { state, wait, control, near, artifacts, startPreset } = h;
  const settled = () => wait(page, s => !s.busy && !s.provisional, "setup settles");
  await check("wider car replaces an infeasible old line with a fresh solve", async () => {
    await startPreset(page, c, "esses");
    const before = await state(page);
    await page.evaluate(() => {
      window.freshSamples = [];
      window.freshSampler = setInterval(() => {
        const s = JSON.parse(window.theLineSnapshot());
        window.freshSamples.push({status:s.status, busy:s.busy, provisional:s.provisional});
      }, 20);
    });
    await control(page, "vehicle");
    const changed = await settled();
    const samples = await page.evaluate(() => { clearInterval(window.freshSampler); return window.freshSamples; });
    assert.ok(samples.some(s => s.busy && s.status.includes("finding a fresh line")), "GUI discards the infeasible old-car line");
    assert.equal(changed.scene.vehicle, "gt");
    assert.ok(changed.config.width > before.config.width);
    assert.notEqual(changed.pathDigest, before.pathDigest);
    assert.ok(changed.duration <= changed.centerDuration && changed.forceResidual < .0005);
    assert.deepEqual(changed.solvedScene, changed.scene, "displayed trajectory owns its captured scene");
    await page.screenshot({path:path.join(artifacts,`${c.name}-wider-car.png`)});
  });
  await check("failed superseding edit preserves and solves its valid predecessor", async () => {
    await startPreset(page, c, "esses");
    const baseline = await state(page);
    const csv = "x,y,z,width_left,width_right\n" + baseline.scene.points.map(p => `${p.x},${p.y},${p.z||0},1.51,1.51`).join("\n") + "\n";
    await control(page, "authoring");
    const chosen = page.waitForEvent("filechooser");
    const clicking = control(page, "import-csv");
    await (await chosen).setFiles({name:"narrow-esses.csv",mimeType:"text/csv",buffer:Buffer.from(csv)});
    await clicking;
    await settled();
    await control(page, "authoring");
    await control(page, "setup");
    // 2.50 m still fits the 3.02-metre road and its clearance margin.
    for (let i=0;i<14;i++) await control(page, "setup:width+");
    const before = await settled();
    near(before.config.width, 2.50, "narrow-road setup baseline");
    await control(page, "setup:mass+");
    const pending = await state(page);
    assert.equal(pending.busy, true, "valid predecessor must still be running");
    near(pending.requestedConfig.mass, before.config.mass + 50, "first edit committed");
    // 2.55 m fails the clearance guard during solving, after validation.
    await control(page, "setup:width+");
    const recovered = await settled();
    near(recovered.config.mass, before.config.mass + 50, "valid mass edit survives rejected width");
    near(recovered.config.width, before.config.width, "only invalid width edit reverted");
    assert.deepEqual(recovered.requestedConfig, recovered.config);
    assert.deepEqual(recovered.solvedScene, recovered.scene);
    assert.ok(recovered.generation >= before.generation + 3, "cancelled predecessor resumed after failed superseding solve");
    assert.ok(recovered.forceResidual < .0005);
    await page.screenshot({path:path.join(artifacts,`${c.name}-superseded-recovery.png`)});
    await control(page, "undo");
    const undone = await settled();
    assert.deepEqual(undone.config, before.config, "one undo removes just the preserved predecessor");
    await control(page, "redo");
    const redone = await settled();
    assert.deepEqual(redone.config, recovered.config, "failed edit never enters redo history");
  });
}
