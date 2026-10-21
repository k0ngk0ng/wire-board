// Before implementation: distinct classic body shapes, finite closed geometry,
// common collision envelope, and names must not be unrelated paint variants.
import test from 'node:test';
import assert from 'node:assert/strict';
import { CAR_MODELS, bodySurface } from '../web/car-definitions.ts';
test('classic catalog contains Porsche, Lamborghini and Ferrari plus three distinct cars',()=>{
 assert.equal(CAR_MODELS.length,6);for(const name of ['保时捷','兰博基尼','法拉利'])assert.ok(CAR_MODELS.some(c=>c.name.includes(name)));
 assert.equal(new Set(CAR_MODELS.map(c=>JSON.stringify(c.body))).size,6);
 assert.equal(new Set(CAR_MODELS.map(c=>c.name)).size,6);
});
test('all sculpted shells stay inside the shared physics footprint and have valid faces',()=>{
 for(const c of CAR_MODELS){const g=bodySurface(c);assert.ok(g.vertices.length>50&&g.indices.length>100);
  for(let i=0;i<g.vertices.length;i+=3){assert.ok(Math.abs(g.vertices[i])<=23.01);assert.ok(g.vertices[i+1]>=0&&g.vertices[i+1]<=12);assert.ok(Math.abs(g.vertices[i+2])<=12.5)}
  for(const index of g.indices)assert.ok(Number.isInteger(index)&&index>=0&&index<g.vertices.length/3);
 }
});
