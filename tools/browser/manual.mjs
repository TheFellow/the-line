import assert from "node:assert/strict";
import path from "node:path";

// Actual pointer/key/storage workflows; no application-state injection.
export async function manualChecks(page, c, check, h) {
  const { state, wait, control, tick, near, coords, key, seekTimeline, artifacts } = h;
  const settled = () => wait(page, s => !s.busy && !s.manualDragging, "authored study settles");
  async function dragByOffset(amount) {
    const before = await state(page), viewport = before.roadViewport;
    const candidates = before.manualPoints.map((point,index) => ({point,index,axis:before.manualAxes[index]}))
      .filter(({point,axis}) => point[0]>viewport[0]+25 && point[0]<viewport[2]-25 && point[1]>viewport[1]+25 && point[1]<viewport[3]-25 && Math.hypot(...axis)>.5)
      .sort((a,b)=>Math.hypot(...b.axis)-Math.hypot(...a.axis));
    assert.ok(candidates.length,"a lateral handle is visible");
    const {point,index,axis}=candidates[0];
    const start=await coords(page,before,point[0]+2,point[1]-2);
    const end=await coords(page,before,point[0]+2+axis[0]*amount,point[1]-2+axis[1]*amount);
    await page.mouse.move(start.x,start.y);await page.mouse.down();
    const grabbed=await wait(page,s=>s.manualDragging,"manual handle captures press");
    assert.equal(grabbed.dragging,false,"manual drag cannot select geometry");
    await page.mouse.move(end.x,end.y,{steps:6});await tick(page);
    const held=await state(page);
    const delta=held.pointer.map((v,i)=>v-grabbed.pointer[i]);
    const expected=before.manualHandles[index].offset+(delta[0]*axis[0]+delta[1]*axis[1])/(axis[0]**2+axis[1]**2);
    near(held.manualHandles[index].offset,expected,"banked screen projection predicts lateral preview");
    assert.deepEqual(held.scene,before.scene,"preview does not commit history");
    return {before,held,index,expected};
  }
  await check("pinned car reference retains its profile and exact station delta",async()=>{
    const before=await settled();
    await control(page,"pin");
    const pinned=await state(page);near(pinned.referenceDuration,before.duration,"pinned current time");
    await control(page,"setup");await control(page,"setup:grip+");const tuned=await settled();
    near(tuned.referenceDuration,before.duration,"setup does not change pinned trajectory");
    assert.equal(tuned.referenceStale,false);
    await seekTimeline(page,1);const end=await state(page);
    near(end.delta,end.duration-end.referenceDuration,"same-station exit delta equals duration difference");
    await control(page,"undo");await settled();await control(page,"setup");
    await page.screenshot({path:path.join(artifacts,`${c.name}-pinned-reference.png`)});
  });
  await check("manual lateral handles track rotated banked cameras and cancel cleanly",async()=>{
    await control(page,"manual");const zero=await settled();
    assert.equal(zero.manualMode,true);near(zero.duration,zero.centerDuration,"zero manual offsets equal centreline");
    if(zero.view==="3d") {
      const r=zero.roadViewport,p=await coords(page,zero,(r[0]+r[2])/2,(r[1]+r[3])/2);
      await page.mouse.move(p.x,p.y);await page.mouse.down({button:"right"});await wait(page,s=>s.cameraDragging,"manual mode camera orbit");
      await page.mouse.move(p.x+60,p.y+9,{steps:6});await tick(page);await page.mouse.up({button:"right"});await tick(page);
    }
    const moved=await dragByOffset(1.3);
    await page.screenshot({path:path.join(artifacts,`${c.name}-manual-preview.png`)});
    await page.mouse.up();const accepted=await settled();
    near(accepted.manualHandles[moved.index].offset,moved.expected,"released handle commits one evaluated line",1e-5);
    assert.deepEqual(accepted.scene.points,moved.before.scene.points,"lateral editing preserves road geometry");
    assert.ok(accepted.forceResidual<.0005);
    const cancelled=await dragByOffset(-.5);await key(page,"Escape");
    assert.equal((await state(page)).manualDragging,false);await page.mouse.up();await tick(page);
    const after=await state(page);assert.deepEqual(after.scene,cancelled.before.scene);assert.equal(after.lineDigest,cancelled.before.lineDigest);
  });
  await check("infeasible authored line reports station and preserves undo and redo",async()=>{
    await control(page,"undo");const before=await settled();assert.equal(before.canRedo,true);
    await dragByOffset(12);await page.mouse.up();await tick(page);
    const rejected=await state(page);
    assert.match(rejected.status,/station.*clearance|clearance.*station/);
    assert.ok(rejected.manualFailure,"failing station highlighted");
    assert.deepEqual(rejected.scene,before.scene);assert.equal(rejected.lineDigest,before.lineDigest);
    assert.equal(rejected.canRedo,true);assert.equal(rejected.canUndo,before.canUndo);
    await page.screenshot({path:path.join(artifacts,`${c.name}-manual-rejected.png`)});
    await control(page,"redo");await settled();
  });
  await check("authored studies restore pinned profiles and detect stale geometry",async()=>{
    await control(page,"manual-adopt");const saved=await state(page);
    near(saved.referenceDuration,saved.duration,"adopt pins authored profile");
    await control(page,"save");assert.match((await state(page)).status,/Saved/);
    await control(page,"unpin");await control(page,"manual-zero");await settled();
    await control(page,"load");const restored=await settled();
    assert.equal(restored.manualMode,true);assert.equal(restored.referenceName,saved.referenceName);
    assert.equal(restored.lineDigest,saved.lineDigest);near(restored.referenceDuration,saved.referenceDuration,"reference time survives actual storage roundtrip");
    await page.screenshot({path:path.join(artifacts,`${c.name}-manual-study.png`)});
    await control(page,"manual");await control(page,"width+");const stale=await settled();
    assert.equal(stale.referenceStale,true);near(stale.playbackDuration,stale.duration,"stale ghost disabled");
    await page.screenshot({path:path.join(artifacts,`${c.name}-stale-reference.png`)});
    await control(page,"undo");const valid=await settled();assert.equal(valid.referenceStale,false);
    if (!(await state(page)).manualMode) await control(page,"manual");
    const authored=await settled();
    await control(page,"manual-optimize");const optimized=await settled();
    assert.equal(optimized.manualMode,false);assert.ok(optimized.duration<=authored.duration+1e-9,"seeded optimization retains better verified manual candidate");
    await control(page,"unpin");await control(page,"fit");
  });
  await check("optimized study selection survives save, load, undo and vehicle changes",async()=>{
    const before=await settled();assert.equal(before.scene.study.active_line,"optimized");
    const hypothesis=before.scene.study.manual;
    await control(page,"save");assert.match((await state(page)).status,/Saved/);
    await control(page,"manual");const authored=await settled();assert.equal(authored.scene.study.active_line,"manual");
    await control(page,"load");const loaded=await settled();assert.equal(loaded.manualMode,false);assert.equal(loaded.scene.study.active_line,"optimized");
    assert.deepEqual(loaded.scene.study.manual,hypothesis,"optimization retains the authored hypothesis");
    await control(page,"undo");const undone=await settled();assert.equal(undone.manualMode,true);assert.equal(undone.scene.study.active_line,"manual");
    await control(page,"redo");const redone=await settled();assert.equal(redone.manualMode,false);assert.equal(redone.lineDigest,loaded.lineDigest);
    await control(page,"vehicle");const changed=await settled();assert.equal(changed.scene.vehicle,"gt");assert.equal(changed.manualMode,false);assert.equal(changed.scene.study.active_line,"optimized");
    assert.deepEqual(changed.scene.study.manual,hypothesis);
    await control(page,"save");await control(page,"load");const restored=await settled();assert.equal(restored.manualMode,false);assert.equal(restored.scene.vehicle,"gt");assert.equal(restored.scene.study.active_line,"optimized");
    assert.ok(restored.forceResidual<.0005);assert.deepEqual(restored.scene.study.manual,hypothesis);
    await page.screenshot({path:path.join(artifacts,`${c.name}-optimized-study.png`)});
  });

}
