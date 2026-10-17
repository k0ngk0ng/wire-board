// Runs the browser's real transport against the actual authenticated Go host.
// HTTP delivery/ack delays are explicit; this is not a browser-rendering test.
import assert from 'node:assert/strict';
import { BasketballNetworkInput } from '../../games/basketball/web/network-input.ts';

const base=process.env.BASKETBALL_HTTP_URL;
if(!base || new URL(base).hostname!=='127.0.0.1')throw Error('Owned local HTTP fixture required');
const trace=[],checks=[];
let cookie='',serial=0,room,transport;
const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function request(path,body) {
  const response=await fetch(base+path,{method:body?'POST':'GET',headers:{'Content-Type':'application/json',Cookie:cookie},body:body?JSON.stringify(body):undefined});
  const cookies=response.headers.getSetCookie();if(cookies.length)cookie=cookies.map(value=>value.split(';')[0]).join('; ');
  const value=await response.json();
  if(!response.ok)throw Error(`HTTP ${response.status}: ${JSON.stringify(value)}`);
  return value;
}
function deferred(){let resolve;const promise=new Promise(done=>resolve=done);return{promise,resolve};}
async function poll() {
  room=(await request('/api/state')).room;
  const v=room.game.data;
  transport?.update({roomID:room.id,userID:room.seats[room.you].id,seat:room.you,epoch:v.epoch,serverAt:v.serverAt,sequence:v.inputs[room.you].sequence,canInput:v.canInput,version:room.version});
  return v;
}
async function until(check) {
  for(let n=0;n<60;n++){const v=await poll();if(check(v))return v;await delay(20);}
  throw Error('Timed out waiting for actual HTTP state');
}
function check(name,condition){assert.ok(condition,name);checks.push(name);}
let deliveryGate,ackGate;
const active=new Set();
try {
  await request('/api/login',{name:'牌局管理员',password:'poker-test-password'});
  await poll();
  transport=new BasketballNetworkInput({onReset:reason=>trace.push({reset:reason}),send:action=>{
    // Capture the bound room and version at dispatch, before any delay.
    const endpoint='/api/rooms/'+room.id;
    const body={type:'action',nonce:'network-'+(++serial),version:room.version,action};
    const before=deliveryGate;deliveryGate=undefined;
    const after=ackGate;ackGate=undefined;
    const sent=(async()=>{
      if(before)await before.promise;
      await request(endpoint,body);
      trace.push({accepted:action});
      if(after)await after.promise;
    })();
    active.add(sent);void sent.finally(()=>active.delete(sent)).catch(()=>{});
    return sent;
  }});
  let v=await poll();
  const initial=v.inputs[0].sequence;
  deliveryGate=deferred();const oldDelivery=deliveryGate;
  transport.sample(64);
  await delay(20);transport.sample(0);
  transport.cancel();
  v=await until(v=>v.inputs[0].sequence===initial+3);
  check('priority cancellation reaches real server before held press',v.inputs[0].buttons===0 && v.inputs[0].pending===0);
  const cancelledAt=v.inputs[0].lastPacketAt;
  oldDelivery.resolve();await Promise.all([...active]);
  v=await poll();
  check('delayed old HTTP press is an idempotent no-op',v.inputs[0].sequence===initial+3 && v.inputs[0].lastPacketAt===cancelledAt && v.inputs[0].buttons===0);

  ackGate=deferred();const oldAck=ackGate;
  transport.sample(1);
  v=await until(v=>v.inputs[0].sequence===initial+4);
  check('server snapshot acknowledges input while HTTP response is stalled',transport.pendingCount===0 && v.inputs[0].buttons===1);
  transport.sample(0);
  v=await until(v=>v.inputs[0].sequence===initial+5);
  oldAck.resolve();await Promise.all([...active]);
  check('new release is not blocked by old response',v.inputs[0].buttons===0);

  deliveryGate=deferred();const expiredDelivery=deliveryGate;
  transport.sample(64);
  for(let n=0;n<8;n++){await delay(80);await poll();transport.sample(0);}
  v=await until(v=>v.inputs[0].buttons===0 && transport.pendingCount===0);
  check('expired gestures are cancelled rather than replayed',trace.some(event=>event.reset==='timeout'));
  const afterTimeout=v.inputs[0].sequence;
  expiredDelivery.resolve();await Promise.all([...active]);v=await poll();
  check('delivery after timeout cannot revive a press',v.inputs[0].sequence===afterTimeout && v.inputs[0].buttons===0);
  transport.sample(65);
  v=await until(v=>v.inputs[0].buttons===65 && v.runtime.held[0]===65 && transport.pendingCount===0);
  transport.cancel(64);
  v=await until(v=>v.inputs[0].buttons===1 && transport.pendingCount===0);
  check('partial device cancellation keeps the independently held direction',v.runtime.held[0]!==0 && (v.runtime.held[0]&64)===0);
  const afterPartial=v.inputs[0].sequence;
  transport.sample(1);await delay(20);v=await poll();
  check('remaining physical source does not generate a false second press',v.inputs[0].sequence===afterPartial);
  transport.cancel();await until(v=>v.inputs[0].buttons===0 && transport.pendingCount===0);
  check('other seats retain separate input sequences',v.inputs.slice(1).every(input=>input.sequence===0));
  process.stdout.write(JSON.stringify({status:'passed',checks,trace,scope:'actual JS transport + real authenticated HTTP server; no browser rendering or full-match claim'})+'\n');
} catch(error) {
  process.stdout.write(JSON.stringify({status:'failed',error:String(error),checks,trace})+'\n');
  process.exitCode=1;
} finally {
  transport?.dispose();
  // An outstanding intentionally stalled request is a failed test, not a
  // reason to keep the fixture alive indefinitely.
  await Promise.race([Promise.allSettled([...active]),delay(250)]);
}
