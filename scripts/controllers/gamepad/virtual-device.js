// Acceptance-only hardware source. Only navigator.getGamepads is substituted;
// UI, HTTP/WebSocket transport, SQLite and game rules stay real.
window.__gamepadDevices = [];
window.__gamepadSamples = [];
Object.defineProperty(navigator, 'getGamepads', { configurable: true, value: () => {
  const at = performance.now();
  window.__gamepadSamples.push(at);
  if (window.__gamepadSamples.length > 1000) window.__gamepadSamples.shift();
  const result = window.__gamepadDevices.map(p => p && ({
    ...p, axes: [...p.axes], timestamp: at, connected: true,
    buttons: p.buttons.map(value => ({value, pressed: value >= .5, touched: value > 0})),
  }));
  const pulse = window.__gamepadPulse;
  if (pulse && --pulse.remaining === 0) {
    const pad = window.__gamepadDevices[pulse.index];
    for (const [key,value] of Object.entries(pulse.buttons)) pad.buttons[+key] = value;
    for (const [key,value] of Object.entries(pulse.axes)) pad.axes[+key] = value;
    window.__gamepadPulse = null;
    pulse.resolve({virtualHardware:true,samples:pulse.frames,started:pulse.started,finished:at});
  }
  return result;
}});
window.__setGamepad = (index, id = 'Acceptance Standard Pad', mapping = 'standard', buttons = {}, axes = {}) => {
  const previous = window.__gamepadDevices[index];
  const pad = previous?.id === id && previous?.mapping === mapping ? previous : {index,id,mapping,buttons:Array(24).fill(0),axes:Array(4).fill(0)};
  for (const [key,value] of Object.entries(buttons)) pad.buttons[+key] = value;
  for (const [key,value] of Object.entries(axes)) pad.axes[+key] = value;
  window.__gamepadDevices[index] = pad;
};
window.__pulseGamepad = (index, buttons, axes, frames) => new Promise(resolve => {
  if (window.__gamepadPulse) throw Error('A virtual hardware pulse is already active');
  const pad = window.__gamepadDevices[index]; if (!pad || frames < 1 || frames > 120) throw Error('Invalid virtual hardware pulse');
  const restoreButtons=Object.fromEntries(Object.keys(buttons).map(k=>[k,pad.buttons[+k]]));
  const restoreAxes=Object.fromEntries(Object.keys(axes).map(k=>[k,pad.axes[+k]]));
  window.__setGamepad(index,pad.id,pad.mapping,buttons,axes);
  window.__gamepadPulse={index,buttons:restoreButtons,axes:restoreAxes,frames,remaining:frames,started:performance.now(),resolve};
});
window.__disconnectGamepad = index => { window.__gamepadDevices[index] = null; window.dispatchEvent(new Event('gamepaddisconnected')); };
// Observe real CDP keyboard input before game listeners can consume it.
window.__nativeKeyEvents = [];
window.addEventListener('keydown', e => {
  window.__nativeKeyEvents.push({code:e.code,at:performance.now(),trusted:e.isTrusted});
  if(window.__nativeKeyEvents.length>100)window.__nativeKeyEvents.shift();
}, true);
// Optional transport latency for prediction acceptance. Original production
// messages are forwarded unchanged; no snapshots or rule results are fabricated.
window.__controllerTransportDelay = 0;
window.__controllerDelayedMessages = [];
const originalControllerSocketSend = WebSocket.prototype.send;
const controllerSendQueues = new WeakMap();
const controllerTracedSockets = new WeakSet();
window.__controllerAcks=[];window.__controllerSentInputs=[];
WebSocket.prototype.send = function(data) {
  if(!controllerTracedSockets.has(this)){
    controllerTracedSockets.add(this);
    this.addEventListener('message',e=>{try{const p=JSON.parse(e.data);if(p.type==='ack'){window.__controllerAcks.push({at:performance.now(),...p});if(window.__controllerAcks.length>200)window.__controllerAcks.shift();}}catch{}});
  }
  let packet;try{packet=JSON.parse(data)}catch{}
  if(packet?.type!=='input'){originalControllerSocketSend.call(this,data);return;}
  const at=performance.now(),requested=window.__controllerTransportDelay;
  const queue=controllerSendQueues.get(this)??[];controllerSendQueues.set(this,queue);
  const due=at+requested,delay=Math.max(due,...queue.map(entry=>entry.due))-at;
  const trace={at,delay,nonce:packet.nonce,action:packet.action};
  window.__controllerSentInputs.push(trace);if(window.__controllerSentInputs.length>200)window.__controllerSentInputs.shift();
  if(!queue.length&&requested<=0){trace.forwardedAt=performance.now();originalControllerSocketSend.call(this,data);return;}
  window.__controllerDelayedMessages.push(trace);
  queue.push({due,data,trace});
  if(queue.length>1)return;
  const drain=()=>{
    while(queue.length){
      const next=queue[0],remaining=next.due-performance.now();
      if(remaining>0){setTimeout(drain,remaining);return;}
      queue.shift();
      if(this.readyState===WebSocket.OPEN){next.trace.forwardedAt=performance.now();originalControllerSocketSend.call(this,next.data);}
    }
  };
  setTimeout(drain,Math.max(0,due-performance.now()));
};
