import { createPredictor } from "../web/runtime.ts";

const fixtures = await (await fetch("/fixtures.json")).json();
const report = window.presentationReport = { cases: [], passed: false, ready: false, done: false };
const clone = value => structuredClone(value);
const check = (ok, message) => { if (!ok) throw Error(message); };
const run = async (name, test) => { try { report.cases.push({name, passed:true, ...await test()}); } catch(error) { report.cases.push({name, passed:false, error:String(error)}); } };
async function predictor(scene) { const p=await createPredictor(""); p.update(clone(fixtures[scene])); return p; }

await run("fog authority owns health, entities, world lifecycle and income", async()=>{
  const p=await predictor("fog"), now=performance.now(), initial=p.sample(now), final=p.sample(now+240);
  const key=s=>JSON.stringify({plants:s.plants.map(v=>[v.id,v.hp,v.state]),zombies:s.zombies.map(v=>[v.id,v.hp,v.armor,v.state]),projectiles:s.projectiles.map(v=>v.id),mowers:s.mowers.map(v=>v.state),craters:s.craters,terrain:s.terrain,pickups:s.pickups,money:s.players.map(v=>v.money),events:s.events});
  const before=key(initial), after=key(final); p.dispose();
  check(before===after, "projection changed authority lifecycle: "+before+" => "+after);
  return {before,after};
});
for(const scene of ["day","fog"]) await run(scene+" fractional same-timeline motion at 60 and 144 Hz", async()=>{
  const p=await predictor(scene), now=performance.now(), samples=[];
  for(const ms of [0,1000/144,1000/60,2000/60,6000/144]) {const view=p.sample(now+ms);const q=view.projectiles.find(q=>q.id===4);samples.push({ms,x:q?.x,tick:view.tick});}
  p.dispose(); check(new Set(samples.map(v=>v.x)).size===samples.length, "visible projectile stuck on 20 Hz integers: "+JSON.stringify(samples));
  check(samples.every((v,i)=>i===0||v.x>samples[i-1].x),"motion went backwards");
  return {samples};
});
await run("fractional rendering never consumes future impact",async()=>{
  const p=await predictor("day"), now=performance.now(), first=p.sample(now), next=p.sample(now+35);
  check(next.zombies.some(z=>z.id===2&&z.hp===1),"fractional lookahead killed a zombie");
  check(next.projectiles.some(q=>q.id===3),"fractional lookahead removed impact projectile");
  check(JSON.stringify(first.events)===JSON.stringify(next.events),"prediction emitted audio event");p.dispose();return {};
});
await run("fog authoritative removal applies once with phase preserved",async()=>{
  const p=await predictor("fog"), now=performance.now();p.sample(now+90);
  const next=clone(fixtures.fog);next.tick+=2;next.snapshot.tick+=2;next.zombies=[];next.snapshot.zombies=[];
  p.update(next);const sample=p.sample(now+95);check(sample.zombies.length===0,"authoritative removal ignored");
  const before=sample.projectiles.find(v=>v.id===4).x;p.update(next);const after=p.sample(now+100).projectiles.find(v=>v.id===4).x;
  check(after>=before,"duplicate packet reset motion phase");p.dispose();return {before,after};
});
await run("explicit input reset discards old prediction clock and accepts next edge",async()=>{
  const p=await predictor("fog");p.sample(performance.now()+220);p.reset();
  const accepted=p.apply({type:"plant",epoch:fixtures.fog.epoch,seq:1,kind:"sunflower",row:5,col:0},0);
  p.dispose();check(accepted,"old displayed future tick rejected first post-reset input");return {};
});
await run("valid-invalid-valid preserves edges and reports actual engine error",async()=>{
  const p=await predictor("fog"),epoch=fixtures.fog.epoch;
  check(p.apply({type:"plant",epoch,seq:1,kind:"sunflower",row:5,col:0},0),"first valid edge failed");
  check(!p.apply({type:"plant",epoch,seq:2,kind:"wallnut",row:5,col:0},0),"occupied cell unexpectedly accepted");
  const error=p.lastError;
  check(p.apply({type:"plant",epoch,seq:2,kind:"wallnut",row:5,col:1},0),"valid edge after local rejection failed");
  const plants=p.sample(performance.now()).plants.filter(v=>v.row===5);p.dispose();
  check(error==="这个格子已有植物","missing actual engine error: "+error);check(plants.length===2,"invalid action erased earlier pending edge");return {error,plants:plants.map(v=>v.kind)};
});
await run("server rejects one pending sequence without clearing later valid edges",async()=>{
  const p=await predictor("fog"),epoch=fixtures.fog.epoch;
  for(const [seq,kind,col] of [[1,"sunflower",0],[2,"wallnut",1],[3,"peashooter",2]])check(p.apply({type:"plant",epoch,seq,kind,row:5,col},0),"valid sequence failed "+seq);
  check(typeof p.reject==="function","per-sequence reject API missing");p.reject(2);
  const state=p.sample(performance.now()),plants=state.plants.filter(v=>v.row===5);p.dispose();
  check(plants.map(v=>v.kind).join(",")==="sunflower,peashooter","reject removed unrelated accepted predictions: "+JSON.stringify(plants));
  check(state.players[0].money===850,"reject refunded the wrong predicted wallet");return {plants:plants.map(v=>v.kind),money:state.players[0].money};
});
await run("compact local step omits snapshots and checksums",async()=>{
  const call=q=>JSON.parse(window.pvzWasm(JSON.stringify(q)));
  check(call({op:"load",id:"compact-check",snapshot:fixtures.day.snapshot}).ok,"fixture load failed");
  const response=call({op:"step",id:"compact-check",ticks:1,compact:true});call({op:"drop",id:"compact-check"});
  check(response.ok&&response.view&&!response.state&&!response.digest,"local compact response includes full state/checksum or is unsupported");
  check(!("randomState" in response.view)&&!("events" in response.view),"compact view leaked replay bookkeeping/events");
  return {bytes:JSON.stringify(response).length};
});
await run("replacement snapshot clears omitted zero motion and optional states",async()=>{
  const call=q=>JSON.parse(window.pvzWasm(JSON.stringify(q))),before=clone(fixtures.fog.snapshot),after=clone(fixtures.day.snapshot);
  delete after.projectiles[1].motionX;delete after.zombies[0].state;
  check(call({op:"load",id:"replace-check",snapshot:before}).ok,"first load failed");
  const reply=call({op:"load",id:"replace-check",snapshot:after});call({op:"drop",id:"replace-check"});
  check(reply.ok&&!reply.state.projection&&!reply.state.projectiles[1].motionX&&!reply.state.zombies[0].state,"full snapshot retained omitted fields from old Go slice/map: "+JSON.stringify(reply.state));return {};
});
await run("late non-loop plant phase has one stable start tick",async()=>{
  const call=q=>JSON.parse(window.pvzWasm(JSON.stringify(q))),s=clone(fixtures.day.snapshot);
  Object.assign(s.plants[0],{kind:"squash",hp:300,maxHP:300,state:"ready",bornAt:1});s.projectiles=[];
  check(call({op:"load",id:"phase-check",snapshot:s}).ok,"phase fixture invalid");
  const first=call({op:"step",id:"phase-check",ticks:1}),second=call({op:"step",id:"phase-check",ticks:1});call({op:"drop",id:"phase-check"});
  const a=first.state?.plants[0],b=second.state?.plants[0];
  check(a?.state==="jumping"&&a.flags.phaseAt===1001&&b?.flags.phaseAt===1001,"late phase missing/stuttering origin "+JSON.stringify([a,b]));return {phaseAt:a.flags.phaseAt};
});

