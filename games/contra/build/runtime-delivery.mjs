import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {existsSync,lstatSync,readFileSync,readdirSync,renameSync,rmSync} from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';

export function verifyContraDelivery(root){
  const receipt=JSON.parse(readFileSync(path.join(root,'contra-delivery.json'),'utf8'));
  assert.match(receipt.runtimeBuildID,/^[a-f0-9]{64}$/);
  assert.match(receipt.rulesBuildID,/^[a-f0-9]{64}$/);
  assert.equal(receipt.prefix,`contra/runtime-${receipt.runtimeBuildID}/`);
  assert.ok(receipt.files['manifest.json']);
  assert.equal(receipt.files['manifest.json'].sha256,receipt.runtimeBuildID);
  const expected=new Set(['contra-delivery.json']);
  for(const [name,entry] of Object.entries(receipt.files)){
    assert.equal(path.posix.normalize(name),name);
    assert.ok(!name.startsWith('/')&&!name.split('/').includes('..')&&!name.includes('\\'));
    assert.ok(['.json','.js','.wasm','.idx','.rgb','.wav'].includes(path.extname(name)));
    const relative=receipt.prefix+name,file=path.join(root,relative);
    assert.ok(lstatSync(file).isFile()&&!lstatSync(file).isSymbolicLink());
    const raw=readFileSync(file);assert.equal(raw.length,entry.bytes);
    assert.equal(createHash('sha256').update(raw).digest('hex'),entry.sha256);
    expected.add(relative);
  }
  const actual=new Set();
  function walk(directory,prefix=''){
    for(const item of readdirSync(directory,{withFileTypes:true})){
      const relative=prefix+item.name;
      assert.ok(!item.isSymbolicLink());
      if(item.isDirectory())walk(path.join(directory,item.name),relative+'/');
      else {assert.ok(item.isFile());actual.add(relative);}
    }
  }
  walk(root);assert.deepEqual(actual,expected);
  return receipt;
}

// Every invocation verifies the source package and rebuilds source-bound
// identities. Only delivery.py's exact runtime payload enters public assets.
export function prepareContraDelivery(root){
  const build=path.join(root,'.local/contra-web-runtime');
  const destination=path.join(root,'.local/contra-web-delivery');
  execFileSync('python3',['-B','scripts/contra/browser_build.py','--out',build],{cwd:root,stdio:'inherit'});
  if(existsSync(destination))rmSync(destination,{recursive:true,force:true});
  execFileSync('python3',['-B','games/contra/build/delivery.py','--runtime',path.join(build,'public/manifest.json'),'--out',destination],{cwd:root,stdio:'inherit'});
  renameSync(path.join(destination,'delivery.json'),path.join(destination,'contra-delivery.json'));
  verifyContraDelivery(destination);
  return destination;
}
