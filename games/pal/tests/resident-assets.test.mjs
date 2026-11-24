import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { mkdir, rm } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";
const root=fileURLToPath(new URL("../../../",import.meta.url));
const temporary=path.join(root,".local/pal/resident-assets/temporary");
await mkdir(temporary,{recursive:true});
const require=createRequire(new URL("../../../web/package.json",import.meta.url));
await require("esbuild").build({entryPoints:[path.join(root,"games/pal/web/resident-assets.ts")],outfile:path.join(temporary,"assets.mjs"),bundle:true,format:"esm",legalComments:"none"});
const {palResidentResources}=await import(pathToFileURL(path.join(temporary,"assets.mjs")));
await rm(temporary,{recursive:true,force:true});
const read=name=>JSON.parse(readFileSync(new URL("../assets/"+name+"-release.json",import.meta.url),"utf8"));
const visual=read("visual"), ending=read("ending"), audio=read("audio"), server=read("server");
const latest=Object.values(server.catalogs).at(-1);
const view={audio:{assetID:Object.keys(audio.catalogs)[0]},local:{core:latest.currentCore}};
test("story resources include all later scenes and sounds with one selected core",()=>{
 const files=palResidentResources(view),urls=new Set(files.map(file=>file.url));
 for(const publication of [Object.values(visual.catalogs).at(-1),Object.values(ending.catalogs).at(-1),audio.catalogs[view.audio.assetID]])for(const file of Object.values(publication.files))assert.ok(urls.has(file.url));
 assert.ok(urls.has(latest.files["cores/"+view.local.core+".wasm"].url));
 assert.equal(files.filter(file=>file.url.endsWith(".wasm")).length,1);
 assert.equal(urls.size,files.length);
});
test("unpublished save audio or cores cannot silently use a different version",()=>{
 assert.throws(()=>palResidentResources({...view,audio:{assetID:"missing"}}));
 assert.throws(()=>palResidentResources({...view,local:{core:"missing"}}));
});
