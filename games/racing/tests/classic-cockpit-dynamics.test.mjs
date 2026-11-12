import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "../../../web/node_modules/typescript/lib/typescript.js";
import * as T from "../../../web/node_modules/three/build/three.module.js";

const source=readFileSync(new URL("../web/classic-cockpit-dynamics.ts",import.meta.url),"utf8");
const output=ts.transpileModule(source,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.CommonJS}}).outputText;
const exports={};new Function("require","exports",output)(name=>{if(name==="three")return T;throw Error(name);},exports);
const {ClassicCockpitDynamics}=exports;
const config={steeringRatio:14,maxRPM:8000,maxSpeedKmh:320};
const ids=["p911","fm50","ldia","mcf1","ccor"];
function native(id){
  const bytes=readFileSync(new URL(`../assets/public/racing/v3/cars/${id}.glb`,import.meta.url));
  const n=bytes.readUInt32LE(12),doc=JSON.parse(bytes.subarray(20,20+n)),blob=bytes.subarray(28+n);
  function values(index){const a=doc.accessors[index],v=doc.bufferViews[a.bufferView],size={VEC2:2,VEC3:3,SCALAR:1}[a.type];
    const offset=(v.byteOffset??0)+(a.byteOffset??0),data=new DataView(blob.buffer,blob.byteOffset+offset);
    return Float32Array.from({length:a.count*size},(_,i)=>data.getFloat32(i*4,true));}
  const cockpit=new T.Group(),original=[];
  for(const node of doc.nodes){const part=node.extras?.originalPart;if(!part)continue;
    const primitive=doc.meshes[node.mesh].primitives[0],geometry=new T.BufferGeometry();
    geometry.setAttribute("position",new T.BufferAttribute(values(primitive.attributes.POSITION),3));
    geometry.setAttribute("uv",new T.BufferAttribute(primitive.attributes.TEXCOORD_0===undefined?new Float32Array():values(primitive.attributes.TEXCOORD_0),2));
    const mesh=new T.Mesh(geometry,new T.MeshBasicMaterial());mesh.name=node.name;mesh.userData={...node.extras};mesh.position.fromArray(node.translation??[0,0,0]);cockpit.add(mesh);
    original.push({mesh,parent:cockpit,position:mesh.position.clone(),geometry,material:mesh.material,vertices:[...geometry.attributes.position.array],uv:[...geometry.attributes.uv.array]});
  }
  cockpit.updateMatrixWorld(true);return {cockpit,original};
}
function worldVertices(mesh){mesh.updateWorldMatrix(true,false);const p=mesh.geometry.attributes.position;return Array.from({length:p.count},(_,i)=>new T.Vector3().fromBufferAttribute(p,i).applyMatrix4(mesh.matrixWorld));}
function near(a,b,epsilon=1e-7){assert(a.distanceTo(b)<epsilon,`${a.toArray()} != ${b.toArray()}`);}

test("all five original cockpit meshes bind native wheel and needles without replacing geometry, UVs or materials",()=>{
  for(const id of ids){const {cockpit,original}=native(id),before=original.map(x=>worldVertices(x.mesh));
    const dynamics=new ClassicCockpitDynamics(cockpit,config);dynamics.update({steeringAngle:0,speedKmh:0,rpm:0});
    const rows=dynamics.inspect();assert.equal(rows.filter(x=>x.kind==="steering").length,1,id);assert.equal(rows.filter(x=>x.kind==="rpm").length,1,id);assert.equal(rows.filter(x=>x.kind==="speed").length,id==="mcf1"?0:1,id);
    original.forEach((o,i)=>{assert.equal(o.mesh.geometry,o.geometry);assert.equal(o.mesh.material,o.material);assert.deepEqual([...o.geometry.attributes.position.array],o.vertices);assert.deepEqual([...o.geometry.attributes.uv.array],o.uv);worldVertices(o.mesh).forEach((p,j)=>near(p,before[i][j]));});
  }
});
test("actual steering angle moves the original wheel around its thin geometric axis and returns exactly",()=>{
  const {cockpit}=native("p911"),d=new ClassicCockpitDynamics(cockpit,config),wheel=cockpit.children.find(n=>n.userData.originalPart==="steering")??cockpit.getObjectByName("steering");
  const start=worldVertices(wheel);d.update({steeringAngle:.1,speedKmh:0,rpm:0});
  const row=d.inspect().find(x=>x.kind==="steering");assert.ok(Math.abs(row.angle-1.4)<1e-9);assert(Math.abs(row.axis[2])>.85);
  assert(worldVertices(wheel).some((p,i)=>p.distanceTo(start[i])>.02));
  d.update({steeringAngle:0,speedKmh:0,rpm:0});worldVertices(wheel).forEach((p,i)=>near(p,start[i]));
});
test("needle roots remain fixed on their original plane even when part origins are tens of centimetres away",()=>{
  for(const id of ids){const {cockpit}=native(id),d=new ClassicCockpitDynamics(cockpit,config);const zero=d.inspect();
    d.update({steeringAngle:0,speedKmh:160,rpm:4000});
    for(const row of d.inspect().filter(r=>r.kind!=="steering")){
      const first=zero.find(x=>x.part===row.part);assert.deepEqual(row.pivot,first.pivot);assert(row.angle>0);assert(row.angle<=Math.PI*2);
      const mesh=cockpit.getObjectByName(row.node),normal=new T.Vector3(...row.axis),pivot=new T.Vector3(...row.pivot);
      for(const p of worldVertices(mesh))assert(Math.abs(p.clone().sub(pivot).dot(normal))<1e-6,id+" "+row.part);
    }
  }
});
test("RPM and speed needles follow separate telemetry, clamp bad reads, and do not mutate day/night visibility",()=>{
  const {cockpit}=native("fm50"),d=new ClassicCockpitDynamics(cockpit,config);const visibility=new Map();cockpit.traverse(n=>visibility.set(n.uuid,n.visible));
  d.update({steeringAngle:0,speedKmh:160,rpm:2000});const first=d.inspect();
  d.update({steeringAngle:0,speedKmh:160,rpm:6000});const second=d.inspect();
  assert.equal(first.find(x=>x.kind==="speed").angle,second.find(x=>x.kind==="speed").angle);assert(second.find(x=>x.kind==="rpm").angle>first.find(x=>x.kind==="rpm").angle);
  d.update({steeringAngle:NaN,speedKmh:Infinity,rpm:-1});assert(d.inspect().every(x=>Number.isFinite(x.angle)));
  cockpit.traverse(n=>{if(visibility.has(n.uuid))assert.equal(n.visible,visibility.get(n.uuid));});
  d.dispose();assert(cockpit.children.every(n=>!n.name.startsWith("classic_cockpit_joint")));
});
