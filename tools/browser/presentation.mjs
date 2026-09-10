import assert from "node:assert/strict";
import path from "node:path";

// Real transport and watch-only input, with read-only trajectory observations.
export async function presentationChecks(page,c,check,h){
 const {state,wait,control,tick,near,coords,key,seekTimeline,artifacts}=h;
 const settled=()=>wait(page,s=>!s.busy,"presentation settles");
 await check("frame and station stepping preserve geometry and exact station delta",async()=>{
  await settled();await seekTimeline(page,.4);
  const before=await state(page);
  await control(page,"frame+");let next=await state(page);near(next.time,before.time+1/60,"one frame is 1/60 second");assert.equal(next.playing,false);
  await key(page,",");next=await state(page);near(next.time,before.time,"comma steps backwards one frame");
  await control(page,"station+");next=await state(page);near(next.current.station,before.current.station+5,"station step advances 5 road metres",1e-5);
  await page.keyboard.down("Shift");await key(page,",");await page.keyboard.up("Shift");next=await state(page);near(next.current.station,before.current.station,"shift-comma steps backwards 5 road metres",1e-5);
  assert.deepEqual(next.scene,before.scene);assert.equal(next.solveRequests,before.solveRequests);assert.equal(next.lineDigest,before.lineDigest);
 });
 await check("sector splits telescope to the same-station finish delta",async()=>{
  await control(page,"analysis");const s=await state(page);assert.equal(s.analysis,true);assert.ok(s.sectors.length>0);
  near(s.sectors.reduce((a,b)=>a+b.Current,0),s.duration,"current sectors sum to duration");
  near(s.sectors.reduce((a,b)=>a+b.Reference,0),s.referenceDuration,"reference sectors sum to duration");
  near(s.sectors.reduce((a,b)=>a+b.Delta,0),s.duration-s.referenceDuration,"sector deltas sum to finish delta");
  await page.screenshot({path:path.join(artifacts,`${c.name}-sectors.png`)});
 });
 await check("perspective watch-only view ignores geometry and camera drags",async()=>{
  const before=await state(page);await control(page,"watch");let s=await wait(page,s=>s.view==="perspective","trackside perspective displayed");
  const viewport=s.roadViewport, visible=s.points.find(p=>p[0]>viewport[0]+10&&p[0]<viewport[2]-10&&p[1]>viewport[1]+10&&p[1]<viewport[3]-10);
  assert.ok(visible,"a projected geometry control is inside the perspective viewport");const p=await coords(page,s,...visible);
  for(const button of ["left","right"]){await page.mouse.move(p.x,p.y);await page.mouse.down({button});await tick(page);await page.mouse.move(p.x+40,p.y+20,{steps:4});await tick(page);s=await state(page);assert.equal(s.dragging,false);assert.equal(s.cameraDragging,false);assert.equal(s.manualDragging,false);await page.mouse.up({button});await tick(page)}
  await key(page,"ArrowRight");s=await state(page);assert.deepEqual(s.scene,before.scene);assert.equal(s.solveRequests,before.solveRequests);assert.equal(s.lineDigest,before.lineDigest);
  await control(page,"frame+");const moved=await state(page);near(moved.time,s.time+1/60,"watch transport remains interactive");
  await page.screenshot({path:path.join(artifacts,`${c.name}-perspective.png`)});
  await control(page,"watch");await wait(page,s=>s.view==="3d","return to orthographic edit view");
  await control(page,"analysis");
  if(before.view==="2d"){await control(page,"view");await wait(page,s=>s.view==="2d","restore plan view")}
 });
}
