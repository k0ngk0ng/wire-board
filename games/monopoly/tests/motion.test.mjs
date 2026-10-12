// Failure cases before presentation logic: forward wrap becomes backwards;
// card back-three becomes a full lap; jail travels through intervening spaces;
// invalid/oversized event data hangs the animation; corners leave the board.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { motionPath, squarePoint } from '../web/motion.ts';
test('follows public direction across GO and for backwards cards',()=>{
 assert.deepEqual(motionPath({kind:'move',from:38,to:1,steps:3}),[39,0,1]);
 assert.deepEqual(motionPath({kind:'move',from:7,to:4,steps:-3}),[6,5,4]);
 assert.deepEqual(motionPath({kind:'move',from:0,to:0,steps:40}).slice(-2),[39,0]);
});
test('jail is a jump and malformed motion cannot invent a route',()=>{
 assert.deepEqual(motionPath({kind:'jump',from:30,to:10,steps:0}),[10]);
 assert.deepEqual(motionPath({kind:'move',from:2,to:15,steps:3}),[]);
 assert.deepEqual(motionPath({kind:'move',from:2,to:15,steps:10000}),[]);
});
test('all square centers stay inside their perimeter cells',()=>{
 const points=Array.from({length:40},(_,i)=>squarePoint(i));
 assert.equal(new Set(points.map(p=>JSON.stringify(p))).size,40);
 assert.ok(points.every(p=>p.x>0&&p.x<100&&p.y>0&&p.y<100));
 assert.ok(points[0].x>90&&points[0].y>90);
 assert.ok(points[10].x<10&&points[10].y>90);
 assert.ok(points[20].x<10&&points[20].y<10);
 assert.ok(points[30].x>90&&points[30].y<10);
});
