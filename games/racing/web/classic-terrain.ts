import materialData from "./classic-terrain-materials.json" with { type: "json" };
import wheelData from "./classic-terrain-wheels.json" with { type: "json" };
import type { ClassicRoadSurface } from "./classic-motion.ts";

type Vec3 = [number, number, number];
export type ClassicTerrainContact = { found: boolean; surface: number; height: number; nx: number; ny: number; nz: number; segment: number };
type Triangle = { a: Vec3; b: Vec3; c: Vec3; den: number; nx: number; ny: number; nz: number; surface: number; segment: number };
export type ClassicTerrainWorld = { geometry: string; triangles: Triangle[]; grid: Map<string, number[]> };
export type ClassicTerrainCatalog = { version: number; unit: string; id: string; geometry: string; sourceSha256: string; surfaces: Array<{ segment: number; surface: number; vertices: number[][]; faces: number[][] }> };
type TireCar = { x: number; y: number; height?: number; angle: number; vehicle?: string };
type WheelSpec = { groundOffset: number; wheels: Array<{ front: boolean; position: number[]; radius: number }> };
const CELL = 200;

export function classicTerrainFromCatalog(raw: ClassicTerrainCatalog, track?: { geometry?: string }): ClassicTerrainWorld {
 if (raw.version !== 1 || !["metres","decimetres"].includes(raw.unit) || !/^[a-f0-9]{64}$/.test(raw.geometry) || !/^[a-f0-9]{64}$/.test(raw.sourceSha256) || track && raw.geometry !== track.geometry || !Array.isArray(raw.surfaces)) throw new Error("赛道地表版本无效");
 const triangles: Triangle[] = [], grid = new Map<string, number[]>(), scale = raw.unit === "metres" ? 10 : 1;
 for (const patch of raw.surfaces) {
  if (!Number.isInteger(patch.surface) || patch.surface < 0 || !Array.isArray(patch.vertices) || !Array.isArray(patch.faces)) throw new Error("赛道地表数据无效");
  const vertices = patch.vertices.map(v => { if (v.length !== 3 || v.some(n => !Number.isFinite(n))) throw new Error("赛道地表坐标无效"); return v.map(n => Math.round(n*scale*1e8)/1e8) as Vec3; });
  for (const face of patch.faces) {
   if (face.some(i => !Number.isInteger(i) || i < 0 || i >= vertices.length)) throw new Error("赛道地表索引无效");
   for (let i=1;i+1<face.length;i++) {
    const a=vertices[face[0]],b=vertices[face[i]],c=vertices[face[i+1]],den=(b[2]-c[2])*(a[0]-c[0])+(c[0]-b[0])*(a[2]-c[2]);
    if (Math.abs(den)<1e-8) continue;
    const ux=b[0]-a[0],uy=b[1]-a[1],uz=b[2]-a[2],vx=c[0]-a[0],vy=c[1]-a[1],vz=c[2]-a[2];
    let nx=uy*vz-uz*vy,ny=uz*vx-ux*vz,nz=ux*vy-uy*vx;const length=Math.hypot(nx,ny,nz);if(length<1e-8)continue;
    if(ny<0){nx=-nx;ny=-ny;nz=-nz;}
    const index=triangles.length;triangles.push({a,b,c,den,nx:nx/length,ny:ny/length,nz:nz/length,surface:patch.surface,segment:patch.segment});
    for(let x=Math.floor(Math.min(a[0],b[0],c[0])/CELL);x<=Math.floor(Math.max(a[0],b[0],c[0])/CELL);x++)for(let z=Math.floor(Math.min(a[2],b[2],c[2])/CELL);z<=Math.floor(Math.max(a[2],b[2],c[2])/CELL);z++){
     const key=`${x},${z}`,ids=grid.get(key)??[];ids.push(index);grid.set(key,ids);
    }
   }
  }
 }
 return {geometry:raw.geometry,triangles,grid};
}

export function classicTerrainAt(world: ClassicTerrainWorld | undefined, x: number, z: number, heightHint: number): ClassicTerrainContact {
 const out: ClassicTerrainContact = {found:false,surface:-1,height:heightHint,nx:0,ny:1,nz:0,segment:-1};
 if(!world || !Number.isFinite(x) || !Number.isFinite(z) || !Number.isFinite(heightHint))return out;
 let best=Infinity;
 for(const index of world.grid.get(`${Math.floor(x/CELL)},${Math.floor(z/CELL)}`)??[]){
  const t=world.triangles[index];if(t.surface===0||t.ny<.1)continue;
  const u=((t.b[2]-t.c[2])*(x-t.c[0])+(t.c[0]-t.b[0])*(z-t.c[2]))/t.den,v=((t.c[2]-t.a[2])*(x-t.c[0])+(t.a[0]-t.c[0])*(z-t.c[2]))/t.den,w=1-u-v;
  if(u< -1e-7||v< -1e-7||w< -1e-7)continue;
  const height=u*t.a[1]+v*t.b[1]+w*t.c[1],distance=Math.abs(height-heightHint);
  if(distance>=best-1e-7)continue;
  best=distance;Object.assign(out,{found:true,surface:t.surface,height,nx:t.nx,ny:t.ny,nz:t.nz,segment:t.segment});
 }
 return out;
}

/** Original factor table; relative dry-road normalization of the cited formula. */
export function classicMaterialGrip(surface: number): number {
 const factor=Number.isInteger(surface)&&surface>=0?materialData.surfaceFactors[surface]:undefined;
 if(typeof factor!=="number" || !Number.isFinite(factor))return 1;
 const k=(factor+1)/2;
 return k*(1+.25*k)/1.25;
}
export function classicTerrainSurface(world: ClassicTerrainWorld | undefined, car: TireCar, base: ClassicRoadSurface): ClassicRoadSurface {
 const vehicles=wheelData.vehicles as Record<string,WheelSpec>,id=car.vehicle??"p911",body=Object.hasOwn(vehicles,id)?vehicles[id]:vehicles.p911;
 const cs=Math.cos(car.angle),sn=Math.sin(car.angle);let front=0,rear=0,nFront=0,nRear=0,nx=0,ny=0,nz=0,hits=0;
 for(const wheel of body.wheels){
  const [lx,ly,lz]=wheel.position,x=car.x+lz*cs+lx*sn,z=car.y+lz*sn-lx*cs,height=(car.height??0)+body.groundOffset+ly-wheel.radius;
  const hit=classicTerrainAt(world,x,z,height),grip=hit.found?classicMaterialGrip(hit.surface):1;
  if(hit.found){nx+=hit.nx;ny+=hit.ny;nz+=hit.nz;hits++;}
  if(wheel.front){front+=grip;nFront++;}else{rear+=grip;nRear++;}
 }
 const result={...base,grip:1,resistance:0,frontGrip:nFront?front/nFront:1,rearGrip:nRear?rear/nRear:1};
 const length=Math.hypot(nx,ny,nz);
 if(hits&&length>1e-8){nx/=length;ny/=length;nz/=length;result.normalY=Math.abs(ny);result.grade=Math.atan(-(nx*cs+nz*sn)/Math.max(.1,ny));}
 return result;
}
