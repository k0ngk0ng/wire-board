import test from 'node:test';
import assert from 'node:assert/strict';
import {catanPortLayout} from '../src/catan-port-layout.ts';
test('New World before the first port accepts null or omitted port lists',()=>{
 for(const ports of [null,undefined,[]]){
  assert.deepEqual(catanPortLayout({ports,seafarers:{newWorld:{index:0}},edges:[],vertices:[],tiles:[]}),[]);
 }
});
