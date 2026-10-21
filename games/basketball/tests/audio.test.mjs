import test from 'node:test';
import assert from 'node:assert/strict';
import catalog from '../assets/audio-catalog.json' with { type: 'json' };
import delivery from '../assets/audio-playback.json' with { type: 'json' };
import playback from '../assets/audio-playback.json' with { type: 'json' };
import { BasketballAudio } from '../web/audio.ts';

function port() {
  const calls=[];
  return { calls, play:async sources=>{calls.push(['playlist',...sources]);return true;},
    playWithFallback:async sources=>{calls.push(['effect',...sources]);return true;},
    stop:()=>{calls.push(['stop']);},
    music:async (sources,options)=>{calls.push(['music',sources,options]);return true;},
    stopMusic:()=>{calls.push(['stopMusic']);},
    pause:()=>{calls.push(['pause']);}, resume:async()=>{calls.push(['resume']);return true;},
    loop:async()=>{throw Error('private loop route forbidden');},stopLoop:()=>{}
  };
}

test('new original music clears pause and repeats of the same ID restart the track',async()=>{
  const p=port(),a=new BasketballAudio(p);
  await a.consume([66,253,66],1);
  assert.deepEqual(p.calls.map(c=>c[0]),['music','pause','resume','stopMusic','music']);
  await a.consume([0,66],2);
  assert.deepEqual(p.calls.slice(-2).map(c=>c[0]),['stop','music']);
});

test('music recovery seeks the current revision without replaying old effects',async()=>{
  const p=port(),a=new BasketballAudio(p);
  a.reset(20);
  const state={epoch:3,trackID:66,assetSHA256:delivery.music['66'].distribution_sha256,controlSequence:17,positionMS:12500,at:100000,paused:false};
  assert.equal(await a.restoreMusic(state,100250),true);
  assert.deepEqual(p.calls.map(c=>c[0]),['stop','music']);
  assert.equal(p.calls[1][2].startOffset,12.75);
  assert.ok(p.calls[1][2].playbackID.includes(state.assetSHA256));
  assert.equal((await a.consume([19],19)).ignored,true);
  const before=p.calls.length;
  assert.equal(await a.restoreMusic({...state,assetSHA256:'0'.repeat(64)}),false);
  assert.equal(p.calls.length,before);
});

test('paused recovery freezes its media position until an original resume or new music',async()=>{
  const p=port(),a=new BasketballAudio(p);a.reset(20);
  const state={epoch:3,trackID:66,assetSHA256:delivery.music['66'].distribution_sha256,controlSequence:20,positionMS:12500,at:100000,paused:true};
  await a.restoreMusic(state,190000);
  assert.deepEqual(p.calls.map(c=>c[0]),['stop','music','pause']);
  assert.equal(p.calls[1][2].startOffset,12.5);
  await a.consume([66],21);
  assert.deepEqual(p.calls.slice(-3).map(c=>c[0]),['resume','stopMusic','music']);
});

test('synchronous music-port failures remain an audio failure, not a game exception',async()=>{
  const p=port(),a=new BasketballAudio(p);a.reset(20);
  p.music=()=>{throw Error('unavailable player');};
  assert.equal(await a.restoreMusic({epoch:3,trackID:66,assetSHA256:delivery.music['66'].distribution_sha256,controlSequence:20,positionMS:12500,at:100000,paused:false}),false);
});

test('opening resume with no current music is a no-op, while a failed real music resume stays reported',async()=>{
  const p=port(),a=new BasketballAudio(p);p.resume=async()=>false;
  assert.deepEqual((await a.consume([253,255,66],1)).failed,[]);
  assert.deepEqual((await a.consume([253,255],2)).failed,[255]);
});

