import * as T from 'three';

export function mapTextureColorSpace(key: string) {
  return key === 'lightmap' ? T.LinearSRGBColorSpace : T.SRGBColorSpace;
}

export function muzzleAttachments(root: T.Object3D) {
  const attachments: T.Object3D[] = [];
  root.traverse(node => { if (node.userData.attachment === true) attachments.push(node); });
  return attachments;
}

export function createMuzzleTexture() {
  const size = 64;
  const pixels = new Uint8Array(size * size * 4);
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) {
    const dx = (x + 0.5) / size * 2 - 1;
    const dy = (y + 0.5) / size * 2 - 1;
    const radius = Math.hypot(dx, dy);
    const reach = 0.62 + Math.cos(Math.atan2(dy, dx) * 5) * 0.2;
    const strength = Math.max(0, 1 - radius / reach);
    const offset = (y * size + x) * 4;
    pixels[offset] = 255;
    pixels[offset + 1] = Math.round(140 + strength * 100);
    pixels[offset + 2] = Math.round(24 + strength * 160);
    pixels[offset + 3] = Math.round(Math.pow(strength, 0.7) * 255);
  }
  const texture = new T.DataTexture(pixels, size, size, T.RGBAFormat);
  texture.colorSpace = T.SRGBColorSpace;
  texture.magFilter = T.LinearFilter;
  texture.minFilter = T.LinearFilter;
  texture.needsUpdate = true;
  return texture;
}

export class ShotEffects {
  readonly root = new T.Group();
  readonly stats = { tracers: 0, impacts: 0, hits: 0 };
  private ray = new T.Raycaster();
  private direction = new T.Vector3();
  private start = new T.Vector3();
  private end = new T.Vector3();
  private normal = new T.Vector3();
  private serial = 0;
  private slots = Array.from({ length: 16 }, () => {
    const geometry = new T.BufferGeometry();
    geometry.setAttribute('position', new T.BufferAttribute(new Float32Array(6), 3));
    const line = new T.Line(geometry, new T.LineBasicMaterial({ color: 0xffe0a4, transparent: true, opacity: 0.6, depthWrite: false }));
    line.frustumCulled = false;
    const impact = new T.Mesh(new T.RingGeometry(0.8, 2.5, 10), new T.MeshBasicMaterial({ color: 0x231a12, transparent: true, opacity: 0.85, depthWrite: false, side: T.DoubleSide, polygonOffset: true, polygonOffsetFactor: -1 }));
    line.visible = impact.visible = false;
    this.root.add(line, impact);
    return { line, impact, tracerUntil: 0, impactUntil: 0 };
  });

  shoot(camera: T.PerspectiveCamera, targets: T.Object3D, now: number, origin?: T.Vector3) {
    camera.updateMatrixWorld();
    targets.updateWorldMatrix(true, true);
    this.ray.setFromCamera(new T.Vector2(0, 0), camera);
    this.ray.near = 4;
    this.ray.far = 8192;
    const hit = this.ray.intersectObject(targets, true).find(hit => {
      if (!(hit.object as T.Mesh).isMesh) return false;
      for (let object: T.Object3D | null = hit.object; object; object = object.parent) if (!object.visible) return false;
      return true;
    });
    const slot = this.slots[this.serial++ % this.slots.length];
    if (origin) this.start.copy(origin);
    else this.start.set(5, -4, -12).applyMatrix4(camera.matrixWorld);
    camera.getWorldDirection(this.direction);
    this.end.copy(hit?.point ?? this.direction.multiplyScalar(8192).add(camera.position));
    const attribute = slot.line.geometry.getAttribute('position') as T.BufferAttribute;
    attribute.setXYZ(0, this.start.x, this.start.y, this.start.z);
    attribute.setXYZ(1, this.end.x, this.end.y, this.end.z);
    attribute.needsUpdate = true;
    slot.line.visible = true;
    slot.line.material.opacity = 0.65;
    slot.tracerUntil = now + 80;
    this.stats.tracers++;
    if (hit) {
      this.normal.copy(hit.face?.normal ?? this.direction.clone().negate()).transformDirection(hit.object.matrixWorld);
      slot.impact.position.copy(hit.point).addScaledVector(this.normal, 0.15);
      slot.impact.quaternion.setFromUnitVectors(new T.Vector3(0, 0, 1), this.normal);
      slot.impact.visible = true;
      slot.impact.material.opacity = 0.85;
      slot.impactUntil = now + 1600;
      this.stats.impacts++;
    } else slot.impact.visible = false;
  }

  confirmHit() {
    this.stats.hits++;
  }

  update(now: number) {
    for (const slot of this.slots) {
      slot.line.visible = now < slot.tracerUntil;
      slot.impact.visible = now < slot.impactUntil;
      if (slot.line.visible) slot.line.material.opacity = Math.min(0.65, (slot.tracerUntil - now) / 80);
      if (slot.impact.visible) slot.impact.material.opacity = Math.min(0.85, (slot.impactUntil - now) / 400);
    }
  }

  dispose() {
    for (const slot of this.slots) {
      slot.line.geometry.dispose();
      slot.line.material.dispose();
      slot.impact.geometry.dispose();
      slot.impact.material.dispose();
    }
    this.root.clear();
  }
}
