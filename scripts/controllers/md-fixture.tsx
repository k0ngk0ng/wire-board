import React, { useEffect, useMemo, useState } from '../../web/node_modules/react';
import { createRoot } from '../../web/node_modules/react-dom/client';
import { MDController } from '../../web/src/controllers/MDController';
import { FCController } from '../../web/src/controllers/FCController';
import { ControllerSettings } from '../../web/src/controllers/Settings';
import { KeyboardController, sampleLogicalControllerButton } from '../../web/src/controllers/input';
import '../../web/src/style.css';

const md={revision:0,turboHz:10,keyboard:{up:'KeyW',down:'KeyS',left:'KeyA',right:'KeyD',mode:'KeyF',start:'Space',b:'KeyJ',a:'KeyK',c:'KeyL',x:'KeyU',y:'KeyI',z:'KeyO'}};
const fc={revision:0,turboHz:10,keyboard:{up:'KeyW',down:'KeyS',left:'KeyA',right:'KeyD',select:'KeyF',start:'Space',b:'KeyJ',a:'KeyK',turboB:'KeyU',turboA:'KeyI'}};
const state={held:{} as Record<number,readonly string[]>,trusted:[] as boolean[],keys:[] as unknown[],sample:()=>[] as string[]};
(window as any).mdTest=state;
window.addEventListener('pointerdown',e=>state.trusted.push(e.isTrusted));
window.addEventListener('keydown',e=>state.keys.push({key:e.key,code:e.code,target:(e.target as HTMLElement)?.tagName,trusted:e.isTrusted}));
window.addEventListener('error',e=>state.keys.push({event:'error',message:e.message,stack:e.error?.stack}));
const began=new Map<number,number>();
const sink={pressPointer(id:number,buttons:readonly string[]){state.held[id]=[...buttons];if(!began.has(id))began.set(id,performance.now())},releasePointer(id:number){delete state.held[id];began.delete(id)}};

function App(){
 const[settings,setSettings]=useState(false),[disabled,setDisabled]=useState(false),[visible,setVisible]=useState(true),[model,setModel]=useState('md-6'),[revision,setRevision]=useState(0);
 const profile=useMemo(()=>({... (model==='md-6'?md:fc),revision}),[model,revision]);
 const keyboard=useMemo(()=>new KeyboardController(profile),[]);
 useEffect(()=>{keyboard.configure(profile)},[profile]);
 useEffect(()=>{
  keyboard.clear();
  const down=(e:KeyboardEvent)=>{if(settings||disabled||!visible||e.target instanceof HTMLInputElement||e.target instanceof HTMLSelectElement||e.target instanceof HTMLTextAreaElement)return;keyboard.press(e.code,performance.now(),e.repeat)};
  const up=(e:KeyboardEvent)=>keyboard.release(e.code),clear=()=>keyboard.clear();
  const hidden=()=>{if(document.hidden)clear()};
  window.addEventListener('keydown',down);window.addEventListener('keyup',up);window.addEventListener('blur',clear);document.addEventListener('visibilitychange',hidden);
  return()=>{clear();window.removeEventListener('keydown',down);window.removeEventListener('keyup',up);window.removeEventListener('blur',clear);document.removeEventListener('visibilitychange',hidden)};
 },[settings,disabled,visible]);
 state.sample=()=>{const result=keyboard.sample(performance.now());for(const[id,buttons]of Object.entries(state.held))for(const b of buttons){const value=sampleLogicalControllerButton(b,began.get(Number(id))??0,performance.now(),profile.turboHz);if(value)result.add(value)}return [...result].sort()};
 const Pad=model==='md-6'?MDController:FCController;
 return <main style={{maxWidth:820,margin:'16px auto',padding:12}}>
  <div style={{display:'flex',gap:8,flexWrap:'wrap',marginBottom:12}}>
   <button id="settings" onClick={()=>setSettings(true)}>控制器设置</button>
   <select id="play-model" aria-label="游玩控制器型号" value={model} onChange={e=>setModel(e.target.value)}><option value="md-6">MD</option><option value="classic-2">FC</option></select>
   <button id="disable" onClick={()=>setDisabled(v=>!v)}>切换禁用</button>
   <button id="profile" onClick={()=>setRevision(v=>v+1)}>切换配置</button>
   <button id="unmount" onClick={()=>setVisible(v=>!v)}>切换显示</button>
  </div>
  <section id="play-surface" tabIndex={0} style={{background:'#172126',padding:4,borderRadius:12}}>{visible&&<Pad profile={profile} mode="play" sink={sink} disabled={disabled||settings}/>}</section>
  <input id="text-input" aria-label="输入框隔离验证" style={{marginTop:12,width:'100%'}}/>
  {settings&&<ControllerSettings userId="md-browser" onClose={()=>setSettings(false)}/>}
 </main>;
}
createRoot(document.getElementById('root')!).render(<App/>);
