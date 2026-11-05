// Acceptance-only passive instrumentation. It never delays or edits messages.
(() => {
  if (window.yuyuAppObservation) return;
  const log = window.yuyuAppObservation = {views:[], inputs:[], acks:[], paints:[], keys:[], fetches:[], sockets:[], network:[]};
  const add = (list, value) => { list.push(value); if (list.length > 6000) list.shift(); };
  const NativeSocket = window.WebSocket;
  window.WebSocket = class extends NativeSocket {
    constructor(...args) {
      super(...args);
      add(log.sockets, {event:'create', at:performance.now(), url:String(args[0])});
      for (const name of ['open','close','error']) this.addEventListener(name, () => add(log.sockets,{event:name,at:performance.now()}));
      this.addEventListener('message', event => {
        let packet; try { packet=JSON.parse(event.data); } catch { return; }
        const at=performance.now(), view=packet.game?.data??packet.state?.room?.game?.data;
        if (view?.epoch && view.frame!==undefined) {
          log.latestView=view;
          add(log.views,{at,bytes:event.data.length,kind:packet.type,view,serverNow:packet.serverNow??packet.state?.serverNow});
        }
        if(packet.type==='ack') add(log.acks,{at,...packet});
        if(packet.type==='network') add(log.network,{at,self:packet.self,serverNow:packet.serverNow});
      });
    }
    send(value) {
      let packet; try {packet=JSON.parse(value);}catch{}
      if(packet?.action) add(log.inputs,{at:performance.now(),...packet});
      return super.send(value);
    }
  };
  const nativeFetch=window.fetch;
  window.fetch=async(...args)=>{
    const at=performance.now(); const response=await nativeFetch(...args);
    add(log.fetches,{at,end:performance.now(),url:String(args[0]),status:response.status});
    return response;
  };
  const nativePaint=CanvasRenderingContext2D.prototype.putImageData;
  CanvasRenderingContext2D.prototype.putImageData=function(image,...args){
    const result=nativePaint.call(this,image,...args);
    if(this.canvas.closest('[data-yuyu-board]')) {
      let hash=2166136261; for(let i=0;i<image.data.length;i++) hash=Math.imul(hash^image.data[i],16777619);
      const root=this.canvas.closest('[data-yuyu-board]');
      const paint={at:performance.now(),hash:(hash>>>0).toString(16)};
      add(log.paints,paint);
      queueMicrotask(()=>Object.assign(paint,{frame:root.dataset.frame,authorityFrame:root.dataset.authorityFrame,x:Number(root.dataset.localX),y:Number(root.dataset.localY)}));
    }
    return result;
  };
  for(const type of ['keydown','keyup','pointerdown','pointerup','pointercancel']) window.addEventListener(type,event=>{
    add(log.keys,{at:performance.now(),type,code:event.code,key:event.key,trusted:event.isTrusted,target:event.target?.tagName});
  },true);
})();
