// Failure inventory before implementation: rapid taps observe stale React state;
// busy drops the next pair; acknowledgement clears a third tile; same indices
// send twice; stale revisions, tool reshuffles, restore or autoplay replay input.
import test from 'node:test';
import assert from 'node:assert/strict';
import {PairInput} from '../web/input.ts';
const board=()=>({rows:2,cols:3,cells:[1,1,2,2,3,3].map(icon=>({kind:'tile',icon}))});
const view=(extra={})=>({scope:'room:0:round:epoch',round:'round',revision:0,stamp:1,playable:true,board:board(),event:{id:0,type:'',at:0},...extra});
function ack(s,pair,revision){const b=structuredClone(s.board);for(const i of pair)b.cells[i]=null;return {...s,board:b,revision,stamp:s.stamp+1,event:{id:revision,type:'match',pair,at:revision}};}
test('rapid pairs queue while a third selection survives acknowledgements',()=>{
 const input=new PairInput();let s=view();input.sync(s);
 input.click(0);input.click(1);const a=input.next(s,false);assert.deepEqual([a.a,a.b,a.revision],[0,1,0]);
 input.click(2);input.click(3);input.click(4);assert.equal(input.selected,4);assert.equal(input.next(s,true),null);
 input.complete(a.token);assert.equal(input.next(s,false),null,'must observe a newer snapshot before advancing');
 s=ack(s,[0,1],1);input.sync(s);assert.equal(input.selected,4);
 const b=input.next(s,false);assert.deepEqual([b.a,b.b,b.revision],[2,3,1]);
 input.complete(b.token);s=ack(s,[2,3],2);input.sync(s);input.next(s,false);assert.equal(input.selected,4);
 input.click(5);const c=input.next(s,false);assert.deepEqual([c.a,c.b,c.revision],[4,5,2]);
});
test('reserved tiles cannot double-submit and peer snapshots preserve input',()=>{
 const input=new PairInput(),s=view();input.sync(s);input.click(0);input.click(1);input.click(0);input.click(1);
 const a=input.next(s,false);assert.ok(a);input.sync({...s,stamp:2});assert.equal(input.next({...s,stamp:2},false),null);
 assert.equal(input.queued,0);assert.equal(input.selected,null);assert.equal(input.reserved(0),true);
});
test('a rejected match leaves the board authoritative and permits the next pair',()=>{
 const input=new PairInput(),s=view();input.sync(s);input.click(0);input.click(1);const a=input.next(s,false);
 input.click(2);input.click(3);input.complete(a.token);const b=input.next({...s,stamp:2},false);
 assert.deepEqual([b.a,b.b,b.revision],[2,3,0]);assert.equal(s.board.cells.filter(Boolean).length,6);
});
test('reshuffle, other seat control and reconnect drop unsent input',()=>{
 for(const change of [{scope:'new-epoch'}, {playable:false}, {revision:1,stamp:2,event:{id:1,type:'shuffle',at:1}}, {revision:1,stamp:2,event:{id:1,type:'match',pair:[0,1],recovery:true,at:1}}]){
  const input=new PairInput(),s=view();input.sync(s);input.click(0);input.click(1);const a=input.next(s,false);input.click(2);input.click(3);input.click(4);
  input.sync({...s,...change});input.complete(a.token);assert.equal(input.queued,0);assert.equal(input.selected,null);
 }
});
test('invalid, frozen and cancelled selections never create a request',()=>{
 const input=new PairInput(),s=view();s.board.cells[0].ice=true;input.sync(s);input.click(0);assert.equal(input.selected,null);
 input.click(-1);input.click(99);input.click(1);input.click(2);assert.equal(input.selected,2);
 input.cancel();assert.equal(input.selected,null);assert.equal(input.next(s,false),null);
});
