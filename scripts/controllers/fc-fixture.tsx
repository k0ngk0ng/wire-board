// Browser fixture for the actual shared controller and settings components.
import React, { useState } from '../../web/node_modules/react';
import { createRoot } from '../../web/node_modules/react-dom/client';
import { FCController } from '../../web/src/controllers/FCController';
import { ControllerSettings } from '../../web/src/controllers/Settings';
import '../../web/src/style.css';
const initial={revision:0,turboHz:10,keyboard:{up:'KeyW',down:'KeyS',left:'KeyA',right:'KeyD',select:'KeyF',start:'Space',b:'KeyJ',a:'KeyK',turboB:'KeyU',turboA:'KeyI'}};
const state={events:[] as unknown[],held:{} as Record<number,readonly string[]>,trusted:[] as boolean[]};
(window as any).fcTest=state;
window.addEventListener('pointerdown',e=>state.trusted.push(e.isTrusted));
const sink={pressPointer(id:number,buttons:readonly string[]){state.held[id]=[...buttons];state.events.push(['press',id,[...buttons]])},releasePointer(id:number,cancelled:boolean){delete state.held[id];state.events.push(['release',id,cancelled])}};
function App(){const[settings,setSettings]=useState(false),[disabled,setDisabled]=useState(false),[profile,setProfile]=useState(initial),[visible,setVisible]=useState(true);return <main style={{maxWidth:760,margin:'24px auto',padding:12}}><h1>FC 控制器验收</h1><p>共享组件与控制器设置的独立浏览器验证。</p><div style={{display:'flex',gap:8,flexWrap:'wrap',marginBottom:16}}><button id="settings" onClick={()=>setSettings(true)}>控制器设置</button><button id="disable" onClick={()=>setDisabled(v=>!v)}>切换禁用</button><button id="profile" onClick={()=>setProfile(p=>({...p,revision:p.revision+1,keyboard:{...p.keyboard,right:p.keyboard.right==='KeyD'?'ArrowRight':'KeyD'}}))}>切换个人键位</button><button id="unmount" onClick={()=>setVisible(v=>!v)}>切换显示</button></div><section id="play-surface" style={{padding:12,borderRadius:12,background:"#172126"}}>{visible&&<FCController profile={profile} mode="play" sink={sink} disabled={disabled || settings}/>}</section><p id="outside">手柄区域外可正常滚动。</p>{settings&&<ControllerSettings userId="fc-browser" onClose={()=>setSettings(false)}/>}</main>}
createRoot(document.getElementById('root')!).render(<App/>);
