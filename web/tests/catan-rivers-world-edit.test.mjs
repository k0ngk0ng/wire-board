import test from 'node:test';
import assert from 'node:assert/strict';
import {swapRiverWorld} from '../src/catan-rivers-world-edit.ts';
const map={hexes:[{resource:4,number:6},{resource:10,number:0},{resource:0,number:5},{resource:1,number:9}],channels:[{tiles:[0,1],outlet:3}]};
test('river terrain stays pinned and edits preserve inventory and source',()=>{
 const before=structuredClone(map);
 assert.equal(swapRiverWorld(map,0,2,false),map);
 assert.equal(swapRiverWorld(map,1,2,true),map);
 const result=swapRiverWorld(map,2,3,false);
 assert.deepEqual(result.hexes,[map.hexes[0],map.hexes[1],map.hexes[3],map.hexes[2]]);
 assert.deepEqual(map,before);assert.deepEqual(result.channels,map.channels);
 const numbers=swapRiverWorld(map,0,2,true);
 assert.equal(numbers.hexes[0].resource,4);assert.equal(numbers.hexes[0].number,5);
 assert.equal(numbers.hexes[2].number,6);assert.deepEqual(map,before);
});
