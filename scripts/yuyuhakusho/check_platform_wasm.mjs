// Actual new network Wasm adapter, with exact local resource bytes.
// Failure contract is written before pixel-target/type validation changes.
import assert from 'node:assert/strict';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {gunzipSync} from 'node:zlib';
import {createHash} from 'node:crypto';
import {fileURLToPath,pathToFileURL} from 'node:url';
const root=fileURLToPath(new URL('../../',import.meta.url));
const build=root+'.local/md-yuyuhakusho/platform-web/';
const out=root+'games/yuyuhakusho/verification/platform-web/';
await mkdir(out,{recursive:true});
const label=process.argv[2]??'wasm-final';
await import(pathToFileURL(build+'wasm_exec.js'));
const go=new globalThis.Go(),binary=await readFile(build+'client.wasm');
const {instance}=await WebAssembly.instantiate(binary,go.importObject);void go.run(instance);
const call=(command,...args)=>JSON.parse(globalThis.yuyuhakushoNetworkWasm(JSON.stringify(command),...args));
const okay=(command,...args)=>{const value=call(command,...args);assert.equal(value.ok,true,value.error);return value;};
const data=root+'games/yuyuhakusho/data/';
const compressed=async name=>new Uint8Array(gunzipSync(await readFile(data+name)));
const buffers=await Promise.all(['chu-world-rules.json.gz','world-sprites.json.gz','arena-assets-default.json.gz','hud-assets.json.gz'].map(compressed));
buffers.push(new Uint8Array(),await compressed('renderstate-metadata.json.gz'),new Uint8Array(await readFile(data+'runtime-resource-descriptor.json')));
let report={label,actual_go_wasm:true,wasm_sha256:createHash('sha256').update(binary).digest('hex'),checks:[]};
try{
 const asset=okay({op:'load',arena:0},...buffers).asset;
 const options={Seed:42,Difficulty:0,Players:[{Slot:0,CharacterID:3,Color:0,HumanChannel:0,Computer:false},{Slot:3,CharacterID:9,Color:1,HumanChannel:1,Computer:true}]};
 const session=okay({op:'prepare',asset,options}).session;
 okay({op:'ready',session});
 const before=okay({op:'save',session}).snapshot;
 const invalid=call({op:'step',session,held:[8,0,0,0],automated:2},new Uint8Array(4));
 assert.equal(invalid.ok,false);assert.equal(okay({op:'save',session}).snapshot,before,'Rejected pixel target advanced stored session');
 report.checks.push('invalid_step_target_atomic');
 const invalidPreview=call({op:'preview',session},new Uint8Array(3));assert.equal(invalidPreview.ok,false);assert.equal(okay({op:'save',session}).snapshot,before);
 const pixels=new Uint8Array(320*224*4);okay({op:'preview',session},pixels);assert.equal(okay({op:'save',session}).snapshot,before);assert.ok(pixels.some(value=>value!==0));report.checks.push('preview_detached_pixels');
 const good=okay({op:'step',session,held:[0x740,0,0,0],automated:2},pixels);assert.equal(good.view.frame,'1');report.checks.push('valid_step_twelve_bits');
 const malformed=call({op:'load',arena:0},null,...buffers.slice(1));assert.equal(malformed.ok,false);okay({op:'inspect',session});report.checks.push('non_uint8_asset_rejected_runtime_alive');
 const changed=buffers.map(value=>new Uint8Array(value));changed[0]=changed[0].slice(0,-1);assert.equal(call({op:'load',arena:0},...changed).ok,false);report.checks.push('exact_raw_json_identity');
 const sessions=[session];for(let n=0;n<15;n++)sessions.push(okay({op:'fork',session}).session);assert.equal(call({op:'fork',session}).ok,false);for(const id of sessions)okay({op:'dispose',session:id});okay({op:'disposeAssets',asset});report.checks.push('bounded_sessions_disposal');
 report.pass=true;
}catch(error){report.pass=false;report.error=String(error).split('\n')[0];await writeFile(out+label+'.json',JSON.stringify(report,null,2)+'\n');console.error(report.error);process.exit(1);}
await writeFile(out+label+'.json',JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify(report));process.exit(0);
