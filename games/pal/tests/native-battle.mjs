// Test first. Isolated original battle failures: fabricated party/enemy stats;
// battle steps coalesced to the field's 100 ms instead of 40 ms; observer calls
// corrupt suspended C locals; saved target/action/RNG diverges in a fresh VM;
// releasing input advances combat; an original shortcut submits another role's
// command; root cancel undoes a teammate; battle rewards are applied twice.
// This starts PAL_StartBattle directly, not a completed campaign checkpoint.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import {create, root} from "./native-vm.mjs";

function battle(vm) {
  assert.equal(typeof vm.e.wb_battle_info, "function", "missing real battle observation");
  const p=vm.e.wb_battle_info();
  const raw=Array.from(new Int32Array(vm.e.memory.buffer,p,87));
  return {
    active:!!raw[0], result:raw[1], phase:raw[2], ui:raw[3], menu:raw[4],
    current:raw[5], selected:raw[6], action:raw[7], object:raw[8], target:raw[9],
    auto:!!raw[10], repeat:!!raw[11], force:!!raw[12], flee:!!raw[13],
    players:Array.from({length:raw[14]},(_,i)=>raw.slice(16+i*12,28+i*12)),
    enemies:Array.from({length:raw[15]},(_,i)=>raw.slice(52+i*7,59+i*7)),
  };
}
const seats=Number(process.env.PAL_TEST_PLAYERS??3);
const vm=await create();
assert.equal(typeof vm.e.wb_platform_players,"function");
vm.e.wb_platform_players(seats);
let ev=vm.step("wb_run_campaign");
const step=(key=0,dir=4)=>ev=vm.step("wb_run_campaign",0,0,key,dir);
function until(test) {
  for(let i=0;i<512;i++) {
    const b=battle(vm);
    if(test(b))return b;
    assert.notEqual(ev.kind,3,"battle ended before expected checkpoint");
    step();
  }
  throw Error("battle checkpoint unavailable: "+JSON.stringify(battle(vm)));
}
const initial=until(b=>b.active&&b.ui===1&&b.current===0);
assert.deepEqual(initial.players.map(p=>[...p.slice(0,4),p[5]]),[
  [0,150,150,100,1],[1,28,240,10,5],[2,220,220,80,3],
],"fixture must retain the original resource stats, including Ling'er's low HP");
assert.deepEqual(initial.enemies.map(e=>e.slice(0,2)),[[399,30]]);
assert.equal(ev.kind,4);
assert.equal(ev.value,40,"battle selection must retain the original 25 fps clock");
const firstCommand=vm.snapshot();
function drive(v,test,key=0,dir=4) {
  for(let i=0;i<16;i++) {
    v.step("wb_run_campaign",0,0,i===0?key:0,dir);
    if(test(battle(v)))return;
  }
  throw Error("original battle input did not reach expected state: "+JSON.stringify(battle(v)));
}
if(seats>1) {
  step(0x200); // original auto shortcut must not batch-submit other owners
  for(let i=0;i<3;i++)step();
  assert.equal(battle(vm).auto,false);
  assert.equal(battle(vm).current,0);
  for(const key of [0x100,0x8000,0x2000]) {
    const shortcut=await create();shortcut.restore(firstCommand);
    shortcut.step("wb_run_campaign",0,0,key);
    for(let i=0;i<4;i++)shortcut.step("wb_run_campaign");
    const b=battle(shortcut);
    assert.equal(b.repeat||b.force||b.flee||b.auto,false,"shortcut retained a whole-party command");
    assert.equal(b.players[0][6],2,"shortcut did not commit the selected original role");
    assert.notEqual(b.players[1][6],2,"shortcut submitted the next role's action");
    assert.equal(b.current,1);
  }
  const misc=await create();misc.restore(firstCommand);
  drive(misc,b=>b.selected===3,0,0); // original South -> miscellaneous icon
  drive(misc,b=>b.menu===4,2);
  assert.equal(battle(misc).menu,4);
  misc.step("wb_run_campaign",0,0,2);
  for(let i=0;i<3;i++)misc.step("wb_run_campaign");
  assert.equal(battle(misc).auto,false,"miscellaneous auto bypassed platform control");
} else {
  const auto=await create();auto.restore(firstCommand);
  drive(auto,b=>b.auto,0x200);
  assert.equal(battle(auto).auto,true,"single-player original auto shortcut was lost");
}
// Defend is a real command; the next original character still needs a command.
step(0x400);
const second=until(b=>b.ui===1&&b.current===1);
assert.equal(second.players[0][7],1);
if(seats>1) {
  step(1);
  for(let i=0;i<3;i++)step();
  assert.equal(battle(vm).current,1,"root cancel crossed role ownership");
  assert.equal(battle(vm).players[0][6],2,"teammate's committed action was undone");
}
const saved=vm.snapshot(),fresh=await create();
fresh.restore(saved);
if(seats===1) {
  const cancel=await create();cancel.restore(saved);
  drive(cancel,b=>b.players[0][6]!==2,1);
  assert.notEqual(battle(cancel).players[0][6],2,"single-player original backtracking was lost");
}
// Observers and release run on the service stack without advancing the clock.
const before=battle(fresh),stateBefore=Array.from(new Uint32Array(fresh.e.memory.buffer,fresh.e.wb_view(),32));
fresh.e.wb_release();fresh.e.wb_release();
assert.deepEqual(battle(fresh),before);
assert.equal(new Uint32Array(fresh.e.memory.buffer,fresh.e.wb_view(),32)[13],stateBefore[13]);
let replayed=0;
for(let i=0;i<1024&&ev.kind!==3;i++) {
  const b=battle(vm);
  const key=b.active&&b.ui!==0?2:0;
  const a=step(key),z=fresh.step("wb_run_campaign",0,0,key);
  assert.deepEqual(a,z,"fresh-instance battle event/overlay diverged");
  assert.deepEqual(battle(vm),battle(fresh),"fresh-instance battle target/stats diverged");
  const va=new Uint32Array(vm.e.memory.buffer,vm.e.wb_view(),32),vz=new Uint32Array(fresh.e.memory.buffer,fresh.e.wb_view(),32);
  assert.deepEqual(Array.from(va),Array.from(vz),"battle RNG/clock/rewards diverged");
  replayed++;
}
assert.equal(ev.kind,3,"actual original battle did not finish");
assert.equal(ev.value,3,"expected original PAL_StartBattle victory");
const final=battle(vm),view=Array.from(new Uint32Array(vm.e.memory.buffer,vm.e.wb_view(),32));
assert.equal(final.active,false);
assert.equal(view[11],3,"actual original lantern reward is three cash");
const report={passed:true,scope:"isolated original PAL_StartBattle, resource stats, 40 ms battle clock, actual commands and rewards, fresh-instance saved continuation; not campaign completion",seats,initial,final,replayed,cash:view[11]};
await fs.writeFile(root+`.local/pal/verification/native-battle-${seats}.json`,JSON.stringify(report,null,2)+"\n");
console.log("PASS original battle / saved continuation / platform seats "+seats);
