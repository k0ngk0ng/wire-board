import {renderToStaticMarkup} from '../../web/node_modules/react-dom/server';
import {GameBoard,ModuleCreate,ModuleWaiting,ModulePlayers} from '../../web/src/game-api/host';
import type {Room} from '../../web/src/types';
const room=(kind:string)=>({id:'contract-room',name:'默认契约',kind,host:'u0',you:0,capacity:4,status:'playing',version:1,options:{},seats:[{id:'u0',name:'甲',ready:true,autoPlay:true,network:{connected:true,rttMs:0}},{id:'u1',name:'乙',ready:true,left:true},{id:'b2',name:'电脑',ready:true,bot:true},{id:'u3',name:'丁',ready:true}],game:{finished:false,winners:[],log:[],actors:[],attention:[],eliminated:[3],data:{}}} as unknown as Room);
const noop=()=>{};
export function html(){
 const render=(node:React.ReactNode)=>renderToStaticMarkup(node);
 const waiting=(kind:string)=>render(<ModuleWaiting room={room(kind)} host busy={false} command={noop}/>);
 const players=(kind:string)=>render(<ModulePlayers room={room(kind)}/>);
 const create=(kind:string)=>render(<ModuleCreate kind={kind}busy={false}onSubmit={noop}draft={{name:'默认',password:'',capacity:4,options:{}}}onDraftChange={noop} availableGames={[]}/>);
 const board=(kind:string)=>render(<GameBoard room={room(kind)}act={async()=>{}}command={async()=>{}}busy={false}assets=""sound={false}audio={{}as never}clock={{serverNow:0,receivedAt:0}}/>);
 return{defaultWaiting:waiting('contract-default'),defaultPlayers:players('contract-default'),defaultCreate:create('contract-default'),defaultBoard:board('contract-default'),ownedWaiting:waiting('contract-opted'),ownedPlayers:players('contract-opted'),ownedCreate:create('contract-opted'),ownedBoard:board('contract-opted'),invalidPlayers:players('contract-invalid')};
}
