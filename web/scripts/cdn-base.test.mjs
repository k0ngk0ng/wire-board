// Failure inventory: unsafe/different origin; missing immutable revision; local
// development forced onto CDN; absolute paths double-prefixed in the manifest.
import test from 'node:test';
import assert from 'node:assert/strict';
import {assetBase,assetURL} from './cdn-base.mjs';
const base='https://cdn.ichenj.com/bga/web/'+'a'.repeat(40)+'/';
test('uses local assets in development and validates immutable release base',()=>{
 assert.equal(assetBase(),'/');assert.equal(assetBase(base),base);
 for(const bad of ['http://cdn.ichenj.com/a/','https://evil.test/bga/web/'+'a'.repeat(40)+'/','https://cdn.ichenj.com/bga/web/main/','//cdn.ichenj.com/','https://cdn.ichenj.com/bga/web/'+'a'.repeat(40)+'/../'])assert.throws(()=>assetBase(bad));
});
test('resolves emitted assets once under the chosen base',()=>{assert.equal(assetURL('/','assets/a.js'),'/assets/a.js');assert.equal(assetURL(base,'assets/a.js'),base+'assets/a.js')});
