// Isolated preparation presentation: authentic labels/PNGs, recorded requests.
// No mock match engine or claim that this harness authorizes server actions.
import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {Preparation} from '../../games/basketball/web/Preparation';
import type {BasketballOptions} from '../../games/basketball/web/module';
import type {GameRoom} from '../../web/src/game-api/module';
import '../../web/src/style.css';
const actions:Record<string,unknown>[]=[];
function Harness(){
 const [options,setOptions]=useState<BasketballOptions>({mode:'versus',court:'new-york',teams:[0,1],characters:[0,1,2,3]});
 const [host,setHost]=useState(true),[busy,setBusy]=useState(false),[you,setYou]=useState(0),[spectating,setSpectating]=useState(false);
 const room={id:'preparation',status:'waiting',you,spectating,seats:[{id:'host',name:'房主',bot:false,left:false},{id:'guest',name:'另一位玩家',bot:false,left:false}]} as GameRoom;
 Object.assign(window,{singlePreparation:{options,setOptions,host,setHost,busy,setBusy,you,setYou,spectating,setSpectating,actions}});
 return <main className="single-preparation-fixture"><h1>热血篮球 · 单场准备验收</h1><Preparation room={room} options={options} host={host} busy={busy} assets="/missing-cdn" prepare={a=>{actions.push(a)}}/></main>;
}
createRoot(document.querySelector('#root')!).render(<Harness/>);