const live=await predictor("fog"), canvas=document.querySelector("canvas"), ctx=canvas.getContext("2d");
let inputAt=0, painted=false;const frames=[];
document.querySelector("button").addEventListener("pointerdown",()=>{
  inputAt=performance.now();
  const epoch=fixtures.fog.epoch;
  report.actions=[live.apply({type:"plant",epoch,seq:1,kind:"sunflower",row:5,col:0},0),live.apply({type:"plant",epoch,seq:2,kind:"wallnut",row:5,col:1},0)];
});
function draw(now){
  const s=live.sample(now);ctx.fillStyle="#18382a";ctx.fillRect(0,0,900,600);
  for(const p of s.plants){ctx.fillStyle="#e2be45";ctx.fillRect(p.col*80+20,p.row*80+20,42,42);}
  for(const p of s.projectiles){ctx.fillStyle="#9de667";ctx.fillRect(p.x/12,p.y/12,8,8);}
  frames.push({at:now,x:s.projectiles.find(v=>v.id===4)?.x});
  if(inputAt&&!painted){painted=true;const pixelAt=performance.now();const got=s.plants.filter(p=>p.row===5);report.cases.push({name:"native pointer to local purchase pixels, ordered rapid edges",passed:got.length===2&&report.actions.every(Boolean)&&pixelAt-inputAt<100,inputMS:pixelAt-inputAt,plants:got.map(p=>p.kind),actions:report.actions});
    report.rafFrames=frames;report.passed=report.cases.every(v=>v.passed);report.done=true;live.dispose();document.querySelector("output").textContent=JSON.stringify(report,null,2);return;}
  requestAnimationFrame(draw);
}
requestAnimationFrame(draw);report.ready=true;
