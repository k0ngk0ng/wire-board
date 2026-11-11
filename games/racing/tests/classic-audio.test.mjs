// Before implementation: RPM/load tables must drive native loop settings;
// model-specific voices must stay bounded and invalid RPM must remain finite.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {classicEngineVoices} from '../web/classic-audio.ts';

for (const id of ['p911','fm50','ldia','mcf1','ccor']) test(`${id} original RPM/load tables produce bounded native voices`, () => {
  const meta=JSON.parse(readFileSync(new URL(`../assets/public/racing/v3/audio/${id}/engine.json`,import.meta.url)));
  const signatures=new Set();
  for (const rpm of [NaN,-1,meta.idleRPM,2500,4500,meta.redlineRPM,20000]) for(const throttle of [0,.5,1]) {
    const voices=classicEngineVoices(meta,rpm,throttle);
    assert(voices.length>0&&voices.length<=8);
    let energy=0;
    for(const voice of voices){assert(voice.path.startsWith(`audio/${id}/`));assert(Number.isFinite(voice.volume)&&voice.volume>=0&&voice.volume<=.25);assert(Number.isFinite(voice.rate)&&voice.rate>=.25&&voice.rate<=4);energy+=voice.volume**2;}
    assert(energy<=.25**2+1e-9);
    signatures.add(JSON.stringify(voices.map(v=>[v.path,Math.round(v.volume*1000),Math.round(v.rate*1000)])));
  }
  assert(signatures.size>4,'engine ignores RPM or load');
});
