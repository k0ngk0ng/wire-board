import * as T from 'three';
import type { EffectName, EffectSprite, GameEvent, Player } from './types.ts';

type Hitbox = { bone: number; group: number; min: number[]; max: number[] };

export function hitboxBloodPoint(model: T.Object3D, group: number, from?: T.Vector3) {
  let hitboxes: Hitbox[] = [], skeleton: T.Skeleton | undefined;
  model.traverse(object => {
    if (!hitboxes.length && Array.isArray(object.userData.hitboxes) && object.userData.hitboxes.length) hitboxes = object.userData.hitboxes as Hitbox[];
    if ((object as T.SkinnedMesh).isSkinnedMesh && !skeleton) skeleton = (object as T.SkinnedMesh).skeleton;
  });
  if (!skeleton) return;
  model.updateWorldMatrix(true, true);
  const points = hitboxes.filter(box => (group === 0 ? box.group !== 8 : box.group === group) && skeleton?.bones[box.bone]).map(box => {
    const bone = skeleton!.bones[box.bone];
    const center = new T.Vector3((box.min[0] + box.max[0]) / 2, (box.min[1] + box.max[1]) / 2, (box.min[2] + box.max[2]) / 2);
    if (!from) return bone.localToWorld(center);
    const local = bone.worldToLocal(from.clone());
    const surface = new T.Ray(local, center.clone().sub(local).normalize()).intersectBox(new T.Box3(new T.Vector3().fromArray(box.min), new T.Vector3().fromArray(box.max)), new T.Vector3());
    return bone.localToWorld(surface ?? center);
  });
  if (!points.length) return;
  if (from) return points.sort((a, b) => a.distanceToSquared(from) - b.distanceToSquared(from))[0];
  return points.reduce((sum, point) => sum.add(point), new T.Vector3()).multiplyScalar(1 / points.length);
}

function variation(id: number, salt: number) {
  return (Math.imul((id + salt) ^ 0x45d9f3b, 1664525) >>> 0) / 4294967296;
}

export class CombatEffects {
  readonly root = new T.Group();
  private seen = new Set<number>();
  private serial = 0;
  private dropSerial = 0;
  private sparkSerial = 0;
  private counts = { blood: 0, explosions: 0, sparks: 0 };
  private slots = Array.from({ length: 48 }, () => {
    const geometry = new T.BufferGeometry();
    geometry.setAttribute('position', new T.BufferAttribute(new Float32Array(12), 3));
    geometry.setAttribute('uv', new T.BufferAttribute(new Float32Array(8), 2));
    geometry.setIndex([0, 1, 2, 2, 1, 3]);
    const material = new T.MeshBasicMaterial({ transparent: true, depthWrite: false, depthTest: true, side: T.DoubleSide, toneMapped: false });
    const mesh = new T.Mesh(geometry, material);
    mesh.visible = false;
    mesh.frustumCulled = false;
    this.root.add(mesh);
    return { mesh, name: 'blood' as EffectName, since: 0, fps: 30, duration: 0, first: 0, gravity: false, origin: new T.Vector3(), velocity: new T.Vector3() };
  });
  private sparks = Array.from({ length: 8 }, () => {
    const geometry = new T.BufferGeometry();
    geometry.setAttribute('position', new T.BufferAttribute(new Float32Array(22 * 3), 3));
    const material = new T.PointsMaterial({ color: 0x4b4b4b, size: 1.5, transparent: true, depthTest: true, depthWrite: false, blending: T.AdditiveBlending, toneMapped: false });
    const points = new T.Points(geometry, material);
    points.visible = false;
    points.frustumCulled = false;
    this.root.add(points);
    return { points, since: 0, velocity: new Float32Array(22 * 3), origin: new T.Vector3() };
  });

  constructor(private definitions: Record<EffectName, EffectSprite>, private textures: Map<EffectName, T.Texture>) {
    for (const name of ['blood', 'bloodspray', 'fexplo', 'eexplo'] as const) if (!definitions[name] || !textures.has(name)) throw Error(`缺少原版战斗特效：${name}`);
    for (const slot of this.slots) slot.mesh.material.map = textures.get('blood')!;
  }

