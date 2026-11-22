import * as T from 'three';
import { clone } from 'three/examples/jsm/utils/SkeletonUtils.js';
import type { GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js';
import type { LoadedAssets } from './assets.ts';
import type { GameEvent, Player, View } from './types.ts';
import { eventFeedback, type FeedbackKind } from './feedback.ts';
import { eyeOffset, smokeRadius, viewModelHand, viewModelVerticalFov, visibleVelocity } from './presentation.ts';
import { carriedBoneMap, playerPose } from './model-pose.ts';
import { createMuzzleTexture, mapTextureColorSpace, muzzleAttachments, ShotEffects } from './shot-effects.ts';
import { CorpseInterpolation, corpseAge, corpseTravel, deathAnimation, deathView, scopeFov, ViewImpulse, weaponAnimation } from './view-effects.ts';
import { MapFlashlight } from './flashlight.ts';

type Actor = { root: T.Group; model: T.Object3D; mixer: T.AnimationMixer; clips: T.AnimationClip[]; animation: string; weapon: string; carried?: T.Object3D; carriedBones?: T.Bone[]; carriedSkeletons?: T.Skeleton[]; alive: boolean; life: number; deathAt: number; corpseMotion: CorpseInterpolation; settled?: T.Vector3 };
type Frame = { view: View; at: number };

export class BattleRenderer {
  readonly renderer: T.WebGLRenderer;
  private scene = new T.Scene();
  private world = new T.Group();
  private camera = new T.PerspectiveCamera(75, 1, 1, 16000);
  private weaponCamera = new T.PerspectiveCamera(90, 1, 4, 200);
  private weaponScene = new T.Scene();
  private weaponRoot = new T.Group();
  private weapon?: Actor;
  private weaponID = '';
  private weaponSilenced = false;
  private actors = new Map<number, Actor>();
  private objects = new Map<string, T.Object3D>();
  private brushes = new Map<number, { root: T.Group; origin: T.Vector3; move: T.Vector3; kind: string; angle: number; flags: number }>();
  private decorationMixers: T.AnimationMixer[] = [];
  private smoke = new Map<number, T.Mesh>();
  private assets: LoadedAssets;
  private frames: Frame[] = [];
  private last = 0;
  private muzzleUntil = 0;
  private muzzle: T.Sprite;
  private effects = new ShotEffects();
  private pendingShots = 0;
  private muzzleNodes: T.Object3D[] = [];
  private weaponShots = 0;
  private meleeSwings = 0;
  private pendingFeedback?: { kind: FeedbackKind; serial: number; contact: string };
  private viewImpulse = new ViewImpulse();
  private viewIdentity = '';
  private flashlight = new MapFlashlight();
  private nightVision = false;
  private flashlightDirection = new T.Vector3();
  private mapMeshes: T.Mesh[] = [];
  private corpseRay = new T.Raycaster();
  private corpseFrom = new T.Vector3();
  private corpseTo = new T.Vector3();
  private corpseDirection = new T.Vector3();
  private collisionDirty = true;
  private presentationFrame = 0;
  private shotOrigin = new T.Vector3();
  private disposed = false;
  private width = 0;
  private height = 0;
  private ratio = 0;
  private mapLighting?: T.Texture;
  private textures: T.Texture[] = [];
  private resources: (T.BufferGeometry | T.Material)[] = [];
  private observedSeat = -1;

  constructor(canvas: HTMLCanvasElement, assets: LoadedAssets) {
    this.assets = assets;
    this.renderer = new T.WebGLRenderer({ canvas, antialias: true, alpha: false, powerPreference: 'high-performance' });
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    this.renderer.outputColorSpace = T.SRGBColorSpace;
    this.renderer.toneMapping = T.NoToneMapping;
    this.renderer.autoClear = false;
    this.scene.background = new T.Color('#82949a');
    this.world.rotation.x = -Math.PI / 2;
    this.scene.add(this.world);
    this.scene.add(this.effects.root);
    this.world.add(new T.HemisphereLight(0xffffff, 0x9b8f72, 2));
    const lightmap = this.texture('lightmap', false);
    this.mapLighting = lightmap;
    if (lightmap) lightmap.channel = 1;
    const materials = new Map<string, T.MeshBasicMaterial>();
    for (const mesh of assets.map.meshes) {
      const materialKey = `${mesh.texture}:${mesh.renderMode ?? 0}:${mesh.alpha ?? 255}`;
      let material = materials.get(materialKey);
      if (!material) {
        const map = this.texture(`texture:${mesh.texture}`, true);
        if (!map) throw Error(`地图纹理未解码：${mesh.texture}`);
        const transparent = !!mesh.renderMode && mesh.renderMode !== 4;
        material = new T.MeshBasicMaterial({ map, lightMap: lightmap, lightMapIntensity: 1, side: T.DoubleSide, transparent, opacity: transparent ? (mesh.alpha ?? 255) / 255 : 1, depthWrite: !transparent, alphaTest: mesh.transparent || mesh.renderMode === 4 ? 0.5 : 0 });
        this.flashlight.apply(material);
        materials.set(materialKey, material);
        this.resources.push(material);
      }
      const geometry = new T.BufferGeometry();
      geometry.setAttribute('position', new T.Float32BufferAttribute(mesh.positions, 3));
      geometry.setAttribute('normal', new T.Float32BufferAttribute(mesh.normals, 3));
      geometry.setAttribute('uv', new T.Float32BufferAttribute(mesh.uv, 2));
      geometry.setAttribute('uv1', new T.Float32BufferAttribute(mesh.uv2, 2));
      geometry.computeBoundingSphere();
      geometry.computeBoundingBox();
      const object = new T.Mesh(geometry, material);
      this.mapMeshes.push(object);
      if (mesh.origin) object.position.fromArray(mesh.origin);
      const rules = assets.rules.brushes as { id: number; origin: { x: number; y: number; z: number }; move: { x: number; y: number; z: number }; kind: string; angle: number; flags: number }[] | undefined;
      const brush = rules?.find(brush => brush.id === mesh.brush);
      if (brush) {
        let dynamic = this.brushes.get(brush.id);
        if (!dynamic) {
          const origin = new T.Vector3(brush.origin.x / 256, brush.origin.y / 256, brush.origin.z / 256);
          const pivot = new T.Group();
          pivot.position.copy(origin);
          this.world.add(pivot);
          dynamic = { root: pivot, origin, move: new T.Vector3(brush.move.x / 256, brush.move.y / 256, brush.move.z / 256), kind: brush.kind, angle: brush.angle, flags: brush.flags };
          this.brushes.set(brush.id, dynamic);
        }
        object.position.sub(dynamic.origin);
        dynamic.root.add(object);
      } else this.world.add(object);
      this.resources.push(geometry);
    }
    const sky = [0, 1, 2, 3, 5, 4].map(index => assets.images.get(`sky:${index}`));
    if (sky.every(image => image)) {
      const cube = new T.CubeTexture(sky);
      cube.colorSpace = T.SRGBColorSpace;
      cube.needsUpdate = true;
      this.scene.background = cube;
      this.textures.push(cube);
    }
    for (const decoration of assets.map.decorations ?? []) {
      const asset = assets.models.get(decoration.model);
      if (!asset) throw Error(`原版装饰模型未加载：${decoration.model}`);
      const actor = this.actor(asset);
      actor.root.position.fromArray(decoration.position);
      actor.root.rotation.set(decoration.angles[2] * Math.PI / 180, decoration.angles[0] * Math.PI / 180, decoration.angles[1] * Math.PI / 180, 'ZYX');
      this.world.add(actor.root);
      const clip = asset.animations[decoration.sequence];
      if (clip) { actor.mixer.stopAllAction(); actor.mixer.clipAction(clip).play(); }
      this.decorationMixers.push(actor.mixer);
    }
    this.weaponRoot.quaternion.setFromRotationMatrix(new T.Matrix4().makeBasis(new T.Vector3(0, 0, -1), new T.Vector3(-1, 0, 0), new T.Vector3(0, 1, 0)));
    this.weaponScene.add(this.weaponRoot);
    this.weaponScene.add(new T.HemisphereLight(0xffffff, 0x958c79, 2.5));
    const muzzleTexture = createMuzzleTexture();
    this.textures.push(muzzleTexture);
    this.renderer.initTexture(muzzleTexture);
    const muzzleMaterial = new T.SpriteMaterial({ map: muzzleTexture, color: '#fff2ac', opacity: 0.9, transparent: true, depthTest: false, depthWrite: false, blending: T.AdditiveBlending });
    this.muzzle = new T.Sprite(muzzleMaterial);
    this.muzzle.position.set(3.8, -4, -26);
    this.muzzle.scale.set(4, 4, 1);
    this.weaponScene.add(this.muzzle);
    this.resources.push(muzzleMaterial);
  }

  update(view: View, at: number) {
    const previous = this.frames.at(-1);
    if (previous?.view.epoch !== view.epoch || previous.view.frame > view.frame || previous.view.round !== view.round) { this.frames = []; this.viewImpulse.reset(); this.viewIdentity = ''; this.pendingFeedback = undefined; this.presentationFrame = view.frame; }
    if (previous?.view.frame === view.frame && previous.view.eventId === view.eventId) return;
    this.frames.push({ view, at });
    if (this.frames.length > 6) this.frames.shift();
  }

  async prepare() {
    await this.renderer.compileAsync(this.scene, this.camera);
    await this.renderer.compileAsync(this.weaponScene, this.weaponCamera);
  }

  get visualStats() {
    return { ...this.effects.stats, muzzle: this.muzzle.visible, muzzleAttachmentCount: this.muzzleNodes.length, lightmapColorSpace: this.mapLighting?.colorSpace, cameraFov: this.camera.fov, weaponAnimation: this.weapon?.animation, weaponShots: this.weaponShots, weaponBack: this.weaponRoot.position.z, cameraPitch: Math.asin(-this.flashlightDirection.y), flashlight: this.flashlight.uniforms.csFlashPower.value, nightVision: this.nightVision, corpses: [...this.actors.values()].filter(actor => !actor.alive && actor.root.visible).length };
  }

  shoot(now: number) {
    this.feedback('shot', now);
  }

  feedback(kind: FeedbackKind, now: number) {
    if (kind === 'shot') { this.muzzleUntil = now + 55; this.pendingShots = Math.min(4, this.pendingShots + 1); this.weaponShots++; this.viewImpulse.shot(now); }
    const serial = kind === 'shot' ? this.weaponShots - 1 : kind === 'slash' ? this.meleeSwings++ : 0;
    this.pendingFeedback = { kind, serial, contact: 'air' };
  }

  event(event: GameEvent, now: number, localSeat: number) {
    if (event.type === 'land' && (event.seat === localSeat || event.seat === this.observedSeat)) this.viewImpulse.land(event.value ?? 0, now);
    if ((event.type === 'knife_hit' || event.type === 'knife_wall') && event.value === 1 && (event.seat === localSeat || event.seat === this.observedSeat)) { this.pendingFeedback = { kind: 'stab', serial: 0, contact: 'hit' }; return; }
    if (event.type === 'hit' && event.seat === localSeat) this.effects.confirmHit();
    const actor = this.actors.get(event.seat);
    if (event.type === 'shot') {
      if (actor) this.animate(actor, /shoot|aim_/i, true);
    }
    if (event.seat === localSeat || event.seat === this.observedSeat) {
      const kind = eventFeedback(event);
      if (kind) this.feedback(kind, now);
    }
  }

  draw(view: View, local: Player | undefined, seat: number, now: number, observerTeam?: string) {
    if (this.disposed) return;
    const canvas = this.renderer.domElement;
    const width = Math.max(1, canvas.clientWidth);
    const height = Math.max(1, canvas.clientHeight);
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    if (width !== this.width || height !== this.height || ratio !== this.ratio) {
      this.width = width;
      this.height = height;
      this.ratio = ratio;
      if (this.renderer.getPixelRatio() !== ratio) this.renderer.setPixelRatio(ratio);
      this.renderer.setSize(width, height, false);
      this.camera.aspect = width / height;
      this.weaponCamera.aspect = width / height;
      this.weaponCamera.fov = viewModelVerticalFov(width / height);
      this.camera.updateProjectionMatrix();
      this.weaponCamera.updateProjectionMatrix();
    }
    const dt = this.last ? Math.min(0.05, Math.max(0, (now - this.last) / 1000)) : 0;
    this.last = now;
    for (const mixer of this.decorationMixers) mixer.update(dt);
    const displayed = this.remote(now);
    const received = this.frames.at(-1);
    const elapsed = Math.max(0, Math.min(150, Math.round(now - (received?.at ?? now))));
    const visualFrame = Math.max(this.presentationFrame, view.frame + elapsed / 10);
    this.presentationFrame = visualFrame;
    this.collisionDirty = true;
    const active = new Set<number>();
    for (const source of displayed) {
      if (source.visible === false || !Number.isFinite(source.x)) continue;
      active.add(source.seat);
      let actor = this.actors.get(source.seat);
      if (!actor) {
        const asset = this.assets.models.get(`player-${source.character}`);
        if (!asset) throw Error(`角色模型未加载：${source.character}`);
        actor = this.actor(asset);
        this.actors.set(source.seat, actor);
        this.world.add(actor.root);
      }
      const player = source.seat === seat && local ? local : source;
      const renewed = actor.life !== source.life;
      if (renewed) { actor.life = source.life; actor.deathAt = 0; actor.settled = undefined; actor.corpseMotion.reset(); }
      actor.root.position.set(player.x / 256, player.y / 256, player.z / 256);
      actor.root.rotation.z = player.yaw * Math.PI * 2 / 65536;
      actor.root.visible = source.seat !== this.observedSeat;
      if (actor.alive !== source.alive || renewed) {
        actor.alive = source.alive;
        if (source.alive) this.animate(actor, /^(?:idle1?|idle_aim_)/i);
        else {
          actor.deathAt = source.deathAt ?? 0;
          const name = deathAnimation(actor.clips.map(clip => clip.name), source.deathHitgroup ?? 0, source.crouch);
          if (name) this.animate(actor, new RegExp(`^${name}$`, 'i'), true);
        }
      } else if (source.alive) {
        const moving = Math.abs(player.vx ?? 0) + Math.abs(player.vy ?? 0) > 1500;
        this.animate(actor, moving ? player.crouch ? /^crouchrun$/i : /^run$/i : new RegExp(`^${playerPose(source.weapon, player.crouch)}$`, 'i'));
      }
      if (actor.weapon !== source.weapon) this.carried(actor, source.weapon);
      if (actor.carried) actor.carried.visible = source.alive;
      if (source.alive) actor.mixer.update(dt);
      else {
        const age = corpseAge(visualFrame, actor.deathAt);
        actor.root.position.copy(this.corpsePosition(player, actor, age));
        const clip = actor.clips.find(clip => clip.name === actor.animation);
        if (clip) { const action = actor.mixer.clipAction(clip); action.enabled = true; action.paused = true; action.time = Math.min(clip.duration, age / 1000); actor.mixer.update(0); }
      }
    }
    for (const [id, actor] of this.actors) if (!active.has(id)) actor.root.visible = false;
    const observable = displayed.filter(player => player.visible !== false && Number.isFinite(player.x) && (!observerTeam || player.team === observerTeam));
    const own = view.players.find(player => player.seat === view.you);
    const deadView = own && !own.alive ? deathView(Math.max(1, corpseAge(visualFrame, own.deathAt)), !observable.some(player => player.alive)) : undefined;
    const cameraPlayer = deadView ? own : local ?? observable.find(player => player.seat === seat && player.alive) ?? observable.find(player => player.alive) ?? observable[0];
    this.observedSeat = cameraPlayer?.seat ?? -1;
    if (cameraPlayer && Number.isFinite(cameraPlayer.x)) {
      const identity = `${cameraPlayer.seat}:${cameraPlayer.life}`;
      if (this.viewIdentity !== identity) { this.viewImpulse.reset(); this.viewIdentity = identity; }
      const impulse = this.viewImpulse.sample(now);
      const yaw = (cameraPlayer.yaw + (cameraPlayer.recoilYaw ?? 0)) * Math.PI * 2 / 65536;
      const pitch = (cameraPlayer.pitch + (cameraPlayer.recoilPitch ?? 0)) * Math.PI * 2 / 65536;
      const eye = deadView?.eye ?? eyeOffset(cameraPlayer.crouch);
      this.camera.position.set(cameraPlayer.x / 256, cameraPlayer.z / 256 + eye, -cameraPlayer.y / 256);
      this.camera.lookAt(this.camera.position.x + Math.cos(yaw) * Math.cos(pitch), this.camera.position.y - Math.sin(pitch), this.camera.position.z - Math.sin(yaw) * Math.cos(pitch));
      this.camera.rotateZ((deadView?.roll ?? 0) + impulse.roll);
      this.camera.getWorldDirection(this.flashlightDirection);
      const nightVision = cameraPlayer.alive && !!cameraPlayer.nightVisionOwned && !!cameraPlayer.nightVision;
      if (this.nightVision !== nightVision) { this.nightVision = nightVision; canvas.classList.toggle('cs16-night-vision', nightVision); }
      this.flashlight.update(this.camera.position, this.flashlightDirection, cameraPlayer.alive && !!cameraPlayer.flashlight && (cameraPlayer.flashlightCharge ?? 100) > 0, nightVision);
      this.weaponRoot.position.set(0, impulse.weaponUp, impulse.weaponBack);
      const weaponState = cameraPlayer.inventory?.find(item => item.id === cameraPlayer.weapon);
      this.weaponSilenced = weaponState?.silenced ?? false;
      const zoom = weaponState?.zoom ?? 0;
      const fov = scopeFov(zoom);
      if (this.camera.fov !== fov) { this.camera.fov = fov; this.camera.updateProjectionMatrix(); }
      this.setWeapon(cameraPlayer.weapon);
      if (this.weapon) {
        const feedback = this.pendingFeedback;
        if (feedback) {
          const name = weaponAnimation(this.weapon.clips.map(clip => clip.name), this.weaponID, feedback.kind, feedback.serial, this.weaponSilenced, feedback.contact);
          if (name) this.animate(this.weapon, new RegExp(`^${name}$`, 'i'), true);
          if (feedback.kind === 'shot') this.weaponShots = Math.max(this.weaponShots, feedback.serial + 1);
          if (feedback.kind === 'slash') this.meleeSwings = Math.max(this.meleeSwings, feedback.serial + 1);
          this.pendingFeedback = undefined;
        }
        this.weapon.root.visible = cameraPlayer.alive && !zoom;
        this.weapon.mixer.update(dt);
      }
    }
    const hidden = this.actors.get(this.observedSeat);
    if (hidden) hidden.root.visible = false;
    this.dynamic(view);
    this.effects.update(now);
    if ((this.pendingShots || now < this.muzzleUntil) && this.muzzleNodes.length) this.muzzleNodes[this.weaponShots % this.muzzleNodes.length].getWorldPosition(this.muzzle.position);
    while (this.pendingShots > 0) {
      this.pendingShots--;
      if (!cameraPlayer?.alive) continue;
      if (this.muzzleNodes.length) {
        this.camera.updateMatrixWorld();
        this.weaponCamera.updateMatrixWorld();
        this.shotOrigin.copy(this.muzzle.position).project(this.weaponCamera);
        this.shotOrigin.z = 0.5;
        this.shotOrigin.unproject(this.camera);
        this.effects.shoot(this.camera, this.world, now, this.shotOrigin);
      } else this.effects.shoot(this.camera, this.world, now);
    }
    this.muzzle.visible = now < this.muzzleUntil && !!cameraPlayer?.alive && !this.weaponSilenced;
    this.renderer.clear();
    this.renderer.render(this.scene, this.camera);
    if (this.weapon?.root.visible) {
      this.renderer.clearDepth();
      this.renderer.render(this.weaponScene, this.weaponCamera);
    }
    return cameraPlayer;
  }

  private texture(key: string, repeat: boolean) {
    const image = this.assets.images.get(key);
    if (!image) return;
    const texture = new T.Texture(image);
    texture.colorSpace = mapTextureColorSpace(key);
    texture.flipY = false;
    texture.magFilter = T.LinearFilter;
    texture.minFilter = T.LinearMipmapLinearFilter;
    if (repeat) texture.wrapS = texture.wrapT = T.RepeatWrapping;
    texture.anisotropy = Math.min(4, this.renderer.capabilities.getMaxAnisotropy());
    texture.needsUpdate = true;
    this.textures.push(texture);
    return texture;
  }

  private actor(asset: GLTF): Actor {
    const root = new T.Group();
    const model = clone(asset.scene);
    model.traverse(object => { object.frustumCulled = false; });
    root.add(model);
    const actor: Actor = { root, model, mixer: new T.AnimationMixer(model), clips: asset.animations, animation: '', weapon: '', alive: true, life: -1, deathAt: 0, corpseMotion: new CorpseInterpolation() };
    this.animate(actor, /^idle1?$|^idle_aim_|^idle/i);
    return actor;
  }

  private animate(actor: Actor, pattern: RegExp, once = false) {
    const clip = actor.clips.find(candidate => pattern.test(candidate.name));
    if (!clip || !once && actor.animation === clip.name) return;
    actor.mixer.stopAllAction();
    const action = actor.mixer.clipAction(clip);
    action.reset();
    action.setLoop(once ? T.LoopOnce : T.LoopRepeat, once ? 1 : Infinity);
    action.clampWhenFinished = once;
    action.play();
    actor.animation = clip.name;
  }

  private corpseHit() {
    return this.corpseRay.intersectObjects(this.mapMeshes, false).find(hit => {
      for (let object: T.Object3D | null = hit.object; object; object = object.parent) if (!object.visible) return false;
      return true;
    });
  }

  private corpsePosition(player: Player, actor: Actor, age: number) {
    if (actor.settled) return actor.settled;
    const position = actor.corpseMotion.sample(age, at => {
      const point = this.corpseCollision(player, at);
      return { x: point.x, y: point.y, z: point.z };
    });
    actor.root.position.set(position.x, position.y, position.z);
    if (age >= 3000) actor.settled = actor.root.position.clone();
    return actor.root.position;
  }

  private corpseCollision(player: Player, age: number) {
    if (this.collisionDirty) { this.world.updateWorldMatrix(true, true); this.collisionDirty = false; }
    const drift = corpseTravel({ x: player.deathVX ?? 0, y: player.deathVY ?? 0, z: player.deathVZ ?? 0 }, age);
    const halfHeight = player.crouch ? 18 : 36;
    this.corpseFrom.set(player.x / 256, player.z / 256, -player.y / 256);
    this.corpseTo.copy(this.corpseFrom);
    this.corpseDirection.set(drift.x, 0, -drift.y);
    const horizontal = this.corpseDirection.length();
    if (Math.round(horizontal * 256) > 0) {
      this.corpseDirection.normalize();
      this.corpseRay.set(this.corpseFrom, this.corpseDirection);
      this.corpseRay.near = 0.01;
      this.corpseRay.far = horizontal + 16;
      const hit = this.corpseHit();
      this.corpseTo.addScaledVector(this.corpseDirection, hit ? Math.max(0, Math.min(horizontal, hit.distance - 16)) : horizontal);
    }
    const originY = this.corpseFrom.y;
    let height = originY + drift.z;
    if (Math.round(player.deathVZ ?? 0) > 0) {
      const velocity = (player.deathVZ ?? 0) / 256;
      this.corpseRay.set(this.corpseTo, this.corpseDirection.set(0, 1, 0));
      this.corpseRay.far = Math.min(512, velocity * velocity / 1600) + halfHeight;
      const ceiling = this.corpseHit();
      if (ceiling) {
        const space = Math.max(0, ceiling.point.y - halfHeight - originY);
        const contact = (velocity - Math.sqrt(Math.max(0, velocity * velocity - 1600 * space))) / 800;
        const falling = Math.max(0, Math.min(3000, age) / 1000 - contact);
        height = Math.min(height, ceiling.point.y - halfHeight - 400 * falling * falling);
      }
    }
    this.corpseTo.y = Math.max(originY, height) + 1;
    this.corpseRay.set(this.corpseTo, this.corpseDirection.set(0, -1, 0));
    this.corpseRay.far = Math.max(halfHeight + 2, this.corpseTo.y - height + halfHeight + 2);
    const floor = this.corpseHit();
    if (floor) height = Math.max(height, floor.point.y + halfHeight);
    return this.corpseTo.set(this.corpseTo.x, -this.corpseTo.z, height);
  }

  private carried(actor: Actor, id: string) {
    if (actor.carried) actor.carried.removeFromParent();
    for (const bone of actor.carriedBones ?? []) bone.removeFromParent();
    for (const skeleton of actor.carriedSkeletons ?? []) skeleton.dispose();
    actor.carriedBones = [];
    actor.carriedSkeletons = [];
    actor.weapon = id;
    const key = id === 'mp5navy' ? 'mp5' : id;
    const model = this.assets.models.get(`p-${key}`);
    if (!model) return;
    const value = clone(model.scene);
    const bones: T.Bone[] = [];
    actor.model.traverse(object => { if (object instanceof T.Bone) bones.push(object); });
    value.traverse(object => {
      if (!(object instanceof T.SkinnedMesh)) return;
      const source = object.skeleton.bones;
      const mapping = carriedBoneMap(source.map(bone => bone.name), bones.map(bone => bone.name));
      for (let index = 0; index < source.length; index++) if (mapping[index] < 0) {
        const parent = source.findIndex(bone => bone === source[index].parent);
        if (parent >= 0 && mapping[parent] >= 0) { bones[mapping[parent]].add(source[index]); actor.carriedBones!.push(source[index]); }
      }
      const skeleton = new T.Skeleton(mapping.map((index, sourceIndex) => index < 0 ? source[sourceIndex] : bones[index]), object.skeleton.boneInverses.map(matrix => matrix.clone()));
      object.bind(skeleton, object.bindMatrix.clone());
      object.frustumCulled = false;
      actor.carriedSkeletons!.push(skeleton);
    });
    actor.model.add(value);
    actor.carried = value;
  }

  private setWeapon(id: string) {
    if (this.weaponID === id) return;
    if (this.weapon) this.weapon.root.removeFromParent();
    this.weaponID = id;
    this.weaponShots = 0;
    this.meleeSwings = 0;
    this.muzzleNodes = [];
    const key = id === 'mp5navy' ? 'mp5' : id;
    const asset = this.assets.models.get(`v-${key}`);
    if (!asset) { this.weapon = undefined; return; }
    this.weapon = this.actor(asset);
    this.muzzleNodes = muzzleAttachments(this.weapon.model);
    this.weapon.root.scale.y = viewModelHand(id);
    this.weaponRoot.add(this.weapon.root);
    const variant = (id === 'm4a1' || id === 'usp') && !this.weaponSilenced;
    this.animate(this.weapon, variant ? /^draw_unsil$/i : /draw|deploy/i, true);
  }

  private remote(now: number) {
    const target = now - 50;
    let next = this.frames.at(-1);
    let previous = next;
    for (let index = 0; index < this.frames.length - 1; index++) {
      if (this.frames[index + 1].at >= target) { previous = this.frames[index]; next = this.frames[index + 1]; break; }
    }
    if (!next) return [];
    if (!previous || previous === next) return next.view.players;
    const mix = Math.max(0, Math.min(1, (target - previous.at) / Math.max(1, next.at - previous.at)));
    return next.view.players.map(player => {
      const old = previous!.view.players.find(candidate => candidate.seat === player.seat);
      if (!old || old.life !== player.life || old.alive !== player.alive || old.visible === false || player.visible === false) return player;
      let yaw = player.yaw - old.yaw;
      if (yaw > 32768) yaw -= 65536;
      if (yaw < -32768) yaw += 65536;
      return { ...player, ...visibleVelocity(old, player, next.view.frame - previous!.view.frame), x: old.x + (player.x - old.x) * mix, y: old.y + (player.y - old.y) * mix, z: old.z + (player.z - old.z) * mix, yaw: old.yaw + yaw * mix, pitch: old.pitch + (player.pitch - old.pitch) * mix };
    });
  }

  private dynamic(view: View) {
    for (const state of view.brushes ?? []) {
      const brush = this.brushes.get(state.id);
      if (!brush) continue;
      brush.root.visible = !(brush.kind === 'func_breakable' && state.health <= 0);
      brush.root.position.copy(brush.origin).addScaledVector(brush.move, (state.progress ?? 0) / 1000);
      if (brush.kind === 'func_door_rotating') brush.root.rotation.z = (brush.angle || 16384) * (state.progress ?? 0) / 1000 * Math.PI * 2 / 65536 * (brush.flags & 2 ? -1 : 1) * (state.direction || 1);
    }
    const used = new Set<string>();
    const place = (key: string, modelKey: string, x: number, y: number, z: number, yaw = 0) => {
      used.add(key);
      let object = this.objects.get(key);
      if (!object) {
        const asset = this.assets.models.get(modelKey);
        if (!asset) return;
        object = clone(asset.scene);
        this.world.add(object);
        this.objects.set(key, object);
      }
      object.position.set(x / 256, y / 256, z / 256);
      object.rotation.z = yaw * Math.PI * 2 / 65536;
    };
    if (view.bomb?.planted || view.bomb?.dropped) place('bomb', 'w-backpack', view.bomb.x, view.bomb.y, view.bomb.z);
    for (const hostage of view.hostages ?? []) if (!hostage.rescued && hostage.health > 0) place(`hostage:${hostage.id}`, 'hostage', hostage.x, hostage.y, hostage.z);
    for (const drop of view.drops ?? []) place(`drop:${drop.id}`, `w-${drop.weapon.id === 'mp5navy' ? 'mp5' : drop.weapon.id}`, drop.x, drop.y, drop.z, drop.yaw ?? 0);
    for (const grenade of view.grenades ?? []) {
      if (grenade.kind === 'smokegrenade' && grenade.active) {
        let smoke = this.smoke.get(grenade.id);
        if (!smoke) {
          const geometry = new T.SphereGeometry(180, 12, 8);
          const material = new T.MeshBasicMaterial({ color: '#a0a39a', transparent: true, opacity: 0.82, depthWrite: false, side: T.DoubleSide });
          smoke = new T.Mesh(geometry, material);
          this.world.add(smoke);
          this.smoke.set(grenade.id, smoke);
          this.resources.push(geometry, material);
        }
        smoke.position.set(grenade.x / 256, grenade.y / 256, grenade.z / 256 + 70);
        smoke.scale.setScalar(smokeRadius(grenade, view.frame) / (180 * 256));
      } else if (!grenade.active) place(`grenade:${grenade.id}`, `w-${grenade.kind}`, grenade.x, grenade.y, grenade.z);
    }
    for (const [key, object] of this.objects) if (!used.has(key)) { object.removeFromParent(); this.objects.delete(key); }
    for (const [id, smoke] of this.smoke) if (!view.grenades.some(grenade => grenade.id === id && grenade.active)) { smoke.removeFromParent(); this.smoke.delete(id); }
  }

  dispose() {
    this.disposed = true;
    this.renderer.domElement.classList.remove('cs16-night-vision');
    this.effects.dispose();
    for (const actor of this.actors.values()) actor.mixer.stopAllAction();
    this.weapon?.mixer.stopAllAction();
    for (const texture of this.textures) texture.dispose();
    for (const resource of this.resources) resource.dispose();
    this.renderer.dispose();
    this.scene.clear();
    this.weaponScene.clear();
  }
}
