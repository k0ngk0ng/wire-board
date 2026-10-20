// Failure inventory: docs/research/realtime-efficiency/MOTION.md.
import test from 'node:test';
import assert from 'node:assert/strict';
import { predictTank, TankMotion } from '../web/prediction.ts';
const tank=()=>({seat:0,team:0,class:'balanced',x:32,y:32,dir:1,alive:true,hp:1,freezeUntil:0,respawnAt:0});
const world=()=>({cells:Array(13).fill('0000000000000'),broken:new Uint8Array(26*26),shovelUntil:[0,0],tanks:[tank()],speed:44});
const view=()=>({phase:'playing',serverAt:1000,tanks:[tank()]});
const pose=t=>({x:t.x,y:t.y,dir:t.dir});
test('tank preview advances locally, caps prediction and never mutates authoritative armor or projectiles',()=>{
 const t=tank(),w=world(),before=structuredClone({t,w});
 const p=predictTank(pose(t),t,'right',w,100,1000);assert.ok(p.x>36&&p.x<37);assert.equal(p.y,32);
 assert.ok(predictTank(pose(t),t,'right',w,10000,1000).x<=43.00001);
 assert.deepEqual({t,w},before);
 assert.deepEqual(predictTank(p,t,'none',w,100,1000),p);
});
test('intact terrain, water, base, other tanks and boundaries block movement',()=>{
 for(const code of ['1','2','3']) {const w=world();w.cells[2]='000'+code+'000000000';assert.equal(predictTank(pose(tank()),tank(),'right',w,250,1000).x,32)}
 const w=world();w.tanks.push({...tank(),seat:1,x:50});assert.ok(predictTank(pose(tank()),tank(),'right',w,250,1000).x<=34);
 assert.equal(predictTank({x:80,y:0,dir:1},tank(),'right',world(),250,1000).x,80);
 assert.equal(predictTank({x:192,y:32,dir:1},tank(),'right',world(),250,1000).x,192);
});
test('destroyed subcells open a path, live shovel steel closes it, and turns align to 8px grid',()=>{
 const w=world();w.cells[2]='0001000000000';
 for(const y of [4,5])for(const x of [6,7])w.broken[y*26+x]=1;
 assert.ok(predictTank(pose(tank()),tank(),'right',w,100,1000).x>32);
 const turned=predictTank({x:35,y:32,dir:1},tank(),'down',world(),60,1000);
 assert.equal(turned.x,32);assert.equal(turned.dir,2);assert.ok(turned.y>32);
 const shovel=world();shovel.shovelUntil=[2000,0];
 assert.equal(predictTank({x:64,y:0,dir:1},tank(),'right',shovel,250,1000).x,64);
});
test('dead and frozen tanks stay authoritative; no frozen timer is locally expired',()=>{
 for(const t of [{...tank(),alive:false},{...tank(),hp:0},{...tank(),freezeUntil:2000}])assert.deepEqual(predictTank(pose(t),t,'right',world(),80,1000),pose(t));
});
test('frame preview freezes without packets, clears on transport changes and snaps to respawn',()=>{
 const m=new TankMotion(),v=view(),w=world();m.frame(v,0,'right',w,0,'a',true);
 assert.ok(m.frame(v,0,'right',w,60,'a',true).x>32);
 const frozen=m.frame(v,0,'right',w,1000,'a',true);
 assert.deepEqual(m.frame(v,0,'right',w,2000,'a',true),frozen);
 assert.deepEqual(m.frame(v,0,'none',w,2100,'b',true),pose(tank()));
 v.serverAt+=100;v.tanks[0].x=100;
 assert.equal(m.frame(v,0,'none',w,2110,'b',true).x,100);
 v.tanks[0].freezeUntil=5000;
 assert.equal(m.frame(v,0,'right',w,2200,'b',true).x,100);
});
