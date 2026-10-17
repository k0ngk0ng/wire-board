import { GameAudio } from '../../web/src/audio';
import { BasketballAudio } from '../../games/basketball/web/audio';
const audio = new GameAudio();
audio.mount();
const adapter = new BasketballAudio(audio.channel('basketball-restored-effects'),{localBase:'/audio',assetsBase:'/unavailable'});
const state={audio,adapter,sequence:0,id:1,report:undefined as unknown,completed:false};
Object.assign(window,{effects:state});
document.querySelector('button')!.onclick=async()=>{
 state.completed=false;
 await audio.unlock();
 state.report=await adapter.consume([state.id],++state.sequence);
 state.completed=true;
};
