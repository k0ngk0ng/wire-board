// Original PAL_RNGPlay and original pixel archives vs stripped authority.
// Observe all six ranges, prior screen capture and every rendered frame. The
// browser consumes these real C hashes, not a second copy of the JS decoder.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {create,root,wasm} from './native-vm.mjs';
const reference=process.env.PAL_ORIGINAL_PIXELS==='1';
const v=await create();let ev=v.step('wb_run_campaign');
const digest=b=>createHash('sha256').update(b).digest('hex');
function seed(slot){
 const p=v.e.wb_raster_seed_info(slot),h=Array.from(new Int32Array(v.e.memory.buffer,p,8));
 assert.equal(h[0],1);assert.ok(h[6]>=0&&h[6]<=8192&&h[7]>=0&&h[7]<=8192);
 const words=Array.from(new Int32Array(v.e.memory.buffer,p+32,(h[6]+h[7])*8));
 const draws=Array.from({length:h[6]},(_,i)=>{const n=words.slice(i*8,i*8+8);return{kind:n[0],a:n[1],b:n[2],c:n[3],d:n[4],x:n[5],baseY:n[6],sortY:n[7]}}).sort((a,b)=>a.sortY-b.sortY);
 const overlay=Array.from({length:h[7]},(_,i)=>{const n=words.slice((h[6]+i)*8,(h[6]+i+1)*8);return{kind:n[0],source:n[1],id:n[2],frame:n[3],x:n[4],y:n[5],color:n[6],small:n[7]}});
 return{sceneReady:!!h[1],clearColor:h[2],map:h[3],cameraX:h[4],cameraY:h[5],draws,overlay};
}
function state(){
 const p=v.e.wb_raster_graph(),h=Array.from(new Int32Array(v.e.memory.buffer,p,3)),w=Array.from(new Int32Array(v.e.memory.buffer,p+12,h[1]*16));
 const raster={version:1,root:h[2],nodes:Array.from({length:h[1]},(_,i)=>w.slice(i*16,i*16+16))};
 for(const n of raster.nodes)if(n[0]===9){if(n[5]===0)raster.base=seed(0);else raster.backup=seed(1)}
 const raw=new Uint32Array(v.e.memory.buffer,v.e.wb_view(),32);
 return{clock:raw[13],seed:raw[12],map:raw[25],palette:raw[26],night:!!raw[27],cameraX:raw[23]|0,cameraY:raw[24]|0,draws:[],raster};
}
const pages=[],buffers=[],sequence=[];let last=-1;
for(let step=0;step<2000;step++){
 const view=state(),movie=view.raster.nodes.filter(n=>n[0]===10).at(-1);
 assert.ok(movie);const frame=movie[7];
 if(frame!==last){
  last=frame;sequence.push(view);
  if(reference){
   const pixels=new Uint8Array(v.e.memory.buffer,v.e.wb_pixels(),64000),colors=new Uint8Array(v.e.memory.buffer,v.e.wb_colors(),1024),rgba=Buffer.alloc(256000);
   for(let i=0;i<64000;i++){for(let c=0;c<3;c++)rgba[i*4+c]=colors[pixels[i]*4+c];rgba[i*4+3]=255}
   pages.push({label:`RNG-1-${frame}`,view,rgbaSHA256:digest(rgba)});buffers.push(rgba);
  }
 }
 if(ev.kind===3)break;
 ev=v.step('wb_run_campaign');assert.notEqual(step,1999);
}
assert.equal(sequence.length,410);assert.equal(last,409);
const out=root+'.local/pal/verification/';
if(reference){
 await fs.writeFile(out+'original-rng-reference.json',JSON.stringify({passed:true,runtimeSHA256:digest(wasm),scope:'original C sword lesson movie frames on nonzero prior screen; isolated playback, not campaign',pages},null,2)+'\n');
 await fs.writeFile(out+'original-rng-pages.rgba',Buffer.concat(buffers));
}else{
 const original=JSON.parse(await fs.readFile(out+'original-rng-reference.json'));
 assert.deepEqual(sequence,original.pages.map(p=>p.view));
 await fs.writeFile(out+'rng-native-projection.json',JSON.stringify({passed:true,frames:410,scope:'stripped authority draw graph and prior screen match original pixel C for all original sword lesson frames'},null,2)+'\n');
}
console.log(reference?'original C RNG frame hashes recorded':'headless RNG projections match original C');
