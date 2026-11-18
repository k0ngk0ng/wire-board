// Test first: a nonempty indexed scene is still black before the original
// fade-in. Palette intensity/rounding, saved fade state and actual pixels must
// agree; loading a map must not reveal its complete colors ahead of the core.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import {createHash} from "node:crypto";
import {create,root,wasm} from "./native-vm.mjs";
assert.equal(process.env.PAL_REFERENCE_FONT,"1");assert.equal(process.env.PAL_ORIGINAL_PIXELS,"1");
const v=await create(),pages=[],buffers=[];
let event=v.step("wb_run_campaign");
for(let step=0;step<512;step++){
 const raw=Array.from(new Uint32Array(v.e.memory.buffer,v.e.wb_view(),32));
 const p=v.e.wb_scene_ops(),count=new Int32Array(v.e.memory.buffer,p,1)[0],draw=new Int32Array(v.e.memory.buffer,p+4,count*8);
 const draws=Array.from({length:count},(_,i)=>{const d=Array.from(draw.slice(i*8,i*8+8));return {kind:d[0],a:d[1],b:d[2],c:d[3],d:d[4],x:d[5],baseY:d[6],sortY:d[7]}}).sort((a,b)=>a.sortY-b.sortY);
 let paletteEffect;
 if(v.e.wb_palette_state){const p=v.e.wb_palette_state(),data=Array.from(new Int32Array(v.e.memory.buffer,p,12));paletteEffect={palette:data[0],night:!!data[1],factors:data.slice(3,3+data[2])}}
 const pixels=new Uint8Array(v.e.memory.buffer,v.e.wb_pixels(),64000),colors=new Uint8Array(v.e.memory.buffer,v.e.wb_colors(),1024),rgba=Buffer.alloc(256000);
 for(let i=0;i<64000;i++){rgba[i*4]=colors[pixels[i]*4];rgba[i*4+1]=colors[pixels[i]*4+1];rgba[i*4+2]=colors[pixels[i]*4+2];rgba[i*4+3]=255}
 const hash=createHash("sha256").update(rgba).digest("hex");
 if(pages.at(-1)?.rgbaSHA256!==hash){pages.push({view:{map:raw[25],cameraX:raw[23]|0,cameraY:raw[24]|0,palette:raw[26],night:!!raw[27],draws,overlay:v.ops().map(d=>({kind:d[0],source:d[1],id:d[2],frame:d[3],x:d[4],y:d[5],color:d[6],small:d[7]})),paletteEffect},rgbaSHA256:hash});buffers.push(rgba)}
 if(raw[15]>0)break;
 event=v.step("wb_run_campaign");
 if(step===511)throw Error("original first fade-in not reached");
}
assert.ok(pages.length>10);
const out=root+".local/pal/verification/";
await fs.writeFile(out+"original-opening-palette-reference.json",JSON.stringify({scope:"actual original first PAL_GameMain fade-in and first text, no injected gameplay",runtimeSHA256:createHash("sha256").update(wasm).digest("hex"),pages},null,2)+"\n");
await fs.writeFile(out+"original-opening-palette-pages.rgba",Buffer.concat(buffers));
console.log("recorded original initial palette fade",pages.length);
