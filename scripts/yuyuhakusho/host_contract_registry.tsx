// Test-only registry for public UI SSR contracts. It supplies no game rules.
import type {GameModule} from '../../web/src/game-api/module';
const Options=()=> <label data-contract-options>默认设置<select defaultValue="one"><option value="one">一</option></select></label>;
const Preparation=()=> <section data-contract-preparation>默认准备</section>;
const Summary=({seat}:{seat:number})=><span data-contract-summary>资源 {seat===0?0:23}</span>;
const base={id:'contract-default',name:'公共默认契约',description:'仅界面契约',minPlayers:2,maxPlayers:4,supportsBots:true,Board:()=> <div data-contract-board/>,Options,Preparation,PlayerSummary:Summary};
const modules:Record<string,unknown>={
 'contract-default':base,
 'contract-opted':{...base,id:'contract-opted',preparationOwnsOptions:true,viewportFit:true,playerLayout:{density:'compact',groups:()=>[{id:'a',label:'A队',seats:[0,1]},{id:'b',label:'B队',seats:[2,3]}]}},
 'contract-invalid':{...base,id:'contract-invalid',playerLayout:{density:'compact',groups:()=>[{id:'bad',label:'错误',seats:[0,0]}]}}
};
export const gameModule=(kind:string)=>modules[kind] as GameModule|undefined;
export const gameName=(kind:string)=>gameModule(kind)?.name??kind;
export const gameLoaded=()=>true;export const isIndependentGame=()=>true;export const useGameRegistry=()=>0;export const loadGame=async(kind:string)=>gameModule(kind)!;
