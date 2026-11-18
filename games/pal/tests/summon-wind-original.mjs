import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import { create, root, wasm } from "./native-vm.mjs";

const scope=process.argv[2];
assert.match(scope,/^summon-[a-z0-9-]+$/);
const side=process.argv[3];
assert.ok(side===undefined||side==="desktop"||side==="phone");
assert.equal(process.env.PAL_ORIGINAL_PIXELS,"1");
const nativeBuild=createHash("sha256").update(wasm).digest("hex");
const directory=root+".local/pal/performance/dense-battle-"+scope+"/"+(side?side+"/":"");
const start=JSON.parse(await readFile(directory+"start.json","utf8"));
assert.equal(start.players,1);
const seed=createHash("sha256").update(`pal/${start.time}/${start.version}/${start.players}`).digest().readUInt32LE(0);
const vm=await create(seed), e=vm.e;
e.wb_platform_players(1);
const battle=()=>Array.from(new Int32Array(e.memory.buffer,e.wb_battle_info(),87));
const step=(keys=0)=>vm.step("wb_run_campaign",0,0,keys,4);
for(let i=0;battle()[3]!==1;i++){
  assert.ok(i<32);
  step();
}
assert.deepEqual(battle().slice(14,21),[1,1,3,810,810,790,790]);
assert.deepEqual(battle().slice(52,54),[399,30]);
for(const key of [8,2,4,32,32])step(key);
const picked=step(2);
assert.equal(battle()[8],315);
const frames=[];
let event=picked;
for(let i=0;i<1200;i++){
  const native=new Uint32Array(e.memory.buffer,e.wb_view(),32), b=battle();
  const p=e.wb_raster_graph(), head=new Uint32Array(e.memory.buffer,p,3);
  const nodes=new Int32Array(e.memory.buffer,p+12,head[1]*16);
  const sprites=[],effects=[];
  for(let k=0;k<nodes.length;k+=16)if(nodes[k]===1){
    const source=nodes[k+5], id=nodes[k+6];
    const draw=Array.from(nodes.slice(k+6,k+10));
    if(source===6&&id===14)sprites.push(draw);
    if(source===8&&id===44)effects.push(draw);
  }
  frames.push({index:i,clock:native[13],kind:event.kind,wait:event.value,phase:b[2],spell:b[8],mp:b[19],enemy:b[52],enemyHP:b[53],cash:native[11],sprites,effects});
  if(native[11]===3||event.kind===3)break;
  event=step();
}
const fire=frames.filter(r=>r.effects.length);
const changes=fire.map(r=>r.effects[0][1]).filter((n,i,a)=>i===0||n!==a[i-1]);
assert.deepEqual(changes,Array.from({length:48},(_,i)=>i%16));
assert.ok(frames.findIndex(r=>r.sprites.length)<frames.findIndex(r=>r.effects.length));
assert.ok(fire.every(r=>r.mp===726&&r.kind===4&&r.wait===40));
assert.ok(fire.slice(0,-1).every(r=>r.enemy===399&&r.enemyHP===30));
assert.notEqual(fire.at(-1).enemyHP,30);
assert.equal(frames.at(-1).cash,3);
await writeFile(directory+"original.json",JSON.stringify({nativeBuild,seed,start,scope:"original single-role wind summon with original pixels and exact original resources",frames},null,2)+"\n",{flag:"wx"});