test('all 27 original effects use one platform fallback request per ID',async()=>{
  const p=port(),a=new BasketballAudio(p,{assetsBase:'https://assets.example.test/release/',localBase:'/research/basketball/audio/'});
  const ids=catalog.effects.map(e=>e.id),result=await a.consume(ids,1);
  assert.equal(ids.length,27);assert.deepEqual(result,{ignored:false,unsupported:[],failed:[]});
  assert.deepEqual(p.calls,ids.map(id=>['effect',`https://assets.example.test/release/basketball/v1/audio/sound-${String(id).padStart(3,'0')}.wav`,`/research/basketball/audio/sound-${String(id).padStart(3,'0')}.wav`]));
});

test('controls keep order and resume never starts music again',async()=>{
  const p=port(),a=new BasketballAudio(p);
  await a.consume([66,19,253,255,253,255,0],1);
  assert.deepEqual(p.calls,[['music',['/basketball/v1/audio/music-066-loop.mp3'],{loop:true}],['effect','/basketball/v1/audio/sound-019.wav'],['pause'],['resume'],['pause'],['resume'],['stop']]);
});

test('events are deduplicated and reset establishes the recovery watermark',async()=>{
  const p=port(),a=new BasketballAudio(p);
  await a.consume([19],20);
  assert.equal((await a.consume([21],20)).ignored,true);
  assert.equal((await a.consume([21],19)).ignored,true);
  a.reset(100);
  assert.equal((await a.consume([21],99)).ignored,true);
  await a.consume([21],101);
  a.dispose();
  assert.equal((await a.consume([19],102)).ignored,true);
  assert.deepEqual(p.calls.map(c=>c[0]),['effect','stop','effect','stop']);
});

test('unsupported IDs never become guessed files and no user mute setting is touched',async()=>{
  const p=port(),a=new BasketballAudio(p);
  const result=await a.consume([2,254,300,-1,NaN,1.5],1);
  assert.deepEqual(result.unsupported,[2,254,300,-1,NaN,1.5]);assert.deepEqual(p.calls,[]);
});

test('platform failures resolve as reports and cannot break the simulation',async()=>{
  const p=port();p.playWithFallback=()=>{throw Error('blocked');};p.resume=async()=>{throw Error('cancelled');};
  const a=new BasketballAudio(p);
  assert.deepEqual(await a.consume([19,255,66],1),{ignored:false,unsupported:[],failed:[19,255]});
  assert.deepEqual(p.calls,[['music',['/basketball/v1/audio/music-066-loop.mp3'],{loop:true}]]);
});

test('all calls issue synchronously without a slow media promise delaying later controls',async()=>{
  const p=port();let settle;p.playWithFallback=s=>{p.calls.push(['effect',...s]);return new Promise(r=>{settle=r;});};
  const a=new BasketballAudio(p),pending=a.consume([19,253,255,0],1);
  assert.deepEqual(p.calls.map(c=>c[0]),['effect','pause','resume','stop']);
  settle(false);assert.deepEqual((await pending).failed,[19]);
});

test('each original Court selects its own verified continuous music',async()=>{
  for (const id of [59,66,61,64,60,62,63,65]) {
    const p=port(),a=new BasketballAudio(p,{assetsBase:'https://assets.example.test/immutable',localBase:'/local/audio'});
    assert.deepEqual(await a.consume([id],1),{ignored:false,unsupported:[],failed:[]});
    const file=`music-${String(id).padStart(3,'0')}-loop.mp3`;
    assert.deepEqual(p.calls,[['music',[`https://assets.example.test/immutable/basketball/v1/audio/${file}`,`/local/audio/${file}`],{loop:true}]]);
  }
});

test('the finite original ending75 plays once',async()=>{
  const p=port(),a=new BasketballAudio(p);
  assert.deepEqual(await a.consume([75],1),{ignored:false,unsupported:[],failed:[]});
  assert.deepEqual(p.calls,[['music',['/basketball/v1/audio/music-075.mp3'],{loop:false}]]);
});

