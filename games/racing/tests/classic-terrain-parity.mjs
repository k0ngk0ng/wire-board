import assert from 'node:assert/strict';
import {readFileSync,writeFileSync} from 'node:fs';
import {classicTerrainFromCatalog,classicTerrainAt,classicTerrainSurface} from '../web/classic-terrain.ts';
const [fixture,report]=process.argv.slice(2),rows=JSON.parse(readFileSync(fixture)),worlds=new Map();
const index=JSON.parse(readFileSync(new URL('../server/data/terrain-index.json',import.meta.url)));
for(const entry of index){const raw=JSON.parse(readFileSync(new URL(`../assets/public/racing/v3/${entry.sourcePath}`,import.meta.url)));worlds.set(entry.id,classicTerrainFromCatalog({...raw,unit:'metres',geometry:entry.geometry,sourceSha256:entry.sourceSha256}));}
let maximumError=0;
function compare(a,b,label){for(const key of Object.keys(a)){if(typeof a[key]==='number'){const error=Math.abs(a[key]-b[key]);maximumError=Math.max(maximumError,error);assert(error<1e-6,`${label}.${key}: ${a[key]} vs ${b[key]}`);}else assert.equal(a[key],b[key],label+'.'+key);}}
for(const row of rows){const world=worlds.get(row.track);compare(row.contact,classicTerrainAt(world,row.car.x,row.car.y,row.car.height??0),row.track+'.contact');compare(row.surface,classicTerrainSurface(world,row.car,{grip:1,normalY:.98,grade:.03,resistance:0}),row.track+'.wheelSurface');}
writeFileSync(report,JSON.stringify({passed:true,probes:rows.length,tracks:worlds.size,maximumError,scope:'Go frozen numeric terrain versus original client metre blobs; triangle layer/normal/raw effect and original four-wheel axle material factors'},null,2)+'\n');
console.log(`Verified ${rows.length} real terrain contacts/four-wheel surfaces across ${worlds.size} tracks.`);
