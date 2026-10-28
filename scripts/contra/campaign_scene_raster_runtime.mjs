// Isolated production-renderer oracle verification, never a game/session replay.
import {readFileSync,writeFileSync,mkdirSync,rmSync} from 'node:fs';
import {resolve} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {gunzipSync} from 'node:zlib';
import {execFileSync} from 'node:child_process';
const root=resolve(import.meta.dirname,'../..'),base=resolve(root,'.local/contra-raster/renderer-proof');mkdirSync(base,{recursive:true});
const productionFiles=['games/contra/web/runtime-renderer.ts','games/contra/web/background.ts','games/contra/web/sprites.ts','games/contra/web/runtime-bridge.ts'];
const sourceBefore=Object.fromEntries(productionFiles.map(file=>[file,createHash('sha256').update(readFileSync(resolve(root,file))).digest('hex')]));
const require=createRequire(resolve(root,'web/package.json'));const esbuild=require('esbuild');
const bundle=await esbuild.build({stdin:{contents:'export {RuntimeRenderer} from "./games/contra/web/runtime-renderer.ts"; export {decodeChrPage} from "./games/contra/web/sprites.ts";',resolveDir:root,loader:'ts'},bundle:true,platform:'node',format:'esm',write:false});
const {RuntimeRenderer,decodeChrPage}=await import('data:text/javascript;base64,'+Buffer.from(bundle.outputFiles[0].text).toString('base64'));
const run=resolve(root,'.local/contra-raster/oracle/runs/raster-v1');const events=JSON.parse(gunzipSync(readFileSync(resolve(root,'docs/research/nes-contra/evidence/campaign-scene-raster-original.json.gz')))).events;
const frames=new Map(readFileSync(resolve(run,'frames.csv'),'utf8').trim().split('\n').slice(1).map(line=>{const[a,b]=line.split(',');return[Number(a),Number(b)]}));
const ram=readFileSync(resolve(run,'ram.bin')),rgb=readFileSync(resolve(root,'.local/contra-background/runs/background-v1/background-colors.rgb'));const sha=bytes=>createHash('sha256').update(bytes).digest('hex');const checks=[];
for(const frame of [6500,6550,6600,7250,7300,7500,7800]){
 const emulatorFrame=frames.get(frame),es=events.filter(e=>e.frame===emulatorFrame),raster=es.find(e=>e.kind==='pc-9fe5'),split=es.find(e=>e.kind==='pc-f980'),effect=raster??split;
 const state=ram.subarray((frame-2)*2048,(frame-1)*2048),ppu=readFileSync(resolve(run,`frame-${String(frame).padStart(5,'0')}-ppu.bin`)),mapping=[...state.subarray(0x7f0,0x7f8)];
 const assets=new Map(),pages={};const asset=(name,bytes)=>{const url=`https://research.invalid/${name}`;const old=assets.get(url);if(old&&!Buffer.from(old).equals(Buffer.from(bytes)))throw Error('Conflicting normalized page');assets.set(url,bytes);return{url,bytes:bytes.length,sha256:sha(bytes)}};
 for(let slot=0;slot<8;slot++)pages[String(mapping[slot])]=asset(`page-${mapping[slot]}`,decodeChrPage(ppu.subarray(slot*1024,(slot+1)*1024)));
 const manifest={version:1,pages,rgb:asset('rgb',rgb)};globalThis.fetch=async url=>{const bytes=assets.get(String(url));if(!bytes)throw Error('unexpected asset fetch');return new Response(bytes,{status:200})};
 let output;const context={imageSmoothingEnabled:true,createImageData:(w,h)=>({data:new Uint8ClampedArray(w*h*4),width:w,height:h}),putImageData:image=>{output=Buffer.from(image.data)}};const canvas={width:0,height:0,getContext:()=>context};
 const renderer=new RuntimeRenderer(canvas);await renderer.load(manifest,()=>{});
 const view={Background:[...ppu.subarray(0x2000,0x2800)],OAM:[...state.subarray(0x200,0x300)],Patterns:mapping,Palette:[...ppu.subarray(0x3f00,0x3f20)],Scroll:{X:effect.shadowX,Y:effect.shadowY,NameTable:effect.control},Display:{AppliedMask:effect.mask,SplitActive:!!split,RasterActive:!!raster,SplitScroll:split?.scroll??0,RasterMask:raster?(raster.mask&0xef):0,RasterControl:raster?(raster.control|1):0}};
 renderer.draw(view);if(!output||output.length!==245760)throw Error('missing renderer output');writeFileSync(resolve(base,`${frame}.rgba`),output);
 checks.push({capture:frame,emulatorFrame,kind:raster?'interlude':'intro',rgbaSHA256:sha(output),sourcePNG:resolve(run,`frame-${String(frame).padStart(5,'0')}.png`)});
}
writeFileSync(resolve(base,'checks.json'),JSON.stringify({checks,sourceBefore}));
execFileSync(resolve(root,'.local/board-art-env/bin/python'),['-B','-c',`from pathlib import Path
import json,hashlib
from PIL import Image
root=Path.cwd();base=root/'.local/contra-raster/renderer-proof';payload=json.loads((base/'checks.json').read_text());checks=payload['checks']
for c in checks:
 actual=Image.open(c['sourcePNG']).convert('RGBA').tobytes();got=(base/(str(c['capture'])+'.rgba')).read_bytes();c['pixelsCompared']=61440;c['mismatchPixels']=sum(actual[i:i+4]!=got[i:i+4]for i in range(0,len(got),4));assert c['mismatchPixels']==0,c
files=['games/contra/web/runtime-renderer.ts','games/contra/web/background.ts','games/contra/web/sprites.ts','games/contra/web/runtime-bridge.ts']
after={f:hashlib.sha256((root/f).read_bytes()).hexdigest()for f in files};assert after==payload['sourceBefore'],'renderer source changed during comparison'
result={'passed':True,'sourceChangedDuringCheck':[],'scope':'Isolated production RuntimeRenderer.draw verification with original presentation oracle and normalized assets; no recorded state is supplied to any game/session entry.','checks':checks,'sourceHashes':after}
(root/'docs/research/nes-contra/evidence/campaign-scene-raster-runtime.json').write_text(json.dumps(result,indent=2)+'\\n');print(json.dumps({'passed':True,'frames':len(checks),'pixelsCompared':sum(c['pixelsCompared']for c in checks),'mismatches':0}))`],{cwd:root,stdio:'inherit'});
rmSync(base,{recursive:true});
