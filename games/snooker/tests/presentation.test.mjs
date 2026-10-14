// Authored before the 2D presentation helpers. Failures: rotated mobile input
// doesn't match the canvas, wheel zoom moves the aim point, a ball disappears
// before the pocket event, delayed snapshots replay collision audio.
import test from 'node:test';
import assert from 'node:assert/strict';
import {worldToScreen,screenToWorld,zoomAt,sampleFlight,recentImpacts} from '../web/presentation.ts';
for(const portrait of [false,true])test(`camera inversion and cursor zoom ${portrait}`,()=>{
 const camera={width:portrait?300:1000,height:portrait?530:530,portrait,zoom:1,pan:{x:0,y:0}};
 const p={x:1.42,y:.7},screen=worldToScreen(p,camera),back=screenToWorld(screen,camera);
 assert.ok(Math.abs(back.x-p.x)<1e-10&&Math.abs(back.y-p.y)<1e-10);
 const enlarged=zoomAt(camera,2,screen),same=worldToScreen(p,enlarged);
 assert.ok(Math.abs(same.x-screen.x)<1e-7&&Math.abs(same.y-screen.y)<1e-7);
});
test('pocket disappearance follows the timed event rather than the next frame',()=>{
 const flight={motion:{duration:1,events:[{time:.75,kind:'pot',ball:1}],frames:[{time:0,balls:[{id:1,pos:{x:1,y:.1},down:false}]},{time:1,balls:[{id:1,pos:{x:1,y:0},down:true}]}]}};
 assert.equal(sampleFlight(flight,.74)[0].down,false);assert.equal(sampleFlight(flight,.75)[0].down,true);assert.equal(sampleFlight(flight,2)[0].down,true);
});
test('collision sounds only include fresh crossed events',()=>{
 const events=[{time:.1},{time:.5},{time:.9},{time:1.5}];
 assert.deepEqual(recentImpacts(events,0,.95),[{time:.9}]);
 assert.deepEqual(recentImpacts(events,.95,1),[]);
 assert.deepEqual(recentImpacts(events,1,1.51),[{time:1.5}]);
});
test('sparse frames retain stationary balls and exact moving positions',()=>{
 const flight={motion:{duration:1,events:[],frames:[{time:0,balls:[{id:0,pos:{x:0,y:0},down:false},{id:1,pos:{x:1,y:1},down:false}]},{time:.5,balls:[{id:0,pos:{x:.5,y:0},down:false}]},{time:1,balls:[]}]}};
 const half=sampleFlight(flight,.25);assert.equal(half.length,2);assert.equal(half.find(b=>b.id===0).pos.x,.25);assert.deepEqual(half.find(b=>b.id===1).pos,{x:1,y:1});
 const end=sampleFlight(flight,1);assert.equal(end.find(b=>b.id===0).pos.x,.5);assert.equal(end.length,2);
});
