// Small counters only: no copied packets, draw histories or atlas inspection.
(() => {
 const metrics={frames:[],messages:[],longTasks:[],draws:0,painted:0,clicks:[]};
 window.__density=metrics;
 const WS=window.WebSocket;
 window.WebSocket=class extends WS {constructor(...args){super(...args);this.addEventListener('message',e=>{if(typeof e.data==='string')metrics.messages.push({at:performance.now(),bytes:new TextEncoder().encode(e.data).byteLength});});}};
 let prior=0;
 function frame(at){if(prior)metrics.frames.push(at-prior);prior=at;requestAnimationFrame(frame);}
 requestAnimationFrame(frame);
 new PerformanceObserver(list=>{for(const e of list.getEntries())metrics.longTasks.push(e.duration)}).observe({type:'longtask',buffered:false});
 for(const name of ['drawImage','stroke','fill','fillRect']) {
  const original=CanvasRenderingContext2D.prototype[name];
  CanvasRenderingContext2D.prototype[name]=function(...args){if(this.canvas.matches('[data-starfort-canvas]'))metrics.draws++;return original.apply(this,args);};
 }
 const clear=CanvasRenderingContext2D.prototype.clearRect;
 CanvasRenderingContext2D.prototype.clearRect=function(...args){if(this.canvas.matches('[data-starfort-canvas]'))metrics.painted++;return clear.apply(this,args);};
 document.addEventListener('pointerdown',e=>{if(e.target.closest('[data-sf-open-info]')){const at=performance.now();const observer=new MutationObserver(()=>{if(document.querySelector('.sf-info-dialog'))requestAnimationFrame(()=>{metrics.clicks.push(performance.now()-at);observer.disconnect();});});observer.observe(document.body,{childList:true,subtree:true});}},true);
 window.__resetDensity=()=>{metrics.frames=[];metrics.messages=[];metrics.longTasks=[];metrics.draws=0;metrics.painted=0;metrics.clicks=[];prior=0;};
})();
