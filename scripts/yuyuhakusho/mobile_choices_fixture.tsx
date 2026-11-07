// Interaction fixture only: real production Options/Preparation and image menu.
// Failure inventory: native touch fails to open, early blur/toggle closes before
// click, dragging picks an item, outside/ESC cannot close, disabled/other seats
// change, duplicate submit, clipping or lost alignment at mobile widths.
import {createRoot} from "../../web/node_modules/react-dom/client";
import {useState} from "react";
import {Preparation} from "../../games/yuyuhakusho/web/Preparation";
import {OptionsForm} from "../../games/yuyuhakusho/web/Options";
const events: unknown[]=[];
Object.assign(window,{mobileChoiceEvents:events});
function Fixture(){
 const [options,setOptions]=useState({arena:0,teamBattle:false,characters:[3,9]});
 const [mode,setMode]=useState("create"),[busy,setBusy]=useState(false);
 return <main style={{maxWidth:720,margin:"0 auto",padding:20}}>
  <nav><button onClick={()=>setMode("create")}>创建</button><button onClick={()=>setMode("waiting")}>准备</button><button onClick={()=>setBusy(!busy)}>切换禁用</button></nav>
  {mode==="create"?<section role="dialog" aria-label="房间设置"><h1>创建房间</h1><OptionsForm value={options} onChange={value=>{events.push(value);setOptions(value)}} capacity={2} disabled={busy}/></section>:
  <section aria-label="比赛准备"><h1>比赛准备</h1><Preparation room={{id:"choice",name:"touch",kind:"yuyuhakusho",host:"human",status:"waiting",version:1,you:0,capacity:2,seats:[{id:"human",name:"玩家",ready:false},{id:"bot",name:"电脑",bot:true,ready:true}]}} options={options} host busy={busy} assets="" prepare={action=>{events.push(action);setOptions(previous=>action.type==="arena"?{...previous,arena:Number(action.arena)}:{...previous,characters:previous.characters.map((id,seat)=>seat===action.seat?Number(action.character):id)})}}/></section>}
  <button id="outside" aria-label="外部按钮" style={{position:"fixed",left:0,top:0,width:14,height:14,padding:0}}>×</button>
 </main>;
}
createRoot(document.getElementById("root")!).render(<Fixture/>);
