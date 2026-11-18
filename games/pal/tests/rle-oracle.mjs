// Original C raster reference, declared before the Go pixel decoder. The two
// fills distinguish transparent pixels from opaque palette indices 0 and 255.
import fs from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {createHash} from 'node:crypto';
const root=fileURLToPath(new URL('../../../',import.meta.url));
const program=await fs.readFile(path.join(root,'.local/pal/native/rules.wasm'));
const compiled=await WebAssembly.compile(program);const imports={};
for(const entry of WebAssembly.Module.imports(compiled)){
 if(entry.kind!=='function')throw Error('unexpected host state');
 imports[entry.module]??={};imports[entry.module][entry.name]=()=>{throw Error('unbound raster host call '+entry.module+'.'+entry.name)};
}
const instance=await WebAssembly.instantiate(compiled,imports);const e=instance.exports;e._initialize();
function chunk(data,id){return data.subarray(data.readUInt32LE(id*4),data.readUInt32LE(id*4+4))}
function allocate(data){const p=e.wb_allocate(data.length);new Uint8Array(e.memory.buffer).set(data,p);return p}
function decompress(data){const size=data.readUInt32LE(0),p=allocate(data),out=e.wb_allocate(size);if(e.YJ2_Decompress(p,out,size)!==size)throw Error('original YJ2 failed');return Buffer.from(new Uint8Array(e.memory.buffer,out,size))}
function frame(data,id){const p=allocate(data),address=e.PAL_SpriteGetFrame(p,id),offset=address-p;if(!address||offset<0||offset+4>data.length)throw Error('reference frame is outside the original sprite');return Buffer.from(new Uint8Array(e.memory.buffer,address,data.length-offset))}
const cases=[['mgo.mkf',2,[0,3,6,9]],['mgo.mkf',193,[0,1,2,3]],['gop.mkf',12,[0,8,32,64]],['gop.mkf',20,[0,4,8,12]],['f.mkf',1,[0,1,2,3]],['abc.mkf',1,[0,1,2]],['abc.mkf',2,[0,1,2]],['data.mkf',9,[0,1,2]],['rgm.mkf',7,[-1]],['ball.mkf',20,[-1]]];
const vectors=[];
for(const [name,id,frames]of cases){
 let source=chunk(await fs.readFile(path.join(root,'.local/pal/original-98',name)),id);
 if(!['gop.mkf','data.mkf','rgm.mkf','ball.mkf'].includes(name))source=decompress(source);
 for(const index of frames){
  const bitmap=index<0?source:frame(source,index);let offset=bitmap.readUInt32LE(0)===2?4:0;
  const width=bitmap.readUInt16LE(offset),height=bitmap.readUInt16LE(offset+2),p=allocate(bitmap);
  const a=e.wb_probe_rle(p,width,height,0);const first=Buffer.from(new Uint8Array(e.memory.buffer,a,width*height));
  const b=e.wb_probe_rle(p,width,height,255);const second=Buffer.from(new Uint8Array(e.memory.buffer,b,width*height));
  const result=Buffer.alloc(width*height*2);
  for(let i=0;i<first.length;i++)if(first[i]===second[i]){result[i*2]=first[i];result[i*2+1]=255;}
  vectors.push({archive:name,chunk:id,frame:index,width,height,sha256:createHash('sha256').update(result).digest('hex')});
 }
}
await fs.writeFile(path.join(root,'games/pal/resources/testdata/rle-reference.json'),JSON.stringify({reference:'pinned SDLPAL PAL_RLEBlitToSurface; two-fill transparency observation',runtimeSHA256:createHash('sha256').update(program).digest('hex'),vectors},null,2)+'\n');
console.log('recorded original C raster vectors:',vectors.length);
