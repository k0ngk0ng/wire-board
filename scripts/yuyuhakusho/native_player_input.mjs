// Dispatch native input only to the research page owned by this acceptance.
const [endpoint, pageURL, input] = process.argv.slice(2);
const socket = new WebSocket(endpoint);
await new Promise((resolve, reject) => {
  socket.addEventListener('open', resolve, {once:true});
  socket.addEventListener('error', reject, {once:true});
});
let seq = 0;
const waiting = new Map();
socket.addEventListener('message', event => {
  const response = JSON.parse(event.data);
  if (!response.id) return;
  const pending = waiting.get(response.id);
  waiting.delete(response.id);
  response.error ? pending.reject(Error(JSON.stringify(response.error))) : pending.resolve(response.result);
});
function call(method, params = {}, sessionId) {
  return new Promise((resolve,reject) => {
    const id = ++seq;
    waiting.set(id,{resolve,reject});
    socket.send(JSON.stringify({id,method,params,...(sessionId?{sessionId}:{})}));
  });
}
const {targetInfos} = await call('Target.getTargets');
const target = targetInfos.find(target => target.type === 'page' && target.url === pageURL);
if (!target) throw Error('Owned player page not found');
const {sessionId} = await call('Target.attachToTarget',{targetId:target.targetId,flatten:true});
await call('Page.bringToFront',{},sessionId);
await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5},sessionId);
const evaluate = expression => call('Runtime.evaluate',{expression,returnByValue:true},sessionId).then(result=>result.result.value);
const observations=[];
for (const action of JSON.parse(input)) {
  const {expect, method='Input.dispatchTouchEvent', ...event} = action;
  if (method === 'acceptance:hide') {
    const other = await call('Target.createTarget',{url:'about:blank',background:false});
    try {
      await call('Target.activateTarget',{targetId:other.targetId});
      let hidden=false;
      for (let n=0;n<50;n++) {
        hidden=await evaluate('document.hidden');
        if(hidden) break;
        await new Promise(resolve=>setTimeout(resolve,20));
      }
      if (!hidden) throw Error('Player page did not become hidden');
      if ((await evaluate('window.yyResearch.sample()')).length) throw Error('Hidden page retained input');
    } finally {
      await call('Target.closeTarget',{targetId:other.targetId});
      await call('Target.activateTarget',{targetId:target.targetId});
    }
  } else {
    await call(method,event,sessionId);
  }
  const held = await evaluate('window.yyResearch.sample()');
  observations.push({event:event.type??method,held});
  if (expect && JSON.stringify(held)!==JSON.stringify(expect)) throw Error(JSON.stringify({held,expect}));
}
console.log(JSON.stringify(observations));
await call('Target.detachFromTarget',{sessionId});
socket.close();
