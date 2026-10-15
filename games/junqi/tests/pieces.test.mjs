// User requirement: ownership is shown by color, not numeric corner labels.
import test from 'node:test';import assert from 'node:assert/strict';
import {pieceSVG,coverSVG,names,colors} from '../assets/art.ts';
test('all four armies and the cover have no numeric piece labels',()=>{
 for(let seat=0;seat<4;seat++)for(const kind of [...Object.keys(names),'back']){
  const svg=pieceSVG(seat,kind);assert.ok(svg.includes(colors[seat]));
  const visible=[...svg.matchAll(/<text\b[^>]*>([^<]*)<\/text>/g)].map(m=>m[1]);
  assert.ok(visible.every(s=>!/[0-9]/.test(s)),`${seat}/${kind}: numeric label`);
 }
 assert.ok([...coverSVG.matchAll(/<text\b[^>]*>([^<]*)<\/text>/g)].every(m=>!/[0-9]/.test(m[1])));
});
