// Failure inventory before implementation: byte counts disappear in wrappers
// or initialization, compressed scene sizes are replaced with decoded sizes,
// parallel/cached resources are counted twice, failed cache entries survive,
// percentages are used to invent sizes, or other arenas enter the total.
import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {gzipSync} from 'node:zlib';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {loadAssets,resourceIDsForArena} from '../web/assets.ts';
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const names={rules:'chu-world-rules',sprites:'world-sprites',hud:'hud-assets',render_meta:'renderstate-metadata','arena-default':'arena-assets-default','arena-a-aux':'arena-a-aux-assets',...Object.fromEntries(['2','4','6','8','a','c','e'].map(n=>['arena-'+n,'arena-assets-'+n]))};
const stream=bytes=>new ReadableStream({start(c){const middle=Math.max(1,Math.floor(bytes.length/2));c.enqueue(bytes.slice(0,middle));c.enqueue(bytes.slice(middle));c.close();}});
function fixture(seed){
 const bodies=new Map();const resources=Object.entries(names).map(([id,name])=>{
  const raw=Buffer.from(JSON.stringify({id,seed,value:'x'.repeat(2048)})),gzip=gzipSync(raw),logical_path='/yuyuhakusho/data/'+name+'.json.gz';bodies.set(logical_path,gzip);
  return{id,logical_path,raw_json_sha256:hash(raw),raw_json_bytes:raw.length,gzip_sha256:hash(gzip),gzip_bytes:gzip.length};
 });
 const descriptor=Buffer.from(JSON.stringify({schema:1,resources}));
 return{bodies,resources,descriptor,sha:hash(descriptor),total:arena=>resources.filter(r=>resourceIDsForArena(arena).includes(r.id)).reduce((n,r)=>n+r.gzip_bytes,0)};
}
test('selected compressed resources report exact intermediate and final bytes',async t=>{
 const f=fixture('cold'),seen=[],calls=[];
 t.mock.method(globalThis,'fetch',async input=>{const url=String(input);calls.push(url);const bytes=url.endsWith('runtime-resource-descriptor.json')?f.descriptor:f.bodies.get(new URL(url).pathname);assert.ok(bytes,url);return new Response(stream(bytes));});
 await loadAssets('https://assets.test',10,f.sha,new AbortController().signal,p=>seen.push(p));
 assert.equal(seen.at(-1).loaded,f.total(10));assert.equal(seen.at(-1).total,f.total(10));
 assert.ok(seen.some(p=>p.loaded>0&&p.loaded<p.total));
 assert.ok(seen.filter(p=>p.total!==undefined).every(p=>p.loaded<=p.total));
 assert.equal(calls.filter(url=>url.endsWith('.json.gz')).length,6);
 assert.ok(!calls.some(url=>/arena-assets-[2468ce]\./.test(url)));
});
test('warm resources count once, retain sizes, and identify cache reuse',async t=>{
 const f=fixture('warm'),calls=[],seen=[];
 t.mock.method(globalThis,'fetch',async input=>{const url=String(input);calls.push(url);return new Response(stream(url.endsWith('runtime-resource-descriptor.json')?f.descriptor:f.bodies.get(new URL(url).pathname)));});
 await loadAssets('https://warm.test',0,f.sha,new AbortController().signal,()=>{});const before=calls.length;
 await loadAssets('https://warm.test',0,f.sha,new AbortController().signal,p=>seen.push(p));
 assert.equal(calls.length-before,1);assert.equal(seen.at(-1).loaded,f.total(0));assert.equal(seen.at(-1).total,f.total(0));assert.ok(seen.some(p=>p.label.includes('缓存')));
});
test('failed verification can retry without accumulating the failed transfer',async t=>{
 const f=fixture('retry');let fail=true;
 t.mock.method(globalThis,'fetch',async input=>{const url=String(input);let bytes=url.endsWith('runtime-resource-descriptor.json')?f.descriptor:f.bodies.get(new URL(url).pathname);if(fail&&url.endsWith('world-sprites.json.gz')){fail=false;bytes=Buffer.from(bytes);bytes[0]^=1;}return new Response(stream(bytes));});
 await assert.rejects(loadAssets('https://retry.test',0,f.sha,new AbortController().signal,()=>{}));
 const seen=[];await loadAssets('https://retry.test',0,f.sha,new AbortController().signal,p=>seen.push(p));
 assert.equal(seen.at(-1).loaded,f.total(0));assert.ok(seen.filter(p=>p.total!==undefined).every(p=>p.loaded<=p.total));
});
const wasmSource=stripTypeScriptTypes(readFileSync(new URL('../web/wasm.ts',import.meta.url),'utf8').replace(/from ["']\.\/assets["']/, 'from '+JSON.stringify(new URL('../web/assets.ts',import.meta.url).href)));
const {loadNetworkWasm}=await import('data:text/javascript;base64,'+Buffer.from(wasmSource).toString('base64'));
test('program bytes survive stream download, initialization and warm reuse',async t=>{
 const oldWindow=globalThis.window,oldLocation=globalThis.location;
 globalThis.window={Go:class{importObject={};async run(){}},yuyuhakushoNetworkWasm:()=>'{"ok":true}'};globalThis.location={href:'https://runtime.test/',origin:'https://runtime.test'};
 t.after(()=>{if(oldWindow===undefined)delete globalThis.window;else globalThis.window=oldWindow;if(oldLocation===undefined)delete globalThis.location;else globalThis.location=oldLocation;});
 const bytes=new Uint8Array(512).map((_,i)=>i%251),loader=32,manifest={wasm:{url:'client.wasm',sha256:hash(bytes),bytes:bytes.length},files:{'wasm_exec.js':{bytes:loader,sha256:'a'.repeat(64)}}};
 t.mock.method(globalThis,'fetch',async input=>String(input).endsWith('build.json')?Response.json(manifest):new Response(stream(bytes)));
 t.mock.method(WebAssembly,'instantiate',async()=>({instance:{}}));
 const seen=[];await loadNetworkWasm('https://runtime.test/v1',p=>seen.push(p));
 assert.ok(seen.some(p=>p.loaded>loader&&p.loaded<p.total));assert.equal(seen.at(-1).loaded,bytes.length+loader);assert.equal(seen.at(-1).total,bytes.length+loader);
 assert.ok(seen.filter(p=>p.label.includes('初始化')).every(p=>p.total===bytes.length+loader));
 const warm=[];await loadNetworkWasm('https://runtime.test/v1',p=>warm.push(p));assert.equal(warm.at(-1).loaded,bytes.length+loader);assert.match(warm.at(-1).label,/缓存/);
});
test('parallel totals sum measured resources rather than percentage weights',async()=>{
 const {combineLoadingProgress,loadingStage}=await import('../web/loading-progress.ts');
 const p=combineLoadingProgress({percent:50,label:'程序',loaded:8192,total:16384},{percent:50,label:'素材',loaded:4096,total:8192},'下载');
 assert.equal(p.loaded,12288);assert.equal(p.total,24576);
 assert.equal(combineLoadingProgress({percent:80,label:'程序'},{percent:20,label:'素材'},'读取清单').total,undefined);
 assert.deepEqual(loadingStage(p,92,'恢复比赛画面'),{...p,percent:92,label:'恢复比赛画面'});
});
