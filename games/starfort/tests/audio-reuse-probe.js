// Native media counters only; never replace play/load behavior or copy audio.
(()=>{const p=HTMLMediaElement.prototype,src=Object.getOwnPropertyDescriptor(p,'src'),load=p.load,play=p.play;const m=window.__sfAudio={sources:0,loads:0,plays:0,resolved:0,rejected:0,players:new Set(),bySource:{}};
 Object.defineProperty(p,'src',{configurable:true,get:src.get,set(value){if(String(value).includes('/starfort/'))this.__starfortAudio=true;if(this.__starfortAudio){m.sources++;const key=String(value).split('/starfort/').at(-1);m.bySource[key]=(m.bySource[key]??0)+1}src.set.call(this,value)}});
 p.load=function(){if(this.__starfortAudio)m.loads++;return load.call(this)};
 p.play=function(){const game=this.__starfortAudio;if(game){m.plays++;m.players.add(this)}const result=play.call(this);if(game)result.then(()=>m.resolved++,()=>m.rejected++);return result};
 window.__sfAudioSnapshot=()=>({sources:m.sources,loads:m.loads,plays:m.plays,resolved:m.resolved,rejected:m.rejected,players:m.players.size,bySource:{...m.bySource}});
})();
