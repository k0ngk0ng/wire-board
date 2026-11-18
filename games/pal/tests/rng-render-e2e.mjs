// Test first: original deltas differ in JS, empty terminal records draw, prior
// screen pixels disappear, segmented playback repeats frames, bad data mutates
// its destination, or the indexed raster cannot reconstruct a saved movie.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {gunzipSync} from 'node:zlib';
const root=new URL('../../../',import.meta.url);
const {build}=createRequire(new URL('../../../web/package.json',import.meta.url))('esbuild');
async function module(name){const result=await build({entryPoints:[new URL('../web/'+name,import.meta.url).pathname],bundle:true,write:false,platform:'node',format:'esm',target:'es2022'});return import('data:text/javascript;base64,'+Buffer.from(result.outputFiles[0].contents).toString('base64'))}
const {applyRNGFrame,readRNGArchive}=await module('rng.ts');
const {renderRaster}=await module('raster-renderer.ts');
const reference=JSON.parse(await fs.readFile(new URL('games/pal/resources/testdata/rng-reference.json',root)));
const manifest=JSON.parse(await fs.readFile(new URL('.local/pal/visual/pal/v1/index.json',root)));
const movies=new Map();
const hash=data=>createHash('sha256').update(data).digest('hex');
for(let id=0;id<12;id++){
 const meta=JSON.parse(await fs.readFile(new URL(`.local/pal/visual/pal/v1/movies/${id}.json`,root)));
 const packed=await fs.readFile(new URL('.local/pal/visual/pal/v1/'+meta.data,root));
 assert.equal(hash(packed),manifest.files[meta.data].sha256);
 const frames=readRNGArchive(new Uint8Array(gunzipSync(packed)));
 assert.equal(frames.length,meta.frames);assert.equal(frames.at(-1).length,0);
 movies.set(id,frames);
}
const initial=fill=>Uint8Array.from({length:64000},(_,i)=>fill?255:i%251);
let clip=-1,fill=-1,pixels;
for(const v of reference.vectors){
 if(clip!==v.clip||fill!==v.fill){clip=v.clip;fill=v.fill;pixels=initial(fill)}
 applyRNGFrame(movies.get(clip)[v.frame],pixels);
 assert.equal(hash(pixels),v.sha256,`original C/JS RNG ${clip}/${v.frame}/${fill}`);
}
const node=(kind,parent,...params)=>[kind,parent,0,320,200,...params,...Array(11-params.length).fill(0)];
const base={sceneReady:false,clearColor:255,map:1,cameraX:0,cameraY:0,draws:[],overlay:[]};
const stops=[112,162,186,195,266,409];
for(let fill=0;fill<2;fill++)for(const frame of [0,1,55,112,113,162,266,267,409]){
 const nodes=[node(9,0)];let first=0;
 for(const end of stops){nodes.push(node(10,nodes.length,1,first,Math.min(frame,end)));first=end+1;if(frame<=end)break}
 const graph={version:1,root:nodes.length,nodes,base};
 const actual=await renderRaster(graph,()=>{throw Error('unexpected image')},()=>{throw Error('unexpected font')},{seed:async()=>initial(fill),movie:async id=>movies.get(id)});
 const want=reference.vectors.find(v=>v.clip===1&&v.fill===fill&&v.frame===frame);
 assert.equal(hash(actual),want.sha256,`saved/segmented graph ${frame}/${fill}`);
}
for(const bad of [[3],[6,1],[4,255,255,6,1,2]]){const pixels=initial(0),before=pixels.slice();assert.throws(()=>applyRNGFrame(Uint8Array.from(bad),pixels));assert.deepEqual(pixels,before)}
await assert.rejects(renderRaster({version:1,root:2,nodes:[node(9,0),node(10,1,1,410,410)],base},()=>{},()=>{},{seed:async()=>initial(0),movie:async id=>movies.get(id)}));
await fs.writeFile(new URL('.local/pal/verification/rng-render-js.json',root),JSON.stringify({passed:true,originalPixelComparisons:reference.vectors.length,segmentedSavedGraphs:18,scope:'exported RNG command archives, indexed JS decoding and saved/segmented raster graphs versus original C; not native/browser playback acceptance'},null,2)+'\n');
console.log('original RNG pixels and segmented raster graphs passed');
