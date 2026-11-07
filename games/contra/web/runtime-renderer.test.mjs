// New raster-region failure cases, before implementing RasterActive rendering:
// marker Y is mistaken for the actual pattern/background overlap; mask/control
// start on the same row; control restarts screen Y at zero; sprites remain below
// the hit; retained values leak when Active=false; a missing hit is fabricated.
// This is a focused pure-pixel contract, separate from the live Session E2E.
import test from 'node:test';import assert from 'node:assert/strict';import {build} from '../../../web/node_modules/esbuild/lib/main.js';import {createRequire} from 'node:module';import {mkdirSync,writeFileSync} from 'node:fs';import {fileURLToPath} from 'node:url';import path from 'node:path';
const root=fileURLToPath(new URL('../../../',import.meta.url)),out=path.join(root,'.local/contra-browser/raster-test');mkdirSync(out,{recursive:true});
const built=await build({entryPoints:[path.join(root,'games/contra/web/runtime-renderer.ts')],bundle:true,write:false,platform:'node',format:'cjs'});const compiled=path.join(out,'renderer.cjs');writeFileSync(compiled,built.outputFiles[0].contents);const {RuntimeRenderer}=createRequire(import.meta.url)(compiled);
async function scene(transparent=false){
 const files=new Map(),pages={};const asset=async(name,bytes)=>{files.set(name,bytes);return{url:name,bytes:bytes.length,sha256:[...new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))].map(v=>v.toString(16).padStart(2,'0')).join('')}};
 for(let i=0;i<8;i++){const pixels=new Uint8Array(4096).fill(i===1?2:i===4?(transparent?0:3):1);pages[i]=await asset('/'+i,pixels)}
 const rgb=new Uint8Array(192);rgb.set([10,20,30],3);rgb.set([40,50,60],6);rgb.set([90,100,110],9);const manifest={pages,rgb:await asset('/rgb',rgb)};
 let drawn;const ctx={imageSmoothingEnabled:false,createImageData:()=>({data:new Uint8ClampedArray(256*240*4)}),putImageData:image=>{drawn=image.data}};const canvas={width:0,height:0,getContext:()=>ctx};const renderer=new RuntimeRenderer(canvas);
 const old=globalThis.fetch;globalThis.fetch=async url=>new Response(files.get(url));try{await renderer.load(manifest,()=>{})}finally{globalThis.fetch=old}
 const background=new Array(2048).fill(0);background.fill(64,1024,1984);background.fill(0,1024,1088);const oam=new Array(256).fill(255);oam.splice(0,4,20,1,0,8);const palette=new Array(32).fill(0);palette[1]=1;palette[2]=2;palette[19]=3;
 const view={Background:background,OAM:oam,Patterns:[0,1,2,3,4,5,6,7],Palette:palette,Scroll:{X:0,Y:0,NameTable:0x20},Display:{AppliedMask:0x1e,SplitActive:false,RasterActive:true,SplitScroll:0,RasterMask:0x0e,RasterControl:0x21}};
 return{renderer,view,pixel:(x,y)=>Array.from(drawn.slice((y*256+x)*4,(y*256+x)*4+3))};
}
test('interlude follows actual hit row for sprite mask and next row for background control',async()=>{const{renderer,view,pixel}=await scene();renderer.draw(view);assert.deepEqual(pixel(8,20),[10,20,30]);assert.deepEqual(pixel(8,21),[10,20,30]);assert.deepEqual(pixel(8,22),[40,50,60]);assert.deepEqual(pixel(150,21),[10,20,30]);assert.deepEqual(pixel(150,22),[40,50,60]);view.Display.RasterActive=false;renderer.draw(view);assert.deepEqual(pixel(8,21),[90,100,110]);assert.deepEqual(pixel(150,22),[10,20,30])});
test('an active raster effect without a real sprite/background hit does not invent a boundary',async()=>{const{renderer,view}=await scene(true);assert.throws(()=>renderer.draw(view),/hit|命中/)});
