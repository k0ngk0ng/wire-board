import assert from 'node:assert/strict';
import {readFileSync,mkdirSync,copyFileSync,writeFileSync,rmSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {gunzipSync} from 'node:zlib';
import path from 'node:path';
import {prepareYuYuRuntime} from './runtime.mjs';
const sha=raw=>createHash('sha256').update(raw).digest('hex');
export function verifyYuYuDelivery(root,publicRoot){
 const target=path.join(publicRoot,'yuyuhakusho/v1/runtime');
 const manifest=JSON.parse(readFileSync(path.join(target,'build.json')));
 const compiled=path.join(prepareYuYuRuntime(root),'yuyuhakusho/v1/runtime');
 const source=JSON.parse(readFileSync(path.join(compiled,'build.json')));
 assert.equal(manifest.sourceSHA256,source.sourceSHA256);
 for(const name of ['client.wasm','wasm_exec.js']){const raw=readFileSync(path.join(target,name));assert.equal(sha(raw),source.files[name].sha256);assert.equal(raw.length,source.files[name].bytes);}
 assert.equal(manifest.wasm.url,'./client.wasm');assert.deepEqual(manifest.wasm,{url:'./client.wasm',...source.files['client.wasm']});
 const descriptor=readFileSync(path.join(root,'games/yuyuhakusho/data/runtime-resource-descriptor.json'));
 assert.equal(sha(readFileSync(path.join(publicRoot,'yuyuhakusho/data/runtime-resource-descriptor.json'))),sha(descriptor));
 const resources=JSON.parse(descriptor).resources;assert.equal(resources.length,13);
 for(const entry of resources){const raw=readFileSync(path.join(publicRoot,entry.logical_path.replace(/^\//,'')));assert.equal(raw.length,entry.gzip_bytes);assert.equal(sha(raw),entry.gzip_sha256);const decoded=gunzipSync(raw);assert.equal(decoded.length,entry.raw_json_bytes);assert.equal(sha(decoded),entry.raw_json_sha256);}
 const catalog=readFileSync(path.join(root,'games/yuyuhakusho/assets/audio-playback.json'));
 const pin=JSON.parse(readFileSync(path.join(root,'games/yuyuhakusho/assets/audio-release.json')));
 assert.equal(pin.catalogSHA256,sha(catalog));assert.equal(pin.files,143);assert.equal(pin.assetsBaseURL,'https://cdn.ichenj.com/bga/yuyuhakusho/audio-'+sha(catalog));
 return {sourceSHA256:manifest.sourceSHA256,wasmSHA256:manifest.wasm.sha256,resources:resources.length,audioCatalogSHA256:pin.catalogSHA256};
}
export function prepareYuYuDelivery(root){
 const compiled=path.join(prepareYuYuRuntime(root),'yuyuhakusho/v1/runtime');
 const output=path.join(root,'.local/yuyuhakusho/runtime-delivery/public');
 rmSync(output,{recursive:true,force:true});const target=path.join(output,'yuyuhakusho/v1/runtime');mkdirSync(target,{recursive:true});
 const manifest=JSON.parse(readFileSync(path.join(compiled,'build.json')));
 for(const name of ['client.wasm','wasm_exec.js'])copyFileSync(path.join(compiled,name),path.join(target,name));
 writeFileSync(path.join(target,'build.json'),JSON.stringify({...manifest,wasm:{url:'./client.wasm',...manifest.files['client.wasm']}},null,2)+'\n');
 const dataRoot=path.join(root,'games/yuyuhakusho/data');const raw=readFileSync(path.join(dataRoot,'runtime-resource-descriptor.json'));
 const resources=JSON.parse(raw).resources;
 for(const entry of resources){const source=path.join(dataRoot,path.basename(entry.logical_path)),dest=path.join(output,entry.logical_path.replace(/^\//,''));mkdirSync(path.dirname(dest),{recursive:true});copyFileSync(source,dest);}
 copyFileSync(path.join(dataRoot,'runtime-resource-descriptor.json'),path.join(output,'yuyuhakusho/data/runtime-resource-descriptor.json'));
 verifyYuYuDelivery(root,output);return output;
}
