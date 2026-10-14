// Before implementation: all combinations sound like paper; a final round event
// replaces the winning play's voice; declared wild pairs announce printed rank;
// fallback URLs become two queued copies instead of one interchangeable clip.
import test from 'node:test';
import assert from 'node:assert/strict';
import {selectPokerSound} from '../src/poker/audio-events.ts';
import {soundFor as ddz} from '../../games/doudizhu/web/sounds.ts';
import {soundFor as gd} from '../../games/guandan/web/sounds.ts';
import {soundFor as sj} from '../../games/shengji/web/sounds.ts';
const e=(type,kind='',value=0)=>({id:1,type,kind,value,seat:0,at:100,cards:[0]});
test('landlord calls tiers and each combination has its own spoken cue',()=>{
 assert.match(ddz(e('bid','',2),true),/tier-2\.mp3$/);assert.match(ddz(e('play','trio_pair')),/trio-pair\.mp3$/);assert.match(ddz(e('play','straight')),/straight\.mp3$/);assert.match(ddz(e('play','bomb')),/bomb\.mp3$/);assert.match(ddz(e('play','rocket')),/rocket\.mp3$/);
});
test('a wildcard pair announces the declared rank rather than the printed level',()=>{assert.match(gd({...e('play','pair',8),cards:[31,85]}),/pair-8\.mp3$/)});
test('shengji calls the actual structure of the submitted cards',()=>{assert.match(sj(e('play','tractor')),/tractor\.mp3$/);assert.match(sj(e('play','follow')),/follow\.mp3$/)});
test('the final played combination takes precedence over settlement noise',()=>{
 const events=[e('play','rocket'),{...e('round_end'),id:2},{...e('finish'),id:3}];assert.match(selectPokerSound(events,ddz),/rocket\.mp3$/);assert.equal(events.length,3);
});
