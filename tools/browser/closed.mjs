import assert from "node:assert/strict";
import path from "node:path";

export async function closedChecks(page,c,check,h) {
 const {state,wait,control,tick,near,coords,seekTimeline,artifacts}=h;
 await check("continuous laps wrap each car at its own verified period",async()=>{
  const before=await state(page);assert.equal(before.closed,true);
  near(before.startNode.speed,before.endNode.speed,"periodic seam speed",1e-7);
  near(before.playbackDuration,before.duration,"timeline shows one current lap");
  await seekTimeline(page,.99);await control(page,"play");
  const after=await wait(page,s=>s.time>s.duration+.15,"current car crosses seam without restarting global clock");
  near(after.current.time,after.time%after.duration,"current wraps own lap",1e-6);
  near(after.centerCurrent.time,after.time%after.referenceDuration,"ghost wraps own lap",1e-6);
  assert.equal(after.lineDigest,before.lineDigest);assert.equal(after.solveRequests,before.solveRequests);
  assert.ok(after.current.station<before.endNode.station*.2,"current returns near start");
  await page.screenshot({path:path.join(artifacts,`${c.name}-closed-seam.png`)});
  await control(page,"play");await control(page,"frame+");const frame=await state(page);
  near(frame.current.time,frame.time%frame.duration,"paused frame stepping after seam stays on later lap",1e-6);
  await control(page,"watch");await tick(page);
  await page.screenshot({path:path.join(artifacts,`${c.name}-closed-perspective.png`)});
  await control(page,"watch");
 });
 await check("closed topology toggle and undo restore the exact periodic solve",async()=>{
  const before=await state(page);await control(page,"closed");
  const open=await wait(page,s=>!s.busy,"open topology settles");assert.equal(open.closed,false);
  await control(page,"undo");const restored=await wait(page,s=>!s.busy,"closed topology undo settles");
  assert.equal(restored.closed,true);assert.equal(restored.lineDigest,before.lineDigest);assert.deepEqual(restored.scene,before.scene);
 });
 await check("one manual seam handle edits both periodic endpoints",async()=>{
  if ((await state(page)).view!==c.view) await control(page,"view");
  // View switches retain zoom. Refit after the preceding 3D topology check so
  // the entire plan-view loop remains visible for the seam interaction capture.
  await control(page,"fit");
  await control(page,"manual");const before=await wait(page,s=>!s.busy,"closed manual baseline");
  const point=before.manualPoints[0],axis=before.manualAxes[0];
  const start=await coords(page,before,...point),end=await coords(page,before,point[0]+axis[0]*1.2,point[1]+axis[1]*1.2);
  await page.mouse.move(start.x,start.y);await page.mouse.down();
  const grabbed=await wait(page,s=>s.manualDragging,"periodic seam handle captures");
  assert.equal(grabbed.dragging,false);await page.mouse.move(end.x,end.y,{steps:6});await tick(page);
  const held=await state(page),delta=held.pointer.map((v,i)=>v-grabbed.pointer[i]);
  const expected=(delta[0]*axis[0]+delta[1]*axis[1])/(axis[0]**2+axis[1]**2);
  near(held.manualHandles[0].offset,expected,"seam preview follows actual screen displacement",1e-6);
  await page.mouse.up();const after=await wait(page,s=>!s.busy&&!s.manualDragging,"periodic authored line settles");
  const offsets=after.scene.study.manual.offsets;
  near(offsets[0],expected,"authored start offset",1e-6);near(offsets.at(-1),expected,"duplicate seam offset mirrors start",1e-6);
  near(after.startNode.speed,after.endNode.speed,"authored seam speed remains periodic",1e-6);
  assert.deepEqual(after.scene.points,before.scene.points);assert.ok(after.forceResidual<.0005);
  await page.screenshot({path:path.join(artifacts,`${c.name}-closed-manual-seam.png`)});
  await control(page,"undo");const restored=await wait(page,s=>!s.busy,"periodic manual undo");
  assert.equal(restored.lineDigest,before.lineDigest);
 });

}
