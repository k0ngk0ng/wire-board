// Failure inventory before implementation: a new owner or old playbackID can
// revive another lane; finite/ended SFX revive; ordinary unmute replays a track;
// restored paused/muted music plays audibly before its flags/offset apply;
// active restore creates duplicate media; source or loop changes sneak through;
// delayed metadata/play/fallback after stop, replacement or disposal resurrects;
// invalid offset, hidden page or global mute bypasses availability; paused restore
// loses fallback or a newer seek before its first native play.
import test from 'node:test';import assert from 'node:assert/strict';
import {GameAudio} from '../src/audio.ts';
class Media {
 dataset={};src='';currentSrc='';currentTime=0;duration=10;readyState=1;paused=true;muted=false;volume=1;loop=false;ended=false;calls=[];onloadedmetadata=null;onerror=null;onended=null;behavior=()=>Promise.resolve();
 play(){this.calls.push({src:this.src,at:this.currentTime,muted:this.muted});this.paused=false;return this.behavior()}
 pause(){this.paused=true}load(){this.currentTime=0}removeAttribute(k){if(k==='src')this.src=''}remove(){this.removed=true}
}
function fixture(configure=()=>{}){const media=[],audio=new GameAudio(()=>{const m=new Media();configure(m,media.length);media.push(m);return m});return{audio,media,p:audio.channel('room'),q:audio.channel('other')}}
test('explicit same-owner same-ID restore recreates only its original continuous source and retains soft mute',async()=>{
 const {media,p,q}=fixture();await p.track('bg',['music','fallback'],{loop:true,loopStartSeconds:2,playbackID:'e:1',muted:true});p.stop();
 assert.equal(await q.restoreTrack('bg','e:1',{startOffset:15}),false);assert.equal(await p.restoreTrack('bg','old',{startOffset:15}),false);
 assert.equal(await p.restoreTrack('bg','e:1',{startOffset:15}),true);assert.equal(media.length,2);const m=media[1];m.onloadedmetadata();assert.equal(m.currentTime,7);assert.equal(m.calls[0].muted,true);assert.equal(m.src,'music');
 const count=m.calls.length;assert.equal(await p.restoreTrack('bg','e:1',{startOffset:16,muted:false}),true);assert.equal(media.length,2);assert.equal(m.calls.length,count);assert.equal(m.currentTime,8);assert.equal(m.muted,false);
});
test('paused restoration prepares without play, honors the newest seek and keeps primary fallback',async()=>{
 const {media,p}=fixture(m=>{m.behavior=()=>m.src==='primary'?Promise.reject(Error('cdn')):Promise.resolve()});
 await p.track('bg',['primary','fallback'],{loop:true,playbackID:'song'});p.stopTrack('bg');
 assert.equal(await p.restoreTrack('bg','song',{startOffset:3,paused:true,muted:true}),true);const m=media.at(-1);assert.equal(m.calls.length,0);assert.equal(m.paused,true);assert.equal(p.seekTrack('bg',4),true);
 assert.equal(await p.resumeTrack('bg'),true);assert.deepEqual(m.calls.map(v=>v.src),['primary','fallback']);assert.ok(m.calls.every(v=>v.muted&&v.at===4));
});
test('finite clips, missing identities, hidden/global mute and invalid offsets never restore',async()=>{
 const {audio,media,p}=fixture();await p.track('shot',['shot'],{playbackID:'shot'});await p.track('bg',['bg'],{loop:true,playbackID:'song'});p.stop();const count=media.length;
 for(const [key,id,offset]of[['shot','shot',0],['bg','',0],['missing','song',0],['bg','song',NaN],['bg','song',-1],['bg','song',Infinity]])assert.equal(await p.restoreTrack(key,id,{startOffset:offset}),false);
 audio.setEnabled(false);assert.equal(await p.restoreTrack('bg','song',{startOffset:2}),false);audio.setEnabled(true);assert.equal(media.length,count);
 const prior=globalThis.document;globalThis.document={hidden:true};try{assert.equal(await p.restoreTrack('bg','song',{startOffset:2}),false)}finally{globalThis.document=prior}
 audio.dispose();assert.equal(await p.restoreTrack('bg','song',{startOffset:2}),false);assert.equal(media.length,count);
});
test('replacement and late pending restore cannot revive a retired identity',async()=>{
 let finish;const {audio,media,p}=fixture((m,index)=>{if(index===1){m.readyState=0;m.duration=NaN;m.behavior=()=>new Promise(r=>finish=r)}});
 await p.track('bg',['old'],{loop:true,playbackID:'old'});p.stop();const restored=p.restoreTrack('bg','old',{startOffset:5});const old=media[1],metadata=old.onloadedmetadata;
 await p.track('bg',['new'],{loop:true,playbackID:'new'});old.readyState=1;old.duration=10;metadata?.();finish();assert.equal(await restored,false);assert.equal(old.paused,true);assert.equal(old.src,'');assert.equal(await p.restoreTrack('bg','old',{startOffset:1}),false);assert.equal(media.at(-1).src,'new');audio.dispose();
});
// A source clock may differ from the authority clock. Rate must apply before
// first play/fallback, stay bound to event identity and survive restore; offsets
// remain media seconds. Invalid rates cannot create a lane or alter one.
test('track rate binds the native source clock and is preserved by stopped and paused restoration',async()=>{
 const {media,p}=fixture();const rate=.9983560090702946;
 await p.track('bg',['music'],{loop:true,playbackID:'song',rate});assert.equal(media[0].playbackRate,rate);assert.equal(media[0].defaultPlaybackRate,rate);
 assert.equal(await p.track('bg',['music'],{loop:true,playbackID:'song',rate:1}),false);p.stop();
 assert.equal(await p.restoreTrack('bg','song',{startOffset:4,paused:true}),true);assert.equal(media[1].playbackRate,rate);assert.equal(media[1].calls.length,0);await p.resumeTrack('bg');assert.equal(media[1].currentTime,4);assert.equal(media[1].playbackRate,rate);
 const count=media.length;for(const rate of [NaN,Infinity,0,-1,.2,4.1])assert.equal(await p.track('bad',['music'],{loop:true,rate}),false);assert.equal(media.length,count);
});
