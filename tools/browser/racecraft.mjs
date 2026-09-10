import assert from "node:assert/strict";
import path from "node:path";

export async function racecraftChecks(page,c,check,h) {
 const {state,wait,control,near,seekTimeline,artifacts,key}=h;
 let qualifying;
 const safe=s=>{
  const [a,b]=s.race.nodes;
  assert.ok(Math.hypot(a.position.X-b.position.X,a.position.Y-b.position.Y)>=2*s.race.radius+s.race.config.clearance_m-1e-8,"solid bodies remain separated");
 };
 await check("race mode uses two certified cars and restores qualifying",async()=>{
  qualifying=await state(page);
  await control(page,"race-mode");
  const race=await wait(page,s=>!s.busy&&s.race!==null,"race settles");
  assert.equal(race.race.config.scenario,"over-under");assert.equal(race.race.events.length,1);safe(race);
  await seekTimeline(page,.91);safe(await state(page));
  await page.screenshot({path:path.join(artifacts,`${c.name}-race-over-under.png`)});
  await key(page,"r");
  const restored=await wait(page,s=>s.race===null,"qualifying restored");
  assert.deepEqual(restored.scene,qualifying.scene);assert.equal(restored.lineDigest,qualifying.lineDigest);
  await key(page,"r");await wait(page,s=>!s.busy&&s.race!==null,"race resumed");
 });
 await check("race controls alter outcomes and save/load exact inputs",async()=>{
  await control(page,"race-save");
  await control(page,"race-gap+");const changed=await wait(page,s=>!s.busy,"gap edit");
  assert.equal(changed.race.config.gap_m,7);assert.equal(changed.race.events.length,0,"larger gap prevents completed exit pass");
  await control(page,"race-overspeed+");let next=await wait(page,s=>!s.busy,"overspeed edit");assert.equal(next.race.config.overspeed_mps,4);
  await control(page,"race-load");next=await wait(page,s=>!s.busy,"load saved race");assert.equal(next.race.config.gap_m,6);assert.equal(next.race.config.overspeed_mps,3);
  // An impossible separation edit must leave the exact last good inputs active.
  for(let i=0;i<5;i++){await control(page,"race-separation+");await wait(page,s=>!s.busy,"separation edit");}
  next=await state(page);assert.match(next.status,/rejected/);assert.ok(next.race.config.separation_m<=6.5);safe(next);
  await control(page,"race-load");await wait(page,s=>!s.busy,"restore example");
 });
 await check("pass/repass scenario, shared scrubbing and both views",async()=>{
  await control(page,"race-scenario");const s=await wait(page,s=>!s.busy,"pass/repass example");
  assert.equal(s.race.config.scenario,"pass-repass");assert.deepEqual(s.race.events.map(e=>e.leader),[1,0]);
  await seekTimeline(page,.55);const middle=await state(page);safe(middle);
  assert.ok(middle.race.nodes[1].station>middle.race.nodes[0].station,"B leads midcorner");
  await seekTimeline(page,.99);const finish=await state(page);safe(finish);
  assert.ok(finish.race.nodes[0].station>finish.race.nodes[1].station,"A recovers exit");
  await key(page,",");const stepped=await state(page);near(stepped.time,finish.time-1/60,"frame step shares race time",1e-5);
  await page.screenshot({path:path.join(artifacts,`${c.name}-race-repass.png`)});
  await control(page,"view");await page.screenshot({path:path.join(artifacts,`${c.name}-race-other-view.png`)});
  await control(page,"restart");const restart=await state(page);near(restart.time,0,"both cars restart");safe(restart);
  await control(page,"race-mode");const restored=await state(page);assert.equal(restored.lineDigest,qualifying.lineDigest);
 });
}