  get stats() {
    return { ...this.counts, activeBlood: this.slots.filter(slot => slot.mesh.visible && slot.name.startsWith('blood')).length, activeExplosions: this.slots.filter(slot => slot.mesh.visible && !slot.name.startsWith('blood')).length };
  }

  event(event: GameEvent, now: number, target?: Pick<Player, 'visible' | 'life'>, point?: T.Vector3) {
    if (this.seen.has(event.id) || ![event.x, event.y, event.z].every(Number.isFinite)) return;
    if (event.type === 'explode' && event.weapon === 'hegrenade') {
      const origin = new T.Vector3(event.x! / 256, event.z! / 256 + 20, -event.y! / 256);
      this.spawn('fexplo', origin, 2.5, now);
      origin.set(event.x! / 256 + variation(event.id, 2) * 128 - 64, event.z! / 256 + 30 + variation(event.id, 3) * 5, -event.y! / 256 - variation(event.id, 4) * 128 + 64);
      this.spawn('eexplo', origin, 3, now);
      this.counts.explosions++;
    } else if (event.type === 'hit' && (event.fx === 1 || event.fx === 2) && (event.target ?? -1) >= 0 && target?.visible === true && (event.life === undefined || event.life === target.life) && point && event.weapon !== 'hegrenade' && event.weapon !== 'c4') {
      if (event.fx === 2) {
        const spark = this.sparks[this.sparkSerial++ % this.sparks.length];
        spark.origin.copy(point);
        spark.points.position.copy(point);
        spark.since = now;
        spark.points.visible = true;
        const positions = spark.points.geometry.getAttribute('position') as T.BufferAttribute;
        for (let i = 0; i < 22; i++) {
          const angle = variation(event.id, i * 3) * Math.PI * 2;
          const speed = 25 + variation(event.id, i * 3 + 1) * 65;
          spark.velocity[i * 3] = Math.cos(angle) * speed;
          spark.velocity[i * 3 + 1] = variation(event.id, i * 3 + 2) * speed;
          spark.velocity[i * 3 + 2] = Math.sin(angle) * speed;
          positions.setXYZ(i, 0, 0, 0);
        }
        positions.needsUpdate = true;
        this.counts.sparks++;
      } else {
        const size = Math.max(3, Math.min(16, Math.floor(Math.min(255, (event.value ?? 0) * 2) / 10)));
        const spray = this.spawn('bloodspray', point, size / (25 + variation(event.id, 20) * 10), now);
        spray.origin.y += 2 + variation(event.id, 21) * 2;
        spray.mesh.position.copy(spray.origin);
        spray.fps = (this.definitions.bloodspray.frames.length - 1) * 4;
        spray.duration = 250;
        spray.mesh.material.opacity = 250 / 255;
        const drops = size + 2 + Math.floor(variation(event.id, 22) * 8) + Math.floor(variation(event.id, 23) * 8);
        for (let drop = 0; drop < drops; drop++) {
          const slot = this.spawn('blood', point, size / (15 + variation(event.id, drop * 5 + 24) * 10), now);
          slot.gravity = true;
          slot.fps = 0;
          slot.first = 1 + Math.floor(variation(event.id, drop * 5 + 25) * (this.definitions.blood.frames.length - 1));
          slot.duration = 1000 + variation(event.id, drop * 5 + 26) * 2000;
          slot.mesh.material.opacity = 250 / 255;
          slot.velocity.set(variation(event.id, drop * 5) * 191 - 96, variation(event.id, drop * 5 + 1) * 127 - 32, 96 - variation(event.id, drop * 5 + 2) * 191);
          this.frame(slot, slot.first);
        }
        this.counts.blood++;
      }
    } else return;
    this.seen.add(event.id);
    if (this.seen.size > 256) this.seen.delete(this.seen.values().next().value!);
  }