test('restored court animations and special balls use their original extra effects',async()=>{
  const p=port(),a=new BasketballAudio(p);
  assert.deepEqual(await a.consume([13,14,30,46],1),{ignored:false,unsupported:[],failed:[]});
  assert.deepEqual(p.calls,[13,14,30,46].map(id=>['effect',`/basketball/v1/audio/sound-${String(id).padStart(3,'0')}.wav`]));
});

test('a missing compressed rendition cannot silently fall back to a large WAV master',async()=>{
  const saved=playback.music['66'].file;
  delete playback.music['66'].file;
  try {
    const {BasketballAudio:Missing}=await import('../web/audio.ts?without-encoded-music');
    const p=port(),a=new Missing(p);
    assert.deepEqual((await a.consume([66],1)).unsupported,[66]);
    assert.deepEqual(p.calls,[]);
  } finally {playback.music['66'].file=saved;}
});

test('captured running, jump and running-release original requests have explicit effect assets',async()=>{
  const p=port(),a=new BasketballAudio(p,{localBase:'/audio'});
  const result=await a.consume([1,23,48],1);
  assert.deepEqual(result,{ignored:false,unsupported:[],failed:[]});
  assert.deepEqual(p.calls,[1,23,48].map(id=>['effect',`/audio/sound-${String(id).padStart(3,'0')}.wav`]));
  for(const id of [1,48]) {
    assert.match(playback.effects[id].sha256,/^[a-f0-9]{64}$/);
    assert.equal(playback.effects[id].listening_accepted,false);
  }
});

// Music76 failures recorded before registration: missing/invalid central loop
// metadata, intro restart on recovery, stale revision, and unversioned saves.
test('original ending76 uses one verified MP3 and the platform suffix option',async()=>{
  const entry=playback.music['76'];assert.ok(entry,'verified76 delivery entry missing');
  const p=port(),a=new BasketballAudio(p,{assetsBase:'/cdn',localBase:'/audio'});
  assert.deepEqual(await a.consume([76],1),{ignored:false,unsupported:[],failed:[]});
  assert.deepEqual(p.calls,[['music',[`/cdn/basketball/v1/audio/${entry.file}`,`/audio/${entry.file}`],{loop:true,loopStartSeconds:entry.loop_start_sample/entry.sample_rate}]]);
  assert.equal(entry.loop_end_sample-entry.loop_start_sample,entry.loop_samples);
  assert.equal(entry.loop_end_sample,entry.decoded_samples);
});
test('music76 restore keeps cumulative media wall time and rejects unversioned history',async()=>{
  const entry=playback.music['76'];assert.ok(entry);
  const p=port(),a=new BasketballAudio(p);a.reset(40);
  const state={epoch:7,trackID:76,assetSHA256:entry.distribution_sha256,controlSequence:40,positionMS:1900000,at:1000,paused:false};
  assert.equal(await a.restoreMusic(state,1500),true);
  assert.equal(p.calls[1][2].startOffset,1900.5);assert.equal(p.calls[1][2].loopStartSeconds,entry.loop_start_sample/entry.sample_rate);
  const n=p.calls.length;assert.equal(await a.restoreMusic({...state,assetSHA256:''}),false);assert.equal(p.calls.length,n);
  assert.equal(await a.restoreMusic({...state,paused:true},901000),true);assert.equal(p.calls.at(-2)[2].startOffset,1900);assert.equal(p.calls.at(-1)[0],'pause');
});
test('invalid76 loop sample metadata never becomes a guessed playback mapping',async()=>{
  const entry=playback.music['76'];assert.ok(entry);const saved={...entry};
  for(const [field,value]of [['sample_rate',0],['loop_start_sample',-1],['loop_end_sample',entry.loop_start_sample],['loop_samples',1],['decoded_samples',1]]){
    Object.assign(entry,saved);entry[field]=value;
    try{const {BasketballAudio:Invalid}=await import(`../web/audio.ts?invalid76-${field}`);const p=port();assert.deepEqual((await new Invalid(p).consume([76],1)).unsupported,[76]);assert.deepEqual(p.calls,[]);}
    finally{Object.assign(entry,saved);}
  }
});
