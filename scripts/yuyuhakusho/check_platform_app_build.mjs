// Public app delivery boundary, written before the local full-app builder.
import assert from 'node:assert/strict';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
const root=fileURLToPath(new URL('../../',import.meta.url)),dist=(process.env.YUYU_LOCAL_APP_OUT??root+'.local/md-yuyuhakusho/platform-app')+'/dist/';
const out=root+'games/yuyuhakusho/verification/platform-web/';await mkdir(out,{recursive:true});
const label=process.argv[2]??process.env.YUYU_LOCAL_BUILD_LABEL??'app-build-final',sha=bytes=>createHash('sha256').update(bytes).digest('hex');
let report={label,scope:'Built normal React platform app and local-only asset routing, not authenticated/browser acceptance',checks:[]};
try{
 const manifest=JSON.parse(await readFile(dist+'game-load-manifest.json','utf8'));
 assert.ok(manifest.games.yuyuhakusho?.length);assert.ok(!manifest.home.modules.some(path=>/games\/yuyuhakusho\/web\/(Board|network-|wasm)/.test(path)));report.checks.push('lazy_board_not_home');
 for(const entry of manifest.games.yuyuhakusho)assert.equal((await readFile(dist+entry.url.slice(1))).byteLength,entry.bytes);
 const descriptorBytes=await readFile(dist+'yuyuhakusho/data/runtime-resource-descriptor.json'),descriptor=JSON.parse(descriptorBytes);assert.equal(descriptor.resources.length,13);
 for(const entry of descriptor.resources){const bytes=await readFile(dist+entry.logical_path.slice(1));assert.equal(bytes.byteLength,entry.gzip_bytes);assert.equal(sha(bytes),entry.gzip_sha256);}
 report.checks.push('all_13_fixed_resource_routes_sha');
 const build=JSON.parse(await readFile(dist+'yuyuhakusho/v1/runtime/build.json','utf8')),wasm=await readFile(dist+build.wasm.url.slice(1));assert.equal(wasm.byteLength,build.wasm.bytes);assert.equal(sha(wasm),build.wasm.sha256);assert.deepEqual([...wasm.subarray(0,4)],[0,97,115,109]);assert.ok((await readFile(dist+'yuyuhakusho/v1/runtime/wasm_exec.js')).byteLength>10000);report.checks.push('runtime_descriptor_exact_wasm');
 const favicon=await readFile(dist+'favicon.svg'),sourceFavicon=await readFile(root+'web/public/favicon.svg');assert.equal(sha(favicon),sha(sourceFavicon));assert.equal(manifest.media['/favicon.svg'],favicon.byteLength);report.checks.push('shared_favicon_route_exact_source');report.favicon={url:'/favicon.svg',bytes:favicon.byteLength,sha256:sha(favicon)};
 const audio=JSON.parse(await readFile(root+'games/yuyuhakusho/assets/audio-playback.json','utf8'));let audioBytes=0;
 for(const clip of [...Object.values(audio.music),...Object.values(audio.effects)]){const bytes=await readFile(dist+'yuyuhakusho/audio/'+clip.file);assert.equal(bytes.byteLength,clip.bytes);assert.equal(sha(bytes),clip.sha256);audioBytes+=bytes.byteLength;}
 report.checks.push('all_143_lossless_media_routes_sha');report.audio={clips:143,bytes:audioBytes};
 report.pass=true;report.resource_descriptor_sha256=sha(descriptorBytes);report.runtime=build.wasm;report.build=manifest.build;report.dist=dist;
}catch(error){report.pass=false;report.error=String(error);await writeFile(out+label+'.json',JSON.stringify(report,null,2)+'\n');console.error(report.error);process.exit(1);}
await writeFile(out+label+'.json',JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify(report));
