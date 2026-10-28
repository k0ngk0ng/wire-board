// Failure inventory: Vite exposes private dev files, silently copies stale or
// incomplete payloads, accepts a different immutable prefix, or misses audio.
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,rmSync,renameSync} from 'node:fs';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {verifyContraDelivery} from './runtime-delivery.mjs';

test('only exact verified runtime files enter the Vite public directory',()=>{
  const parent=path.resolve('.local/contra-delivery-tests');mkdirSync(parent,{recursive:true});
  const root=mkdtempSync(path.join(parent,'test-'));
  try{
    const data=Buffer.from('{}');
    const sha=createHash('sha256').update(data).digest('hex');
    const id=sha,prefix=`contra/runtime-${id}/`;
    mkdirSync(path.join(root,prefix),{recursive:true});
    const receipt={runtimeBuildID:id,rulesBuildID:'b'.repeat(64),prefix,files:{'manifest.json':{bytes:2,sha256:sha,mime:'application/json',requiresRange:false}}};
    writeFileSync(path.join(root,prefix,'manifest.json'),data);
    const save=()=>writeFileSync(path.join(root,'contra-delivery.json'),JSON.stringify(receipt));save();
    assert.equal(verifyContraDelivery(root).runtimeBuildID,id);
    writeFileSync(path.join(root,prefix,'private.html'),'private');
    assert.throws(()=>verifyContraDelivery(root));rmSync(path.join(root,prefix,'private.html'));
    writeFileSync(path.join(root,prefix,'manifest.json'),'bad');
    assert.throws(()=>verifyContraDelivery(root));writeFileSync(path.join(root,prefix,'manifest.json'),data);
    receipt.prefix='contra/runtime-'+'c'.repeat(64)+'/';save();
    assert.throws(()=>verifyContraDelivery(root));
    receipt.runtimeBuildID='c'.repeat(64);save();
    renameSync(path.join(root,prefix),path.join(root,receipt.prefix));
    assert.throws(()=>verifyContraDelivery(root));
  } finally {rmSync(root,{recursive:true,force:true});}
});
