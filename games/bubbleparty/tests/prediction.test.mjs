// Failure inventory: docs/research/realtime-efficiency/MOTION.md.
import test from 'node:test';
import assert from 'node:assert/strict';
import { predictPlayer, PlayerMotion } from '../web/prediction.ts';
const cells=()=>Array(15*13).fill(0);
const player=()=>({seat:0,x:2,y:3,px:2,py:3,seq:0,alive:true,trappedUntil:0,speedMs:150});
const view=()=>({rulesVersion:2,serverAt:1000,phase:'playing',players:[player()],cells:cells()});
const close=(a,b)=>assert.ok(Math.abs(a-b)<1e-6,`${a} != ${b}`);
test('movement preview is immediate, bounded and does not change authoritative state',()=>{
 const p=player(),grid=cells(),before=structuredClone({p,grid});
 const at=predictPlayer({x:2,y:3},p,'right',grid,60);
 close(at.x,2.4);close(at.y,3);
 const bounded=predictPlayer({x:2,y:3},p,'right',grid,10000);
 close(bounded.x,2+250/150);
 assert.deepEqual({p,grid},before);
 assert.deepEqual(predictPlayer(at,p,'none',grid,60),at);
});
test('walls, crates and outer borders stop the preview body; turns spend distance aligning first',()=>{
 for(const solid of [1,2]){const grid=cells();grid[3*15+3]=solid;close(predictPlayer({x:2,y:3},player(),'right',grid,200).x,2.22)}
 const turned=predictPlayer({x:2.3,y:3},player(),'down',cells(),60);
 close(turned.x,2);close(turned.y,3.1);
 assert.ok(predictPlayer({x:0,y:3},player(),'left',cells(),250).x>=-0.22);
});
test('a player can leave its balloon but cannot re-enter through its collision body',()=>{
 const grid=cells();grid[3*15+2]=3;
 const left=predictPlayer({x:2,y:3},player(),'left',grid,120);
 close(left.x,1.2);
 close(predictPlayer(left,player(),'right',grid,120).x,1.22);
});
test('dead and trapped players cannot move; preview never changes bomb or result data',()=>{
 for(const p of [{...player(),alive:false},{...player(),trappedUntil:2000}])assert.deepEqual(predictPlayer({x:2,y:3},p,'right',cells(),80),{x:2,y:3});
});
test('local motion advances before snapshots, stops after 250ms, and resets across transport/control changes',()=>{
 const m=new PlayerMotion(),v=view(),before=structuredClone(v);
 m.frame(v,0,'right',0,'room:transport1',true);
 const moved=m.frame(v,0,'right',60,'room:transport1',true);assert.ok(moved.x>2);
 const stopped=m.frame(v,0,'none',80,'room:transport1',true);close(stopped.x,moved.x);
 m.frame(v,0,'right',250,'room:transport1',true);
 const frozen=m.frame(v,0,'right',1000,'room:transport1',true);
 assert.deepEqual(m.frame(v,0,'right',2000,'room:transport1',true),frozen);
 assert.deepEqual(m.frame(v,0,'none',2100,'room:transport2',true),{x:2,y:3});
 assert.deepEqual(m.frame(v,0,'right',2200,'room:transport2',false),{x:2,y:3});
 assert.deepEqual(v,before);
});
test('authoritative large correction, trap and new wall override a stale preview',()=>{
 const m=new PlayerMotion(),v=view();m.frame(v,0,'right',0,'s',true);m.frame(v,0,'right',100,'s',true);
 v.serverAt+=100;v.players[0].px=8;v.players[0].x=8;
 close(m.frame(v,0,'none',110,'s',true).x,8);
 v.serverAt+=100;v.players[0].trappedUntil=5000;
 assert.deepEqual(m.frame(v,0,'right',120,'s',true),{x:8,y:3});
 v.players[0].trappedUntil=0;v.cells[3*15+9]=1;
 assert.ok(m.frame(v,0,'right',160,'s',true).x<=8.22);
});
test('maximum speed pickup retains the server 70ms tile speed',()=>{
 close(predictPlayer({x:2,y:3},{...player(),speedMs:70},'right',cells(),70).x,3);
});

test('legacy integer-step matches never receive a continuous local preview',()=>{
 const m=new PlayerMotion(),v={...view(),rulesVersion:0};
 m.frame(v,0,'right',0,'legacy',true);
 assert.deepEqual(m.frame(v,0,'right',100,'legacy',true),{x:2,y:3});
});

// v3 integration failures were listed before implementation in
// docs/research/bubble-classic-maps/BATTLEFIELD-SCENARIOS.md.
test('v3 push crates are solid and only airships pass crates, never walls or balloons',()=>{
 const grid=cells();grid[3*15+3]=4;
 close(predictPlayer({x:2,y:3},player(),'right',grid,200).x,2.22);
 for(const mount of ['airship','airship-broken']){
  assert.ok(predictPlayer({x:2,y:3},{...player(),mount},'right',grid,200).x>3);
  for(const obstacle of [1,3]){grid[3*15+3]=obstacle;close(predictPlayer({x:2,y:3},{...player(),mount},'right',grid,200).x,2.22)}
  grid[3*15+3]=4;
 }
});
test('v3 local motion uses server speed and resets when mount changes',()=>{
 const m=new PlayerMotion(),v={...view(),rulesVersion:3};v.players[0].speedMs=300;
 m.frame(v,0,'right',0,'s',true);close(m.frame(v,0,'right',30,'s',true).x,2.1);
 v.players[0].mount='turtle';v.players[0].speedMs=600;
 close(m.frame(v,0,'right',60,'s',true).x,2);
 close(m.frame(v,0,'right',90,'s',true).x,2.05);
});
test('native conveyors and directional gates use authoritative movement without invented advances',async()=>{
 const catalog=(await import('../data/classic-2002.json',{with:{type:'json'}})).default;
 const tiles=new Map(catalog.tiles.map(t=>[t.id,t]));
 for(const kind of ['belt','gate']){
  const map=catalog.maps.find(m=>(kind==='belt'?m.ground:m.objects).flat().some(id=>kind==='belt'?tiles.get(id)?.tileInfo.fieldsRaw[3]===2:tiles.get(id)?.category==='passable'&&tiles.get(id).tileInfo.fieldsRaw.slice(1,5).some(x=>!x)));
  const layer=kind==='belt'?map.ground:map.objects;let spot;
  layer.forEach((row,y)=>row.forEach((id,x)=>{const t=tiles.get(id);if(!spot&&(kind==='belt'?t?.category==='ground'&&t.tileInfo.fieldsRaw[3]===2:t?.category==='passable'&&t.tileInfo.fieldsRaw.slice(1,5).some(a=>!a)))spot={x,y}}));
  const v={...view(),rulesVersion:3,mapId:map.id};v.players[0]={...player(),...spot,px:spot.x,py:spot.y};
  const m=new PlayerMotion();m.frame(v,0,'right',0,'s',true);
  assert.deepEqual(m.frame(v,0,'right',30,'s',true),spot);
  v.serverAt+=100;v.players[0].px+=.2;
  const next=m.frame(v,0,'right',100,'s',true);assert.ok(next.x>=spot.x&&next.x<=spot.x+.2);
 }
});
