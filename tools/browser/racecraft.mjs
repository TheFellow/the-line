import assert from "node:assert/strict";
import path from "node:path";

export async function racecraftChecks(page,c,check,h) {
 const {state,wait,control,near,seekTimeline,artifacts,key,coords,tick,startPreset}=h;
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
  assert.equal(changed.race.config.gap_m,7);
  for(let i=0;i<4;i++){await control(page,"race-gap+");await wait(page,s=>!s.busy,"larger gap edit");}
  const larger=await state(page);assert.equal(larger.race.config.gap_m,11);assert.equal(larger.race.events.length,0,"larger gap prevents completed exit pass");
  await control(page,"race-overspeed+");let next=await wait(page,s=>!s.busy,"overspeed edit");assert.equal(next.race.config.overspeed_mps,4);
  await control(page,"race-load");next=await wait(page,s=>!s.busy,"load saved race");assert.equal(next.race.config.gap_m,6);assert.equal(next.race.config.overspeed_mps,3);
  // An impossible separation edit must leave the exact last good inputs active.
  for(let i=0;i<5;i++){
   const before=await state(page);
   await control(page,"race-separation+");const edited=await wait(page,s=>!s.busy,"separation edit");
   if(edited.status.includes("rejected")){
    assert.deepEqual(edited.race,before.race);assert.deepEqual(edited.camera,before.camera);near(edited.time,before.time,"failed plan retains playback");
   }
  }
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

 await check("failed race loads and qualifying keys preserve accepted state",async()=>{
  await key(page,"r");await wait(page,s=>!s.busy&&s.race!==null,"race enters");
  await control(page,"race-save");
  const saved=await page.evaluate(()=>localStorage.getItem("the-line:race:racecraft.json"));
  await seekTimeline(page,.4);const before=await state(page);
  await page.evaluate(()=>localStorage.setItem("the-line:race:racecraft.json","{bad json"));
  await control(page,"race-load");const rejected=await state(page);
  assert.deepEqual(rejected.race,before.race);assert.deepEqual(rejected.camera,before.camera);near(rejected.time,before.time,"failed load retains playback");
  assert.ok(rejected.status.length>0);assert.equal(rejected.busy,false);
  await key(page,"ArrowLeft");const blocked=await state(page);
  assert.match(blocked.status,/Qualifying controls are inactive/);assert.deepEqual(blocked.scene,before.scene);
  await page.evaluate(saved=>localStorage.setItem("the-line:race:racecraft.json",saved),saved);
  await key(page,"r");
 });
 await check("held race timeline and camera gestures stop at mode exit",async()=>{
  for(const gesture of ["timeline","camera"]){
   const before=await state(page);
   await key(page,"r");await wait(page,s=>!s.busy&&s.race!==null,"race enters");
   const race=await state(page),r=gesture==="timeline"?race.controls.scrub:race.roadViewport;
   const p=await coords(page,race,(r[0]+r[2])/2,(r[1]+r[3])/2);
   await page.mouse.move(p.x,p.y);await page.mouse.down({button:gesture==="timeline"?"left":"middle"});await tick(page);
   if(gesture==="camera") assert.equal((await state(page)).cameraDragging,true);
   await key(page,"r");
   await page.mouse.move(p.x+30,p.y+10);await tick(page);
   const restored=await state(page);
   assert.equal(restored.race,null);assert.equal(restored.cameraDragging,false);
   near(restored.time,0,"held timeline cannot scrub restored study");assert.deepEqual(restored.camera,before.camera);
   await page.mouse.up({button:gesture==="timeline"?"left":"middle"});await tick(page);
  }
 });
 await check("perspective study returns with its renderer and analysis settings",async()=>{
  const before=await state(page);
  await control(page,"watch");const perspective=await state(page);assert.equal(perspective.view,"perspective");
  await key(page,"r");const race=await wait(page,s=>!s.busy&&s.race!==null,"perspective enters race");assert.equal(race.view,"3d");
  await key(page,"r");const restored=await state(page);
  assert.equal(restored.view,"perspective");assert.deepEqual(restored.camera,perspective.camera);assert.equal(restored.lineDigest,perspective.lineDigest);
  assert.equal(restored.analysis,perspective.analysis);assert.equal(restored.chartChannel,perspective.chartChannel);
  await control(page,"watch");
  if((await state(page)).view!==before.view) await control(page,"view");
 });
 await check("delayed CSV reads cannot cross the race mode boundary",async()=>{
  for(const finishInRace of [true,false]){
   const before=await state(page);
   await control(page,"authoring");
   // Delay the actual browser File read, then complete the real chooser. No
   // application state or inspection bridge is modified by this interleaving.
   await page.evaluate(()=>{
    const original=File.prototype.text;
    window.releaseCSVRead=null;
    File.prototype.text=function(){
     File.prototype.text=original;
     return new Promise((resolve,reject)=>{window.releaseCSVRead=()=>original.call(this).then(resolve,reject);});
    };
   });
   const selected=page.waitForEvent("filechooser");const clicking=control(page,"import-csv");
   const chooser=await selected;
   await chooser.setFiles({name:"late-race-import.csv",mimeType:"text/csv",buffer:Buffer.from("x,y,z,width_left,width_right\n-100,0,0,6,6\n0,0,0,6,6\n100,20,0,6,6\n")});
   await clicking;await page.waitForFunction(()=>typeof window.releaseCSVRead==="function");
   await key(page,"r");await wait(page,s=>!s.busy&&s.race!==null,"race enters with CSV pending");
   if(!finishInRace) await key(page,"r");
   await page.evaluate(()=>window.releaseCSVRead());await tick(page);
   const afterRead=await state(page);
   assert.deepEqual(afterRead.scene,before.scene);assert.equal(afterRead.solveRequests,before.solveRequests);
   if(finishInRace) await key(page,"r");
   const restored=await state(page);
   assert.equal(restored.lineDigest,before.lineDigest);assert.deepEqual(restored.solvedScene,before.solvedScene);assert.deepEqual(restored.camera,before.camera);
   await control(page,"authoring");
  }
 });
 await check("next example explicitly resets custom road, car and controls",async()=>{
  await key(page,"r");await wait(page,s=>!s.busy&&s.race!==null,"race enters");await control(page,"race-save");
  const baseline=await page.evaluate(()=>JSON.parse(localStorage.getItem("the-line:race:racecraft.json")));
  const custom=structuredClone(baseline);custom.scene.name="Custom race road";custom.scene.points.forEach(p=>{p.x+=15;});custom.vehicle.mass*=1.01;custom.racecraft.gap_m=7;
  await page.evaluate(e=>localStorage.setItem("the-line:race:racecraft.json",JSON.stringify(e)),custom);
  await control(page,"race-load");await wait(page,s=>!s.busy,"custom race loaded");await control(page,"race-save");
  assert.deepEqual(await page.evaluate(()=>JSON.parse(localStorage.getItem("the-line:race:racecraft.json"))),custom);
  await control(page,"race-scenario");const next=await wait(page,s=>!s.busy,"next example");
  assert.match(next.status,/reset road, vehicle and controls/);assert.equal(next.race.config.scenario,"pass-repass");await control(page,"race-save");
  const saved=await page.evaluate(()=>JSON.parse(localStorage.getItem("the-line:race:racecraft.json")));
  assert.deepEqual(saved.scene.points,baseline.scene.points);assert.deepEqual(saved.vehicle,baseline.vehicle);assert.notEqual(saved.racecraft.gap_m,custom.racecraft.gap_m);
  await key(page,"r");
 });
 await check("pending race plan can exit and cannot overwrite restored qualifying",async()=>{
  const before=await state(page);
  await key(page,"r");await wait(page,s=>!s.busy&&s.race!==null,"race enters");await control(page,"race-save");
  await page.evaluate(()=>{
   const e=JSON.parse(localStorage.getItem("the-line:race:racecraft.json"));
   e.scene.points.forEach(p=>{p.x*=10;p.y*=10;});
   localStorage.setItem("the-line:race:racecraft.json",JSON.stringify(e));
  });
  const s=await state(page),r=s.controls["race-load"],p=await coords(page,s,(r[0]+r[2])/2,(r[1]+r[3])/2);
  await page.mouse.move(p.x,p.y);await page.mouse.down();
  await wait(page,s=>s.busy,"custom plan starts");await page.mouse.up();
  await key(page,"r");const restored=await wait(page,s=>!s.busy&&s.race===null,"pending plan cancelled");
  assert.deepEqual(restored.scene,before.scene);assert.equal(restored.lineDigest,before.lineDigest);assert.deepEqual(restored.camera,before.camera);
  await tick(page);await tick(page);assert.equal((await state(page)).race,null,"cancelled reply stays detached");
 });
 await check("race file implies startup mode and vehicle override applies to saved race",async()=>{
  // Reuse the complete saved input through the normal browser persistence API.
  await key(page,"r");await wait(page,s=>!s.busy&&s.race!==null,"race enters");await control(page,"race-save");
  const url=new URL(page.url());url.searchParams.set("mode","qualifying");url.searchParams.set("race-file","racecraft.json");url.searchParams.set("vehicle","gt");
  await page.goto(url.href);await wait(page,s=>s.updates>5&&!s.busy&&s.race!==null,"race-file startup");
  await control(page,"play");await control(page,"race-save");
  const saved=await page.evaluate(()=>JSON.parse(localStorage.getItem("the-line:race:racecraft.json")));
  assert.match(saved.vehicle.name,/^GT /);
  await key(page,"r");const qualifying=await state(page);assert.deepEqual(qualifying.config,saved.vehicle);
  await startPreset(page,c,"hairpin");
 });
}
