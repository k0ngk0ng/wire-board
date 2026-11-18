import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import { create, root, wasm } from "./native-vm.mjs";

const scope = process.argv[2];
assert.match(scope, /^[a-z][a-z0-9-]{0,39}$/);
assert.equal(process.env.PAL_ORIGINAL_PIXELS, "1");
const nativeBuild = createHash("sha256").update(wasm).digest("hex");
assert.equal(nativeBuild, "d22564b568f6d6b6177c542d71211cb0342151ee57d6d24eafb0e2d865c9c438");
const out = root+`.local/pal/performance/dense-battle-${scope}/`;
const start = JSON.parse(await readFile(out+"start.json","utf8"));
const seed = createHash("sha256").update(`pal/${start.time}/${start.version}/${start.players}`).digest().readUInt32LE(0);
assert.equal(start.players,1);
const vm = await create(seed);
const { e } = vm;
e.wb_platform_players(1);
const battle = () => new Int32Array(e.memory.buffer,e.wb_battle_info(),87);
const step = (keys = 0) => vm.step("wb_run_campaign",0,0,keys,4);
let ev = step();
for (let i = 0; battle()[3] !== 1; i++) {
  assert.ok(i < 32);
  ev = step();
}
assert.equal(battle()[53],30);
step(32);
assert.equal(battle()[6],2);
ev = step(2);
assert.equal(battle()[3],2);
const frames = [];
for (let i = 0; i < 1024; i++) {
  const state = new Uint32Array(e.memory.buffer,e.wb_view(),32);
  const b = battle();
  const p = e.wb_raster_graph();
  const head = new Uint32Array(e.memory.buffer,p,3);
  const graph = new Int32Array(e.memory.buffer,p+12,head[1]*16);
  const effects = [];
  for (let j=0;j<graph.length;j+=16) {
    if (graph[j]===1 && graph[j+5]===8)
      effects.push([graph[j+6],graph[j+7],graph[j+8],graph[j+9]]);
  }
  const colors = new Uint32Array(e.memory.buffer,e.wb_colors(),256);
  const indices = new Uint8Array(e.memory.buffer,(e.wb_presentation_pixels ?? e.wb_pixels)(),64000);
  const regions = [[0,0,320,200],[40,24,125,145],[160,36,304,150],[15,4,305,24],[120,163,319,197]];
  const hashes = regions.map(([left,top,right,bottom]) => {
    let hash = 2166136261;
    for (let y=top;y<bottom;y++)
      for (let x=left;x<right;x++)
        hash = Math.imul(hash ^ (colors[indices[y*320+x]] | 0xff000000),16777619);
    return hash>>>0;
  });
  frames.push({i,clock:state[13],waitMS:ev.value,kind:ev.kind,hash:hashes[0],regions:hashes,nodes:head[1],effects,phase:b[2],menuState:b[3],enemy:b[52],enemyHP:b[53],partyHP:[b[17],b[29],b[41]],cash:state[11]});
  if (state[11]===3 || ev.kind===3) break;
  ev = step();
}
assert.equal(frames.at(-1).cash,3);
const fire = frames.filter(f => f.effects.length);
assert.deepEqual(fire.map(f=>f.effects[0].slice(0,2)),[0,1,2,3,3,4,5,6].map(f=>[50,f]));
assert.equal(fire[0].enemyHP,30);
assert.notEqual(fire.at(-1).enemyHP,30);
assert.ok(frames.filter(f=>f.phase===1 && f.waitMS===40).length>48);
await writeFile(out+"original.json",JSON.stringify({nativeBuild,seed,start,scope:"original three-role cooperative magic, same fixture and seed as authenticated browser",frames},null,2)+"\n",{flag:"wx"});
