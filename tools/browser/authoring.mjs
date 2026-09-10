import assert from "node:assert/strict";
import path from "node:path";

export async function authoringChecks(page, c, check, h) {
  const { state, wait, control, near, artifacts } = h;
  const settled = () => wait(page, s => !s.busy, "road authoring settles");
  const left = p => p.width_left ?? p.width / 2;
  async function upload(text, filename) {
    const selected = page.waitForEvent("filechooser");
    // The native chooser can suspend game updates. Complete it while the
    // ordinary click helper is still waiting for its post-input frames.
    const clicking = control(page, "import-csv");
    const chooser = await selected;
    await chooser.setFiles({ name:filename, mimeType:"text/csv", buffer:Buffer.from(text) });
    await clicking;
  }
  await check("asymmetric road-width controls commit and undo one physical edit", async () => {
    const before = await settled();
    await control(page, "authoring");
    assert.ok((await state(page)).controls["road:left+"]);
    await control(page, "road:left+");
    const changed = await settled(), i = before.selected;
    assert.equal(changed.scene.version, 2);
    near(changed.scene.points[i].width_left, left(before.scene.points[i]) + .5, "left width changes independently");
    near(changed.scene.points[i].width_right, before.scene.points[i].width_right ?? before.scene.points[i].width / 2, "right width retained");
    assert.ok(changed.forceResidual < .0005);
    await control(page, "undo"); const restored = await settled();
    assert.deepEqual(restored.scene, before.scene);
    assert.equal(restored.lineDigest, before.lineDigest);
  });
  await check("kerbs have physical widths and excluded grip cannot change the line", async () => {
    const before = await settled();
    await control(page, "road:kerb-left+"); await settled();
    await control(page, "road:grip-left+");
    const excluded = await settled();
    near(excluded.scene.points[before.selected].kerb_left.width, .5, "kerb width step");
    near(excluded.scene.points[before.selected].kerb_left.grip, .85, "kerb grip step");
    assert.equal(excluded.lineDigest, before.lineDigest, "excluded kerb cannot affect trajectory");
    await control(page, "road:limits"); const enabled = await settled();
    assert.equal(enabled.scene.kerbs_count_as_road, true);
    assert.ok(enabled.forceResidual < .0005);
    await page.screenshot({path:path.join(artifacts, `${c.name}-road-limits.png`)});
    for(let n=0;n<3;n++) { await control(page, "undo"); await settled(); }
    assert.deepEqual((await state(page)).scene, before.scene);
  });
  await check("actual CSV chooser validates imports and preserves failed-import history", async () => {
    const before = await settled();
    const csv = "x,y,z,width_left,width_right,bank,surface\n-100,0,0,4,7,0,asphalt\n0,0,2,5,7,2,asphalt\n100,30,4,5,6,0,asphalt\n";
    await upload(csv, "headless-centreline.csv");
    const imported = await wait(page, s => !s.busy && s.scene.name === "headless-centreline.csv", "CSV import solves");
    assert.equal(imported.scene.points.length, 3);
    assert.equal(imported.scene.points[0].width_left, 4);
    assert.equal(imported.scene.points[0].width_right, 7);
    assert.ok(imported.forceResidual < .0005);
    await page.screenshot({path:path.join(artifacts, `${c.name}-csv-import.png`)});
    await control(page, "undo"); const undone = await settled();
    assert.deepEqual(undone.scene, before.scene);
    assert.equal(undone.canRedo, true);
    await upload("x,y,z,width_left,width_right\n0,0,0,6,6\n0,0,0,6,6\n", "bad-centreline.csv");
    const rejected = await wait(page, s => s.status.includes("must be at least 1 metre apart"), "invalid CSV rejected");
    assert.deepEqual(rejected.scene, undone.scene);
    assert.equal(rejected.lineDigest, undone.lineDigest);
    assert.equal(rejected.canRedo, true);
    await control(page, "authoring");
  });
}