  private spawn(name: EffectName, point: T.Vector3, scale: number, now: number) {
    const slot = this.slots[name === 'blood' ? 16 + this.dropSerial++ % 32 : this.serial++ % 16];
    slot.name = name;
    slot.since = now;
    slot.first = 0;
    slot.fps = 30;
    slot.duration = (this.definitions[name].frames.length - 1) * 1000 / 30;
    slot.gravity = false;
    slot.origin.copy(point);
    slot.velocity.set(0, 0, 0);
    slot.mesh.position.copy(point);
    slot.mesh.scale.setScalar(scale);
    slot.mesh.material.map = this.textures.get(name)!;
    const definition = this.definitions[name];
    slot.mesh.material.blending = definition.blend === 'additive' ? T.AdditiveBlending : T.NormalBlending;
    slot.mesh.material.alphaTest = definition.textureFormat === 3 ? 0.5 : 0;
    const tint = definition.tint ?? [255, 255, 255];
    slot.mesh.material.color.setRGB(tint[0] / 255, tint[1] / 255, tint[2] / 255, T.SRGBColorSpace);
    slot.mesh.material.opacity = 1;
    slot.mesh.material.needsUpdate = true;
    slot.mesh.visible = true;
    slot.mesh.userData.effect = name;
    slot.mesh.userData.sourceSHA256 = definition.sourceSHA256;
    this.frame(slot, 0);
    return slot;
  }

  private frame(slot: typeof this.slots[number], index: number) {
    const definition = this.definitions[slot.name], frame = definition.frames[index];
    const left = frame.origin[0], right = left + frame.width, top = frame.origin[1], bottom = top - frame.height;
    const positions = slot.mesh.geometry.getAttribute('position') as T.BufferAttribute;
    positions.setXYZ(0, left, bottom, 0); positions.setXYZ(1, right, bottom, 0); positions.setXYZ(2, left, top, 0); positions.setXYZ(3, right, top, 0);
    positions.needsUpdate = true;
    const uv = slot.mesh.geometry.getAttribute('uv') as T.BufferAttribute;
    const x = frame.x / definition.width, y = frame.y / definition.height, endX = (frame.x + frame.width) / definition.width, endY = (frame.y + frame.height) / definition.height;
    uv.setXY(0, x, endY); uv.setXY(1, endX, endY); uv.setXY(2, x, y); uv.setXY(3, endX, y);
    uv.needsUpdate = true;
    slot.mesh.userData.frame = index;
  }

  update(now: number, camera: T.PerspectiveCamera) {
    for (const slot of this.slots) {
      if (!slot.mesh.visible) continue;
      const age = Math.max(0, now - slot.since);
      const frame = slot.first + Math.floor(age * slot.fps / 1000);
      if (age >= slot.duration || frame >= this.definitions[slot.name].frames.length) { slot.mesh.visible = false; continue; }
      if (slot.mesh.userData.frame !== frame) this.frame(slot, frame);
      slot.mesh.quaternion.copy(camera.quaternion);
      if (slot.gravity) { slot.mesh.position.copy(slot.origin).addScaledVector(slot.velocity, age / 1000); slot.mesh.position.y -= 200 * age * age / 1000000; }
    }
    for (const spark of this.sparks) {
      if (!spark.points.visible) continue;
      const age = Math.max(0, now - spark.since) / 1000;
      if (age >= 0.35) { spark.points.visible = false; continue; }
      const positions = spark.points.geometry.getAttribute('position') as T.BufferAttribute;
      for (let i = 0; i < 22; i++) positions.setXYZ(i, spark.velocity[i * 3] * age, spark.velocity[i * 3 + 1] * age - 200 * age * age, spark.velocity[i * 3 + 2] * age);
      positions.needsUpdate = true;
      spark.points.material.opacity = Math.max(0, 1 - age / 0.35);
    }
  }

  reset() {
    this.seen.clear();
    for (const slot of this.slots) slot.mesh.visible = false;
    for (const spark of this.sparks) spark.points.visible = false;
  }

  dispose() {
    for (const slot of this.slots) { slot.mesh.geometry.dispose(); slot.mesh.material.dispose(); }
    for (const spark of this.sparks) { spark.points.geometry.dispose(); spark.points.material.dispose(); }
    this.root.clear();
    this.seen.clear();
  }
}
