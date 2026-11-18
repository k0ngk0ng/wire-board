// Test first. Isolated failures: original coop magic spends HP/ends selection
// before approval; injured characters whose command is skipped are omitted;
// pending input or time advances the battle; cancel commits/costs resources;
// save loses the suspended commit call; approved multiplayer changes original
// single-player damage, HP cost, RNG, clock, rewards or combat result.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import {create,root} from "./native-vm.mjs";

function battle(v){return Array.from(new Int32Array(v.e.memory.buffer,v.e.wb_battle_info(),87));}
function info(v){
  assert.equal(typeof v.e.wb_coop_info,"function","missing original cooperative action boundary");
  return Array.from(new Int32Array(v.e.memory.buffer,v.e.wb_coop_info(),8));
}
function view(v){return Array.from(new Uint32Array(v.e.memory.buffer,v.e.wb_view(),32));}
async function opening(seats){
  const v=await create();v.e.wb_platform_players(seats);
  let ev=v.step("wb_run_campaign");
  for(let i=0;battle(v)[3]!==1;i++){
    assert.ok(i<512,"original first role not available");
    ev=v.step("wb_run_campaign");
  }
  for(let i=0;battle(v)[6]!==2;i++){
    assert.ok(i<16,"original coop icon not selectable");
    ev=v.step("wb_run_campaign",0,0,0,3);
  }
  ev=v.step("wb_run_campaign",0,0,2);
  for(let i=0;i<16;i++){
    if(ev.kind===6||battle(v)[16+7]===4)return {v,ev};
    ev=v.step("wb_run_campaign");
  }
  throw Error("original cooperative action was not reached");
}

const {v,ev:first}=await opening(3);
assert.equal(first.kind,6,"multiplayer coop committed without consent");
const request=info(v),before=battle(v),clock=view(v)[13];
assert.equal(request[0],1);
assert.equal(request[1],0);
assert.equal(request[4],7,"all three roles lose their separate selection");
assert.equal(request[5],5,"only original healthy roles pay HP; Ling'er stays injured");
assert.ok(request[2]>0&&request[6]>0);
assert.notEqual(before[16+7],4,"cooperative action was already committed");
const saved=v.snapshot(),fresh=await create();fresh.restore(saved);
for(let i=0;i<3;i++){
  const ev=fresh.step("wb_run_campaign",0,0,0x8000,0);
  assert.equal(ev.kind,6,"physical input bypassed cooperative consent");
  assert.deepEqual(info(fresh),request);
  assert.deepEqual(battle(fresh),before);
  assert.equal(view(fresh)[13],clock,"pending consent advanced original clock");
}
fresh.e.wb_release();fresh.e.wb_release();
assert.deepEqual(info(fresh),request,"input release erased cooperative request");
fresh.step("wb_run_campaign",0,2);
assert.equal(info(fresh)[0],0);
assert.equal(battle(fresh)[3],1,"cancel did not return to original action selection");
assert.deepEqual(battle(fresh).slice(16,52),before.slice(16,52),"cancel consumed or committed a character action");
assert.equal(view(fresh)[12],view(v)[12],"cancel changed combat RNG");

const original=await opening(1);
assert.notEqual(original.ev.kind,6,"solo unnecessarily waits for other platform seats");
const resumed=await create();resumed.restore(saved);
const invalid=await create();invalid.restore(saved);
assert.throws(()=>invalid.step("wb_run_campaign",0,3),/invalid cooperative magic response/);
let ev=resumed.step("wb_run_campaign",0,1),solo=original.ev;
let steps=0;
for(;ev.kind!==3;steps++){
  assert.ok(steps<2048,"original cooperative battle did not finish");
  assert.deepEqual(battle(resumed),battle(original.v),"saved/approved coop changed original battle state");
  assert.deepEqual(view(resumed),view(original.v),"saved/approved coop changed original clock/RNG/rewards");
  ev=resumed.step("wb_run_campaign");
  solo=original.v.step("wb_run_campaign");
  assert.equal(ev.kind,solo.kind);
}
assert.equal(ev.value,3);
assert.equal(view(resumed)[11],3);
assert.deepEqual(battle(resumed),battle(original.v));
const report={passed:true,scope:"isolated original cooperative magic, all affected roles/healthy HP contributors, pre-commit consent, cancel, saved pending native continuation, exact single-player battle equivalence; not full campaign",request,steps,final:battle(resumed),cash:view(resumed)[11]};
await fs.writeFile(root+".local/pal/verification/native-coop-js.json",JSON.stringify(report,null,2)+"\n");
console.log("PASS original cooperative magic consent / cancel / single-player equivalence");
