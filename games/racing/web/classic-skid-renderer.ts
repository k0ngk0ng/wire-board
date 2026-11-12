import * as T from "three";
import { ClassicSkidTrail, classicSkidStrength, type ClassicSkidContact, type ClassicSkidSegment, type ClassicSkidStrength } from "./classic-skids";
import type { Car } from "./types";

/** One fixed geometry for every car; expiry is shader time, never new meshes. */
export class ClassicSkidRenderer {
  private readonly trail = new ClassicSkidTrail();
  private readonly geometry = new T.BufferGeometry();
  private readonly material: T.ShaderMaterial;
  private readonly mesh: T.Mesh;
  private readonly positions = new Float32Array(this.trail.capacity * 18);
  private readonly births = new Float32Array(this.trail.capacity * 6);
  private readonly strengths = new Float32Array(this.trail.capacity * 6);
  private readonly causes: Array<{ seat: number } & ClassicSkidStrength> = [];
  private latest?: ClassicSkidSegment;
  private reduced = false;
  private dirty = false;
  constructor(scene: T.Scene) {
    const uvs = new Float32Array(this.trail.capacity * 12);
    for (let i = 0; i < this.trail.capacity; i++) uvs.set([0,0,1,0,0,1,0,1,1,0,1,1], i*12);
    this.geometry.setAttribute("position", new T.BufferAttribute(this.positions,3).setUsage(T.DynamicDrawUsage));
    this.geometry.setAttribute("skidBirth", new T.BufferAttribute(this.births,1).setUsage(T.DynamicDrawUsage));
    this.geometry.setAttribute("skidStrength", new T.BufferAttribute(this.strengths,1).setUsage(T.DynamicDrawUsage));
    this.geometry.setAttribute("uv", new T.BufferAttribute(uvs,2));
    this.geometry.setDrawRange(0,0);
    this.material = new T.ShaderMaterial({
      uniforms: T.UniformsUtils.merge([T.UniformsLib.fog, {
        skidNow: { value: 0 }, skidLife: { value: this.trail.lifetime/1000 },
      }]),
      vertexShader: `#include <common>
        #include <fog_pars_vertex>
        attribute float skidBirth,skidStrength; uniform float skidNow,skidLife;
        varying vec2 skidUV; varying float skidAlpha;
        void main(){skidUV=uv;float age=max(0.0,skidNow-skidBirth);
          skidAlpha=skidStrength*clamp((skidLife-age)/(skidLife*.625),0.0,1.0);
          vec4 mvPosition=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mvPosition;
          #include <fog_vertex>
        }`,
      fragmentShader: `#include <common>
        #include <fog_pars_fragment>
        varying vec2 skidUV; varying float skidAlpha;
        void main(){float edge=smoothstep(0.0,.1,skidUV.x)*(1.0-smoothstep(.9,1.0,skidUV.x));
          float tread=.8+.2*step(.3,fract(skidUV.x*9.0));float alpha=skidAlpha*edge*tread*.66;
          if(alpha<.005)discard;gl_FragColor=vec4(.025,.029,.033,alpha);
          #include <fog_fragment>
          #include <colorspace_fragment>
        }`,
      transparent: true, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -1,
      polygonOffsetUnits: -1, fog: true,
    });
    this.mesh = new T.Mesh(this.geometry,this.material);
    this.mesh.name = "classic_tire_marks"; this.mesh.frustumCulled = false; this.mesh.renderOrder = 2;
    scene.add(this.mesh);
  }
  begin(now: number, reduced: boolean) {
    this.causes.length = 0; this.dirty = false;
    this.material.uniforms.skidNow.value = now/1000;
    if (reduced && !this.reduced) { this.trail.clear(); this.geometry.setDrawRange(0,0); this.latest = undefined; }
    this.reduced = reduced;
  }
  sample(car: Car, contacts: readonly ClassicSkidContact[], now: number, serverNow: number) {
    const strength = classicSkidStrength(car,serverNow);
    if (!strength.front && !strength.rear) this.trail.remove(car.seat);
    if (strength.front || strength.rear) this.causes.push({ seat: car.seat, ...strength });
    for (const mark of this.trail.sample(car.seat,contacts,strength,now,this.reduced)) {
      this.positions.set(mark.vertices,mark.slot*18);
      this.births.fill(mark.born/1000,mark.slot*6,mark.slot*6+6);
      this.strengths.fill(mark.strength,mark.slot*6,mark.slot*6+6);
      (this.geometry.getAttribute("position") as T.BufferAttribute).addUpdateRange(mark.slot*18,18);
      (this.geometry.getAttribute("skidBirth") as T.BufferAttribute).addUpdateRange(mark.slot*6,6);
      (this.geometry.getAttribute("skidStrength") as T.BufferAttribute).addUpdateRange(mark.slot*6,6);
      this.latest = mark; this.dirty = true;
    }
    this.geometry.setDrawRange(0,Math.min(this.trail.capacity,this.trail.written)*6);
  }
  finish(canvas: HTMLCanvasElement, now: number) {
    if (this.dirty) for (const name of ["position","skidBirth","skidStrength"]) this.geometry.getAttribute(name).needsUpdate = true;
    const live = this.trail.live(now);
    canvas.dataset.skidSegments = String(live.length);
    canvas.dataset.skidWritten = String(this.trail.written);
    canvas.dataset.skidCauses = JSON.stringify(this.causes);
    if (this.latest) canvas.dataset.skidSample = JSON.stringify(this.latest); else delete canvas.dataset.skidSample;
    this.mesh.visible = live.length > 0 && !this.reduced;
  }
  remove(seat: number) { this.trail.remove(seat); }
  dispose() { this.mesh.removeFromParent(); this.geometry.dispose(); this.material.dispose(); this.trail.clear(); this.latest = undefined; }
}
