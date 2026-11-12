import * as T from "three";
import { GLTFLoader, type GLTF } from "three/examples/jsm/loaders/GLTFLoader.js";
import { mergeGeometries } from "three/examples/jsm/utils/BufferGeometryUtils.js";
import { classicVehicleByID } from "./classic-catalog";
import { classicVehicle } from "./classic-motion";
import { recolorCarPixels } from "./classic-paint";
import { classicInlineImagePlugin } from "./classic-images";
import { applyClassicRoadAttitude } from "./classic-attitude";
import { CLASSIC_CAMERA_MODES, CLASSIC_DRAW_DISTANCES, classicCameraPlan, classicMirrorViewport } from "./classic-camera-modes";
import { advanceClassicCameraLift, classicCameraClearance, type ClassicCameraBody } from "./classic-camera-clearance";
import { classicRoadAt, classicTrackFromRoute, classicPropBroken, classicBarrierBroken, type ClassicPropWorld, type ClassicRuleTrack, type NativeClassicRoute } from "./classic-road";
import { ClassicPropPlayback, ClassicPropClipPlayback, classicAmbientClipNames, validateClassicPropCatalog, type ClassicVisualPropCatalog, type ClassicPropPose } from "./classic-prop-animation";
import { seatColors, type Car, type Track } from "./types";
import type { SceneFrame } from "./chase-renderer";
import type { ChasePose } from "./chase-camera";

type Config = { timeOfDay: "day" | "night"; vehicles: readonly string[] };
type ClassicSceneFrame = SceneFrame & { cameraMode?: number; mirrorVisible?: boolean; drawDistance?: number };
type VisualCar = Car & {
  vehicle?: string;
  height?: number;
  pitch?: number;
  roll?: number;
  routeIndex?: number;
  motion?: { steering?: number; longAccel?: number; lateralAccel?: number; vx?: number; vz?: number };
};
type Environment = {
  horizon?: Record<string, number[] | number | boolean>;
  fog?: { color: number[]; density: number; regions?: Array<{ start_slice: number; center_slice: number; end_slice: number; start_color: number[]; center_color: number[]; end_color: number[]; start_density: number; center_density: number; end_density: number }> };
  lights?: Array<{ position: number[]; color: number[]; size: number; blinkMs: number | null }>;
};
type Wheel = { node: T.Object3D; base: T.Quaternion; turn: T.Group; front: boolean; radius: number };
type VehicleVisual = {
  id: string;
  root: T.Group;
  road: T.Group;
  suspension: T.Group;
  body: T.Group;
  exterior: T.Object3D;
  cockpit?: T.Object3D;
  cockpitEye?: readonly number[];
  wheels: Wheel[];
  materials: T.Material[];
  spin: number;
  pitch: number;
  roll: number;
  pitchVelocity: number;
  rollVelocity: number;
  displayedSteering: number;
};
type Batch = { mesh: T.Mesh; pieces: T.Sphere[] };
type TextureAnimation = { material: T.MeshBasicMaterial; frames?: T.Texture[]; interval?: number; scroll?: T.Texture; speed?: number; current: number };
type PropVisual = { root: T.Object3D; meshes: T.Mesh[]; sphere: T.Sphere; phase: ClassicPropPose["phase"]; time: number };

const clamp = (n: number, a: number, b: number) => Math.max(a, Math.min(b, n));
const finite = (n: unknown, fallback = 0): number => typeof n === "number" && Number.isFinite(n) ? n : fallback;
const angularDelta = (a: number, b: number) => Math.atan2(Math.sin(a-b), Math.cos(a-b));
const colour = (rgb: readonly number[] | undefined, fallback = "#7892a2") => rgb && rgb.length >= 3
  ? new T.Color().setRGB(rgb[0]/255, rgb[1]/255, rgb[2]/255, T.SRGBColorSpace)
  : new T.Color(fallback);

/** Original meshes and pixels, with batching rather than geometry simplification. */
export class ClassicRenderer {
  readonly mode = "webgl";
  readonly renderer: T.WebGLRenderer;
  private readonly scene = new T.Scene();
  private readonly camera = new T.PerspectiveCamera(62, 1, 1, 24000);
  private readonly mirrorCamera = new T.PerspectiveCamera(25, 3.5, .5, 4000);
  private readonly mirrorScene = new T.Scene();
  private readonly mirrorScreenCamera = new T.OrthographicCamera(-1, 1, 1, -1, 0, 1);
  private mirrorTarget?: T.WebGLRenderTarget;
  private readonly world = new T.Group();
  private readonly road: ClassicRuleTrack;
  private readonly geometries = new Set<T.BufferGeometry>();
  private readonly materials = new Set<T.Material>();
  private readonly textures = new Set<T.Texture>();
  private readonly templates = new Map<string, T.Group>();
  private readonly cockpitEyes = new Map<string, readonly number[]>();
  private readonly painted = new Map<string, T.Texture>();
  private readonly originals = new Map<string, ImageData>();
  private readonly vehicles = new Map<number, VehicleVisual>();
  private readonly batches: Batch[] = [];
  private readonly raySegments = new Map<number, T.Mesh[]>();
  private readonly rayStatic: T.Mesh[] = [];
  private readonly barrierCells = new Map<string, number[]>();
  private readonly textureAnimations: TextureAnimation[] = [];
  private readonly props = new Map<number, PropVisual>();
  private propWorld?: ClassicPropWorld;
  private propCatalog?: ClassicVisualPropCatalog;
  private propPlayback?: ClassicPropPlayback;
  private propClips?: ClassicPropClipPlayback;
  private propMixer?: T.AnimationMixer;
  private readonly localCamera = new T.Vector3();
  private readonly raycaster = new T.Raycaster();
  private readonly rayHits: T.Intersection[] = [];
  private readonly frustum = new T.Frustum();
  private readonly projection = new T.Matrix4();
  private readonly box = new T.Box3();
  private readonly neighborBox = new T.Box3();
  private readonly corner = new T.Vector3();
  private readonly scratch = new T.Vector3();
  private readonly eyeOffset = new T.Vector3();
  private readonly target = new T.Vector3();
  private readonly eye = new T.Vector3();
  private readonly viewDirection = new T.Vector3();
  private readonly shadowGeometry: T.PlaneGeometry;
  private readonly shadowTexture: T.CanvasTexture;
  private readonly hemisphere: T.HemisphereLight;
  private readonly sunlight: T.DirectionalLight;
  private mixer?: T.AnimationMixer;
  private environmentTarget?: T.WebGLRenderTarget;
  private sky?: T.Mesh;
  private glow?: T.Points;
  private environment: Environment = {};
  private fogDensity = 0.00015;
  private last = 0;
  private elapsed = 0;
  private heading = 0;
  private previousRear = false;
  private previousMode = 0;
  private sceneFar = 24000;
  private followed = -1;
  private cameraReady = false;
  private disposed = false;
  private width = 1;
  private height = 1;
  private frameCounter = 0;
  private decodedImages = 0;
  private neighborLift = 0;

  private constructor(private readonly canvas: HTMLCanvasElement, private readonly track: Track, private readonly config: Config, route: NativeClassicRoute) {
    this.road = classicTrackFromRoute(route);
    const declared = (track as Track & { geometry?: string }).geometry;
    if (declared && declared !== this.road.geometry) throw Error("赛车画面与道路版本不一致");
    this.renderer = new T.WebGLRenderer({ canvas, antialias: true, alpha: false, powerPreference: "high-performance" });
    // Main world, optional live mirror world, and its composite all count.
    this.renderer.info.autoReset = false;
    this.renderer.setPixelRatio(Math.min(1.5, window.devicePixelRatio || 1));
    this.renderer.outputColorSpace = T.SRGBColorSpace;
    this.renderer.toneMapping = T.NoToneMapping;
    this.world.scale.setScalar(10);
    this.scene.add(this.world);
    this.hemisphere = new T.HemisphereLight(config.timeOfDay === "night" ? "#a4b6dc" : "#e4efff", "#77735c", config.timeOfDay === "night" ? .8 : 1.45);
    this.sunlight = new T.DirectionalLight(config.timeOfDay === "night" ? "#b9cdf5" : "#fff3d9", config.timeOfDay === "night" ? .65 : 2.1);
    this.sunlight.position.set(-3000, 7000, 1800);
    this.scene.add(this.hemisphere, this.sunlight);
    this.shadowGeometry = new T.PlaneGeometry(1, 1);
    this.geometries.add(this.shadowGeometry);
    const shadow = document.createElement("canvas");
    shadow.width = 64; shadow.height = 128;
    const ctx = shadow.getContext("2d");
    if (!ctx) throw Error("赛车阴影初始化失败");
    ctx.scale(32, 64);
    const gradient = ctx.createRadialGradient(1, 1, .12, 1, 1, 1);
    gradient.addColorStop(0, "rgba(0,0,0,.65)");
    gradient.addColorStop(.65, "rgba(0,0,0,.4)");
    gradient.addColorStop(1, "rgba(0,0,0,0)");
    ctx.fillStyle = gradient; ctx.fillRect(0, 0, 2, 2);
    this.shadowTexture = new T.CanvasTexture(shadow);
    this.textures.add(this.shadowTexture);
    this.indexBarriers();
    this.resize();
  }

  static async create(canvas: HTMLCanvasElement, track: Track, files: ReadonlyMap<string, ArrayBuffer>, config: Config): Promise<ClassicRenderer> {
    const get = (path: string) => {
      const data = files.get(path);
      if (!data) throw Error(`赛车资源尚未就绪：${path}`);
      return data;
    };
    const route = JSON.parse(new TextDecoder().decode(get(`tracks/${track.id}.route.json`))) as NativeClassicRoute;
    const engine = new ClassicRenderer(canvas, track, config, route);
    try {
      const manager = new T.LoadingManager();
      manager.setURLModifier((url) => {
        if (!url.startsWith("blob:") && !url.startsWith("data:")) throw Error("赛车模型引用了未校验的外部资源");
        return url;
      });
      const loader = new GLTFLoader(manager);
      loader.register((parser) => classicInlineImagePlugin(parser, (image) => new T.Texture(image), {
        own: (texture) => {
          if (engine.disposed) {
            const image = texture.image as ImageBitmap;
            image.close?.(); texture.dispose();
            throw Error("赛车画面初始化已终止");
          }
          engine.textures.add(texture);
        },
        decoded: () => { engine.decodedImages++; },
      }));
      const name = track.id + (config.timeOfDay === "night" ? "-night" : "");
      const [environment, ...cars] = await Promise.all([
        loader.parseAsync(get(`tracks/${name}.glb`), ""),
        ...[...new Set(config.vehicles)].map((id) => loader.parseAsync(get(`cars/${id}.glb`), "")),
      ]);
      engine.registerTree(environment.scene);
      engine.environment = environment.parser.json.extras?.environment ?? {};
      const propCatalog = validateClassicPropCatalog(
        JSON.parse(new TextDecoder().decode(get(`props/${track.id}.json`))),
        track.id, engine.road.geometry, environment.parser.json.extras?.propsCatalogHash,
      );
      await engine.prepareTrack(environment, propCatalog);
      const ids = [...new Set(config.vehicles)];
      cars.forEach((gltf, index) => {
        engine.registerTree(gltf.scene);
        engine.templates.set(ids[index], gltf.scene);
        const eye = gltf.parser.json.extras?.cockpitCamera;
        if (Array.isArray(eye) && eye.length === 3 && eye.every(Number.isFinite)) engine.cockpitEyes.set(ids[index], eye);
      });
      await engine.prepareSky(get(`tracks/${name}-sky.png`));
      for (const [id, template] of engine.templates)
        for (let seat = 0; seat < seatColors.length; seat++)
          template.traverse((object) => {
            if (object instanceof T.Mesh)
              for (const material of Array.isArray(object.material) ? object.material : [object.material])
                if (material.userData.carPaintMaskInAlpha && (material as T.MeshStandardMaterial).map)
                  engine.paintTexture(id, seat, (material as T.MeshStandardMaterial).map!);
          });
      for (const texture of engine.painted.values()) engine.renderer.initTexture(texture);
      const first = engine.road.points[0], sample = engine.road.road[0];
      engine.camera.position.set(first.x - sample.fx*70, sample.height + 28, first.y - sample.fz*70);
      engine.camera.lookAt(first.x, sample.height+9, first.y);
      const staging = new T.Scene();
      staging.environment = engine.scene.environment;
      staging.environmentIntensity = engine.scene.environmentIntensity;
      staging.add(engine.hemisphere.clone(), engine.sunlight.clone());
      for (const [index, id] of ids.entries()) {
        const visual = engine.makeVehicle(id, index);
        visual.root.position.set(first.x, sample.height, first.y);
        staging.add(visual.root);
      }
      await engine.renderer.compileAsync(staging, engine.camera);
      engine.renderer.render(staging, engine.camera);
      for(const object of staging.children){
        const cockpit=object.getObjectByName("cockpit"),exterior=object.getObjectByName("exterior");
        if(cockpit){cockpit.visible=true;if(exterior)exterior.visible=false;}
      }
      await engine.renderer.compileAsync(staging,engine.camera);
      engine.renderer.render(staging,engine.camera);
      await engine.renderer.compileAsync(engine.scene, engine.camera);
      engine.renderer.render(engine.scene, engine.camera);
      // The temporary instances only upload the real model buffers/programs.
      // Seat ownership is established by the first authoritative SceneFrame.
      for (const object of [...staging.children]) {
        staging.remove(object);
        if (object instanceof T.Group) engine.releaseVehicleMaterials(object);
      }
      canvas.dataset.classic = "3";
      canvas.dataset.geometry = engine.road.geometry;
      canvas.dataset.decodedImages = String(engine.decodedImages);
      return engine;
    } catch (error) {
      engine.dispose();
      throw error;
    }
  }

  private configureTexture(texture: T.Texture, repeat = false) {
    texture.wrapS = repeat ? T.RepeatWrapping : T.ClampToEdgeWrapping;
    texture.wrapT = repeat ? T.RepeatWrapping : T.ClampToEdgeWrapping;
    texture.anisotropy = Math.min(8, this.renderer.capabilities.getMaxAnisotropy());
    texture.colorSpace = T.SRGBColorSpace;
    texture.needsUpdate = true;
    this.textures.add(texture);
  }

  private registerTree(root: T.Object3D) {
    root.traverse((object) => {
      if (!(object instanceof T.Mesh)) return;
      this.geometries.add(object.geometry);
      for (const material of Array.isArray(object.material) ? object.material : [object.material]) {
        this.materials.add(material);
        for (const value of Object.values(material))
          if (value instanceof T.Texture) this.configureTexture(value);
      }
    });
  }

  private async prepareTrack(gltf: GLTF, catalog: ClassicVisualPropCatalog) {
    const raw = gltf.scene;
    raw.scale.setScalar(10);
    raw.updateMatrixWorld(true);
    const inverse = raw.matrixWorld.clone().invert();
    this.propCatalog = catalog;
    this.propPlayback = new ClassicPropPlayback(catalog);
    const destructible: T.Object3D[] = [];
    raw.traverse(object => { if (object.userData.destructible) destructible.push(object); });
    if (destructible.length !== catalog.props.length) throw Error("原物件独立节点数量不一致");
    const propMaterials = new Set<T.Material>();
    for (const prop of catalog.props) {
      const object = raw.getObjectByName(prop.node), motion = object?.getObjectByName(prop.motionNode);
      if (!object?.parent || !object.userData.destructible || object.userData.propIndex !== prop.index ||
          object.userData.propId !== prop.id || !motion || motion.parent !== object ||
          motion.position.lengthSq() > 1e-12 || motion.quaternion.angleTo(new T.Quaternion()) > 1e-7 ||
          motion.scale.distanceToSquared(new T.Vector3(1,1,1)) > 1e-12)
        throw Error("原物件节点或局部动作原点不一致");
      for (const speed of ["low", "high"] as const) {
        const clip = gltf.animations.find(clip => clip.name === prop.clips[speed]);
        if (!clip || Math.abs(clip.duration - catalog.animations[speed].duration) > 1e-7 ||
            !clip.tracks.length || clip.tracks.some(track => !track.name.startsWith(`${prop.motionNode}.`)))
          throw Error("原物件碰撞动作绑定不一致");
      }
      // Detach before static collection: original pivots and mesh bytes survive,
      // while neither the batches nor their hidden ray copies retain this prop.
      const wrapper = new T.Group(); wrapper.matrixAutoUpdate = false;
      wrapper.matrix.multiplyMatrices(inverse, object.parent.matrixWorld);
      object.removeFromParent(); wrapper.add(object); this.world.add(wrapper);
      const meshes: T.Mesh[] = [];
      object.traverse(node => {
        if (!(node instanceof T.Mesh)) return;
        meshes.push(node);
        for (const material of Array.isArray(node.material) ? node.material : [node.material]) propMaterials.add(material);
      });
      if (!meshes.length) throw Error("原物件缺少原始几何");
      this.props.set(prop.index, { root: wrapper, meshes, sphere: new T.Sphere(), phase: "standing", time: 0 });
    }
    for (const material of propMaterials) {
      if (material.userData.originalBlend === "ADDITIVE") {
        material.transparent = true; material.depthWrite = false; material.blending = T.AdditiveBlending;
      }
      await this.prepareTextureAnimation(material as T.MeshBasicMaterial, gltf);
    }
    this.propMixer = new T.AnimationMixer(this.world);
    this.propClips = new ClassicPropClipPlayback(this.propMixer, gltf.animations, T.LoopOnce);
    const animated: T.Object3D[] = [];
    raw.traverse((object) => {
      if (object.userData.animated) animated.push(object);
    });
    for (const object of animated) {
      let ancestor = object.parent, nested = false;
      while (ancestor && ancestor !== raw) { nested ||= !!ancestor.userData.animated; ancestor = ancestor.parent; }
      if (nested) continue;
      const wrapper = new T.Group();
      wrapper.matrixAutoUpdate = false;
      wrapper.matrix.multiplyMatrices(inverse, object.parent!.matrixWorld);
      object.removeFromParent(); wrapper.add(object); this.world.add(wrapper);
    }
    if (gltf.animations.length) {
      this.mixer = new T.AnimationMixer(this.world);
      const ambient = classicAmbientClipNames(catalog, gltf.parser.json.animations ?? []);
      for (const clip of gltf.animations) if (ambient.has(clip.name)) this.mixer.clipAction(clip).play();
    }
    type Pending = { material: T.Material; geometries: T.BufferGeometry[]; pieces: T.Sphere[]; billboard: boolean };
    const pending = new Map<string, Pending>();
    const sourceMaterials = new Map<string, T.Material>();
    raw.traverse((object) => {
      if (!(object instanceof T.Mesh)) return;
      if (Array.isArray(object.material)) throw Error("赛车模型材质分组不受支持");
      const material = object.material;
      let ancestor: T.Object3D | null = object, billboard = false, pivot: T.Object3D | undefined, segment = -1;
      while (ancestor && ancestor !== raw) {
        if (ancestor.userData.billboard) { billboard = true; pivot = ancestor; }
        if (typeof ancestor.userData.segment === "number") segment = ancestor.userData.segment;
        ancestor = ancestor.parent;
      }
      if (!billboard && !material.transparent && !(material as T.MeshBasicMaterial).alphaTest) {
        const list = segment >= 0 ? this.raySegments.get(segment) ?? [] : this.rayStatic;
        list.push(object);
        if (segment >= 0) this.raySegments.set(segment, list);
      }
      const matrix = new T.Matrix4().multiplyMatrices(inverse, object.matrixWorld);
      const geometry = object.geometry.clone().applyMatrix4(matrix);
      geometry.computeBoundingSphere();
      const sphere = geometry.boundingSphere!.clone();
      sphere.center.multiplyScalar(10); sphere.radius *= 10;
      if (billboard) {
        const center = new T.Vector3().setFromMatrixPosition(pivot!.matrixWorld).applyMatrix4(inverse);
        const count = geometry.getAttribute("position").count, pivots = new Float32Array(count*3);
        for (let at = 0; at < count; at++) pivots.set([center.x, center.y, center.z], at*3);
        geometry.setAttribute("classicPivot", new T.BufferAttribute(pivots, 3));
        sphere.radius += 100;
      }
      const key = `${material.uuid}:${billboard ? "billboard" : "static"}:${Object.keys(geometry.attributes).sort().join(",")}`;
      let group = pending.get(key);
      if (!group) {
        const copy = material.clone();
        this.materials.add(copy);
        sourceMaterials.set(copy.uuid, material);
        if (copy.userData.originalBlend === "ADDITIVE") {
          copy.transparent = true; copy.depthWrite = false; copy.blending = T.AdditiveBlending;
        }
        if (billboard) this.billboardMaterial(copy);
        group = { material: copy, geometries: [], pieces: [], billboard };
        pending.set(key, group);
      }
      group.geometries.push(geometry); group.pieces.push(sphere);
    });
    for (const group of pending.values()) {
      const geometry = mergeGeometries(group.geometries, false);
      group.geometries.forEach((part) => part.dispose());
      if (!geometry) throw Error("赛车原始几何合批失败");
      geometry.computeBoundingBox(); geometry.computeBoundingSphere();
      if (group.billboard) {
        geometry.boundingBox!.expandByScalar(10);
        geometry.boundingSphere!.radius += 10;
      }
      this.geometries.add(geometry);
      const mesh = new T.Mesh(geometry, group.material);
      mesh.name = `classic_batch_${this.batches.length}`;
      this.world.add(mesh); this.batches.push({ mesh, pieces: group.pieces });
      await this.prepareTextureAnimation(group.material as T.MeshBasicMaterial, gltf);
    }
    this.world.updateMatrixWorld(true);
    for (const prop of this.props.values()) this.box.setFromObject(prop.root).getBoundingSphere(prop.sphere);
    this.canvas.dataset.propsCatalog = catalog.catalogHash;
    this.canvas.dataset.propsCount = String(catalog.props.length);
    this.makeLightGlows();
  }

  private billboardMaterial(material: T.Material) {
    material.onBeforeCompile = (shader) => {
      shader.uniforms.classicCamera = { value: this.localCamera };
      shader.vertexShader = `attribute vec3 classicPivot; uniform vec3 classicCamera;\n${shader.vertexShader}`;
      shader.vertexShader = shader.vertexShader.replace("#include <begin_vertex>", `
        #include <begin_vertex>
        vec3 classicDelta = transformed - classicPivot;
        float classicYaw = atan(classicCamera.x - classicPivot.x, classicCamera.z - classicPivot.z);
        float classicC = cos(classicYaw), classicS = sin(classicYaw);
        transformed.x = classicPivot.x + classicC * classicDelta.x + classicS * classicDelta.z;
        transformed.z = classicPivot.z - classicS * classicDelta.x + classicC * classicDelta.z;
      `);
    };
    material.customProgramCacheKey = () => "classic-original-billboards-v1";
    material.needsUpdate = true;
  }

  private async prepareTextureAnimation(material: T.MeshBasicMaterial, gltf: GLTF) {
    const animation = material.userData.textureAnimation as { kind?: string; textures?: number[]; interval?: number; speed?: number } | undefined;
    if (animation?.kind === "flipbook" && animation.textures?.length) {
      const frames = await Promise.all(animation.textures.map((index) => gltf.parser.getDependency("texture", index) as Promise<T.Texture>));
      frames.forEach((texture) => this.configureTexture(texture));
      this.textureAnimations.push({ material, frames, interval: Math.max(1/64, finite(animation.interval, 1/8)), current: -1 });
    } else if (animation?.kind === "uv-scroll" && material.map) {
      const texture = material.map.clone();
      this.configureTexture(texture, true); material.map = texture;
      this.textureAnimations.push({ material, scroll: texture, speed: finite(animation.speed), current: -1 });
    }
  }

  private async prepareSky(bytes: ArrayBuffer) {
    const bitmap = await createImageBitmap(new Blob([bytes], { type: "image/png" }));
    const sky = document.createElement("canvas");
    sky.width = bitmap.width; sky.height = Math.max(bitmap.height*2, Math.ceil(bitmap.width/2));
    const ctx = sky.getContext("2d");
    if (!ctx) { bitmap.close(); throw Error("赛车天空初始化失败"); }
    const horizon = this.environment.horizon ?? {};
    const css = (value: unknown, fallback: string) => Array.isArray(value) ? `rgb(${value.slice(0,3).join(",")})` : fallback;
    const gradient = ctx.createLinearGradient(0, 0, 0, sky.height);
    gradient.addColorStop(0, css(horizon.sun_top_side, this.config.timeOfDay === "night" ? "#0a0b1b" : "#c6dce9"));
    gradient.addColorStop(.49, css(horizon.sun_opposite_side, this.config.timeOfDay === "night" ? "#101122" : "#9ac3cf"));
    gradient.addColorStop(.72, css(horizon.earth_top, "#425146"));
    gradient.addColorStop(1, css(horizon.earth_bottom, "#283b35"));
    ctx.fillStyle = gradient; ctx.fillRect(0,0,sky.width,sky.height);
    ctx.drawImage(bitmap, 0, Math.round(sky.height*.49-bitmap.height*.13));
    bitmap.close();
    const texture = new T.CanvasTexture(sky);
    texture.colorSpace = T.SRGBColorSpace; texture.wrapS = T.RepeatWrapping; this.textures.add(texture);
    const geometry = new T.SphereGeometry(1, 32, 16);
    const material = new T.MeshBasicMaterial({ map: texture, side: T.BackSide, depthWrite: false, fog: false, toneMapped: false });
    this.geometries.add(geometry); this.materials.add(material);
    this.sky = new T.Mesh(geometry, material); this.sky.frustumCulled = false;
    this.sky.renderOrder = -10; this.scene.add(this.sky);
    const pmrem = new T.PMREMGenerator(this.renderer);
    this.environmentTarget = pmrem.fromEquirectangular(texture);
    this.scene.environment = this.environmentTarget.texture;
    this.scene.environmentIntensity = this.config.timeOfDay === "night" ? .45 : .8;
    pmrem.dispose();
    const fog = this.environment.fog;
    this.fogDensity = clamp(finite(fog?.density, this.config.timeOfDay === "night" ? 65 : 15)*.00001, .00005, .0015);
    this.scene.fog = new T.FogExp2(colour(fog?.color), this.fogDensity);
    this.camera.far = clamp(Math.sqrt(Math.log(4096))/this.fogDensity, 4000, 24000);
    this.sceneFar=this.camera.far;
    this.camera.updateProjectionMatrix();
  }

  private makeLightGlows() {
    const lights = this.environment.lights?.filter((light) => light.size > 0) ?? [];
    if (!lights.length) return;
    const geometry = new T.BufferGeometry();
    const positions: number[] = [], colors: number[] = [], sizes: number[] = [], blink: number[] = [];
    for (const light of lights) {
      positions.push(...light.position.map((n) => n*10));
      colors.push(...colour(light.color).toArray());
      sizes.push(clamp(light.size*6, 8, 50)); blink.push(light.blinkMs ? light.blinkMs/1000 : 0);
    }
    geometry.setAttribute("position", new T.Float32BufferAttribute(positions,3));
    geometry.setAttribute("color", new T.Float32BufferAttribute(colors,3));
    geometry.setAttribute("classicSize", new T.Float32BufferAttribute(sizes,1));
    geometry.setAttribute("classicBlink", new T.Float32BufferAttribute(blink,1));
    const material = new T.ShaderMaterial({
      uniforms: { classicTime:{value:0}, classicHeight:{value:this.height}, classicFog:{value:this.fogDensity}, classicReduced:{value:0} },
      vertexShader: `attribute vec3 color; attribute float classicSize; attribute float classicBlink;
        uniform float classicHeight, classicFog, classicTime, classicReduced; varying vec3 glowColor; varying float glowAlpha;
        void main(){ vec4 p=modelViewMatrix*vec4(position,1.0); gl_Position=projectionMatrix*p;
          gl_PointSize=clamp(classicSize*classicHeight/max(1.0,-p.z),1.0,54.0);
          glowColor=color; glowAlpha=exp(-pow(max(0.0,-p.z)*classicFog,2.0));
          if(classicBlink>0.0&&classicReduced<0.5)glowAlpha*=step(0.0,sin(classicTime*6.2831853/classicBlink)); }`,
      fragmentShader: `varying vec3 glowColor; varying float glowAlpha;
        void main(){float r=length(gl_PointCoord-0.5)*2.0;if(r>1.0)discard;
          float a=pow(1.0-r,2.0)*glowAlpha;gl_FragColor=vec4(glowColor,a);
          #include <colorspace_fragment>
        }`,
      transparent: true, depthWrite: false, blending: T.AdditiveBlending,
    });
    this.geometries.add(geometry); this.materials.add(material);
    this.glow = new T.Points(geometry, material); this.glow.renderOrder = 3; this.scene.add(this.glow);
  }

  private paintTexture(id: string, seat: number, original: T.Texture): T.Texture {
    const key = `${id}:${seat}:${original.source.uuid}`;
    const known = this.painted.get(key); if (known) return known;
    let pixels = this.originals.get(original.source.uuid);
    if (!pixels) {
      const image = original.image as CanvasImageSource & { width: number; height: number };
      const canvas = document.createElement("canvas"); canvas.width = image.width; canvas.height = image.height;
      const ctx = canvas.getContext("2d", { willReadFrequently: true });
      if (!ctx) throw Error("原车贴图读取失败");
      ctx.drawImage(image, 0, 0); pixels = ctx.getImageData(0,0,canvas.width,canvas.height);
      this.originals.set(original.source.uuid, pixels);
    }
    const definition = classicVehicleByID(id);
    const palette = definition.colors[0];
    if (!palette) throw Error(`车型缺少原始颜色：${id}`);
    const hex = new T.Color(seatColors[seat % seatColors.length]).getHex(T.SRGBColorSpace);
    const primary = [(hex>>16)&255,(hex>>8)&255,hex&255];
    const canvas = document.createElement("canvas"); canvas.width = pixels.width; canvas.height = pixels.height;
    const ctx = canvas.getContext("2d"); if (!ctx) throw Error("原车颜色初始化失败");
    ctx.putImageData(new ImageData(recolorCarPixels(pixels.data,primary,palette),pixels.width,pixels.height),0,0);
    const texture = new T.CanvasTexture(canvas);
    texture.flipY = original.flipY;
    texture.minFilter = original.minFilter; texture.magFilter = original.magFilter;
    this.configureTexture(texture); this.painted.set(key,texture);
    return texture;
  }

  private makeVehicle(id: string, seat: number): VehicleVisual {
    const template = this.templates.get(id);
    if (!template) throw Error(`车型资源尚未加载：${id}`);
    const definition = classicVehicleByID(id);
    const root = new T.Group(), road = new T.Group(), suspension = new T.Group();
    const body = template.clone(true);
    const exterior = body.getObjectByName("exterior") ?? body;
    const cockpit = body.getObjectByName("cockpit");
    if (cockpit) cockpit.visible = false;
    cockpit?.traverse(node=>{
      const part=String(node.userData.originalPart??"");
      if(part==="interior_dashboard_lit")node.visible=this.config.timeOfDay==="night";
      else if(part.includes("_DASH unlit"))node.visible=this.config.timeOfDay!=="night";
    });
    body.scale.setScalar(10); body.position.y = -definition.groundOffset*10;
    suspension.position.y = definition.groundOffset*10;
    root.add(road); road.add(suspension); suspension.add(body);
    const materials: T.Material[] = [], materialCopies = new Map<string,T.Material>();
    body.traverse((object) => {
      if (!(object instanceof T.Mesh)) return;
      const replace = (source: T.Material) => {
        const known = materialCopies.get(source.uuid); if (known) return known;
        const copy = source.clone() as T.MeshStandardMaterial;
        if (copy.map && copy.userData.carPaintMaskInAlpha) copy.map = this.paintTexture(id,seat,copy.map);
        copy.needsUpdate = true; materialCopies.set(source.uuid,copy);
        materials.push(copy); this.materials.add(copy); return copy;
      };
      object.material = Array.isArray(object.material) ? object.material.map(replace) : replace(object.material);
    });
    const wheels: Wheel[] = [];
    const nodes: T.Object3D[] = [];
    body.traverse((node) => { if (node.name.endsWith("_whl")) nodes.push(node); });
    for (const node of nodes) {
      const turn = new T.Group(); turn.name = `${node.name}_steering`;
      turn.position.copy(node.position); node.parent!.add(turn); turn.add(node); node.position.set(0,0,0);
      const box = new T.Box3().setFromObject(node), size = new T.Vector3(); box.getSize(size);
      const radius = clamp(Math.max(size.y,size.z)/20,.22,.48);
      wheels.push({ node, base:node.quaternion.clone(), turn, front:node.name.startsWith("front_"), radius });
    }
    const shadow = new T.Mesh(this.shadowGeometry, new T.MeshBasicMaterial({ map:this.shadowTexture, transparent:true, depthWrite:false, opacity:this.config.timeOfDay === "night" ? .18 : .35, toneMapped:false }));
    shadow.name = "contact_shadow"; shadow.rotation.x = -Math.PI/2; shadow.position.y = .08;
    shadow.scale.set(definition.dimensions[0]*11,definition.dimensions[2]*10.3,1); shadow.renderOrder = 1;
    road.add(shadow); materials.push(shadow.material); this.materials.add(shadow.material);
    return { id,root,road,suspension,body,exterior,cockpit,cockpitEye:this.cockpitEyes.get(id),wheels,materials,spin:0,pitch:0,roll:0,pitchVelocity:0,rollVelocity:0,displayedSteering:0 };
  }

  private releaseVehicleMaterials(root: T.Object3D) {
    root.traverse((object) => {
      if (!(object instanceof T.Mesh)) return;
      for (const material of Array.isArray(object.material) ? object.material : [object.material]) {
        material.dispose(); this.materials.delete(material);
      }
    });
  }

  resize(): void {
    if (this.disposed) return;
    const rect = this.canvas.getBoundingClientRect();
    this.width = Math.max(1,rect.width); this.height = Math.max(1,rect.height);
    this.renderer.setSize(this.width,this.height,false);
    this.camera.aspect = this.width/this.height; this.camera.updateProjectionMatrix();
    if (this.glow) (this.glow.material as T.ShaderMaterial).uniforms.classicHeight.value = this.height*this.renderer.getPixelRatio();
  }

  private indexBarriers() {
    this.road.barriers.forEach((barrier,index) => {
      const x0=Math.floor(Math.min(barrier.a.x,barrier.b.x)/100),x1=Math.floor(Math.max(barrier.a.x,barrier.b.x)/100);
      const z0=Math.floor(Math.min(barrier.a.y,barrier.b.y)/100),z1=Math.floor(Math.max(barrier.a.y,barrier.b.y)/100);
      for(let x=x0;x<=x1;x++)for(let z=z0;z<=z1;z++){
        const key=`${x},${z}`,values=this.barrierCells.get(key)??[];values.push(index);this.barrierCells.set(key,values);
      }
    });
  }

  private cameraObstacle(origin: T.Vector3, desired: T.Vector3, routeIndex: number): T.Vector3 {
    const direction = desired.clone().sub(origin), length=direction.length();
    if (length<1e-5) return desired;
    let limit=length;
    const nearby=new Set<number>();
    for(let x=Math.floor(Math.min(origin.x,desired.x)/100);x<=Math.floor(Math.max(origin.x,desired.x)/100);x++)
      for(let z=Math.floor(Math.min(origin.z,desired.z)/100);z<=Math.floor(Math.max(origin.z,desired.z)/100);z++)
        for(const index of this.barrierCells.get(`${x},${z}`)??[])nearby.add(index);
    for(const index of nearby){
      if (classicBarrierBroken(this.propWorld, index)) continue;
      const wall=this.road.barriers[index], wx=wall.b.x-wall.a.x,wz=wall.b.y-wall.a.y;
      const den=direction.x*wz-direction.z*wx;if(Math.abs(den)<1e-8)continue;
      const ax=wall.a.x-origin.x,az=wall.a.y-origin.z;
      const t=(ax*wz-az*wx)/den,u=(ax*direction.z-az*direction.x)/den;
      if(t<.03||t>1||u<0||u>1)continue;
      const y=origin.y+direction.y*t;
      if(y>=wall.bottom-2&&y<=wall.top+2)limit=Math.min(limit,length*t-5);
    }
    const segment=this.road.road[clamp(routeIndex,0,this.road.road.length-1)]?.segment??0;
    const candidates=[...this.rayStatic];
    for(let delta=-3;delta<=3;delta++){
      const max=this.raySegments.size,index=(segment+delta+max)%max;
      candidates.push(...this.raySegments.get(index)??[]);
    }
    for (const [index, prop] of this.props) {
      if (classicPropBroken(this.propWorld, index) || prop.phase !== "standing" ||
          prop.sphere.center.distanceTo(origin) > length + prop.sphere.radius + 20) continue;
      candidates.push(...prop.meshes);
    }
    this.raycaster.set(origin,direction.clone().normalize());this.raycaster.near=2;this.raycaster.far=length;
    this.rayHits.length=0;this.raycaster.intersectObjects(candidates,false,this.rayHits);
    if(this.rayHits[0])limit=Math.min(limit,this.rayHits[0].distance-5);
    if(limit<length)return origin.clone().addScaledVector(direction,Math.max(8,limit)/length);
    return desired;
  }

  private pose(car: VisualCar, frame: ClassicSceneFrame, dt: number): ChasePose {
    const speed=Math.abs((car.vx*Math.cos(car.angle)+car.vy*Math.sin(car.angle))/10);
    const ratio=clamp(speed/(classicVehicleByID(car.vehicle??this.config.vehicles[0]).maxSpeedKmh/3.6),0,1);
    const visual=this.vehicles.get(car.seat)!;
    const definition=classicVehicleByID(visual.id);
    const plan=classicCameraPlan(frame.cameraMode,frame.rear,this.width<this.height,definition.dimensions,visual.cockpitEye);
    const switched=!this.cameraReady||this.previousRear!==frame.rear||this.followed!==car.seat||this.previousMode!==plan.mode;
    this.canvas.dataset.cameraMode=String(plan.mode);this.canvas.dataset.cameraModeName=CLASSIC_CAMERA_MODES[plan.mode];
    visual.exterior.visible=!plan.hideExterior;
    if(visual.cockpit)visual.cockpit.visible=plan.kind==="cockpit";
    if(plan.kind!=="chase"){
      if(plan.kind==="cockpit"&&!visual.cockpit)throw Error("原车驾驶舱网格尚未就绪");
      visual.root.updateWorldMatrix(true,true);
      this.eye.fromArray(plan.eye).applyMatrix4(visual.body.matrixWorld);
      const forward=new T.Vector3(0,0,plan.direction).transformDirection(visual.body.matrixWorld);
      this.target.copy(this.eye).addScaledVector(forward,300);
      this.camera.up.set(0,1,0).transformDirection(visual.body.matrixWorld);
      this.neighborLift=0;
      this.canvas.dataset.cameraClearance=JSON.stringify({unit:"metres",requestedLift:0,appliedLift:0,neighbors:[]});
      this.camera.position.copy(this.eye);this.camera.lookAt(this.target);
      this.camera.near=plan.near*10;this.camera.fov=plan.fov+(frame.reduced?0:ratio*3);this.camera.updateProjectionMatrix();
      this.cameraReady=true;this.previousRear=frame.rear;this.previousMode=plan.mode;this.followed=car.seat;
      return {eye:{x:this.eye.x,y:this.eye.y,z:this.eye.z},target:{x:this.target.x,y:this.target.y,z:this.target.z},fov:this.camera.fov,near:this.camera.near};
    }
    if(switched||frame.reduced)this.heading=car.angle;
    else this.heading+=angularDelta(car.angle,this.heading)*(1-Math.exp(-dt*9));
    const sign=frame.rear?-1:1,fx=Math.cos(this.heading)*sign,fz=Math.sin(this.heading)*sign;
    const height=finite(car.height),distance=plan.distance*10;
    const lateral=frame.reduced?0:clamp(finite(car.motion?.lateralAccel)*.3,-4,4);
    const x=-fx*distance-fz*lateral,z=-fz*distance+fx*lateral;
    const eyeRoad=classicRoadAt(this.road,car.x+x,car.y+z,height,finite(car.routeIndex,-1));
    const relative=new T.Vector3(x,Math.max(20,eyeRoad.height-height+plan.lift*10),z);
    if(switched||frame.reduced)this.eyeOffset.copy(relative);
    else this.eyeOffset.lerp(relative,1-Math.exp(-dt*15));
    this.eye.set(car.x,height,car.y).add(this.eyeOffset);
    this.eye.y=Math.max(this.eye.y,eyeRoad.height+12);
    const probe=new T.Vector3(car.x,height+definition.dimensions[1]*10+3,car.y);
    this.eye.copy(this.cameraObstacle(probe,this.eye,finite(car.routeIndex,-1)));
    if(Math.hypot(this.eye.x-car.x,this.eye.z-car.y)<29)this.eye.y=Math.max(this.eye.y,height+38);
    const baseEyeY=this.eye.y,bodies:ClassicCameraBody[]=[];
    for(const [seat,visual]of this.vehicles){
      if(seat===car.seat)continue;
      const definition=classicVehicleByID(visual.id);
      if(Math.hypot(this.eye.x-visual.root.position.x,this.eye.z-visual.root.position.z)>definition.dimensions[2]*5+8)continue;
      visual.root.updateWorldMatrix(true,true);
      this.neighborBox.setFromObject(visual.exterior);
      if(this.neighborBox.isEmpty())continue;
      bodies.push({seat,min:{x:this.neighborBox.min.x,y:this.neighborBox.min.y,z:this.neighborBox.min.z},
                        max:{x:this.neighborBox.max.x,y:this.neighborBox.max.y,z:this.neighborBox.max.z}});
    }
    const clearance=classicCameraClearance(this.eye,car.seat,bodies);
    this.neighborLift=advanceClassicCameraLift(this.neighborLift,clearance,dt,switched||frame.reduced);
    this.eye.y+=this.neighborLift;
    if(this.neighborLift>.001){
      this.eye.copy(this.cameraObstacle(probe,this.eye,finite(car.routeIndex,-1)));
      if(Math.hypot(this.eye.x-car.x,this.eye.z-car.y)<29)this.eye.y=Math.max(this.eye.y,height+38);
    }
    const actual=classicCameraClearance(this.eye,car.seat,bodies);
    this.canvas.dataset.cameraClearance=JSON.stringify({unit:"metres",requestedLift:clearance.targetLift/10,
      appliedLift:(this.eye.y-baseEyeY)/10,neighbors:actual.neighbors.map(neighbor=>({seat:neighbor.seat,
        roof:neighbor.roof/10,clearance:(this.eye.y-neighbor.roof)/10,footprintDistance:neighbor.footprintDistance/10}))});
    this.target.set(car.x+fx*30,height+9,car.y+fz*30);
    const fov=plan.fov+(this.width<this.height||frame.reduced?0:ratio*5);
    this.camera.position.copy(this.eye);this.camera.up.set(0,1,0);this.camera.lookAt(this.target);
    if(Math.abs(this.camera.fov-fov)>.001||this.camera.near!==plan.near*10){this.camera.fov=fov;this.camera.near=plan.near*10;this.camera.updateProjectionMatrix();}
    this.cameraReady=true;this.previousRear=frame.rear;this.previousMode=plan.mode;this.followed=car.seat;
    return {eye:{x:this.eye.x,y:this.eye.y,z:this.eye.z},target:{x:this.target.x,y:this.target.y,z:this.target.z},fov,near:this.camera.near};
  }

  private cullWorld(camera: T.PerspectiveCamera) {
    camera.updateMatrixWorld(true);
    this.projection.multiplyMatrices(camera.projectionMatrix,camera.matrixWorldInverse);
    this.frustum.setFromProjectionMatrix(this.projection);
    for(const batch of this.batches)batch.mesh.visible=batch.pieces.some(sphere=>
      sphere.center.distanceToSquared(camera.position)<(camera.far+sphere.radius)**2&&this.frustum.intersectsSphere(sphere));
    for (const prop of this.props.values()) prop.root.visible = prop.phase !== "hidden" &&
      prop.sphere.center.distanceToSquared(camera.position) < (camera.far + prop.sphere.radius) ** 2 && this.frustum.intersectsSphere(prop.sphere);
  }

  private updateProps(frame: ClassicSceneFrame) {
    this.propWorld = frame.propWorld;
    if (!this.propPlayback || !this.propCatalog) return;
    const poses = this.propPlayback.update(frame.propWorld, frame.propEvents ?? [], finite(frame.serverNow), frame.reduced);
    const changed = this.propClips?.apply(poses);
    if (changed) this.world.updateMatrixWorld(true);
    for (const pose of poses) {
      const visual = this.props.get(pose.index)!;
      visual.root.visible = pose.phase !== "hidden";
      if (changed && (visual.time !== pose.time || visual.phase !== pose.phase))
        this.box.setFromObject(visual.root).getBoundingSphere(visual.sphere);
      visual.phase = pose.phase; visual.time = pose.time;
    }
    const broken = JSON.stringify(this.propCatalog.props.filter(prop => classicPropBroken(frame.propWorld, prop.index)).map(prop => prop.index));
    const animations = JSON.stringify(poses.filter(pose => pose.phase !== "standing"));
    if (this.canvas.dataset.propsBroken !== broken) this.canvas.dataset.propsBroken = broken;
    if (this.canvas.dataset.propAnimations !== animations) this.canvas.dataset.propAnimations = animations;
  }

  private renderMirror(visual: VehicleVisual) {
    const rect=classicMirrorViewport(this.width,this.height);if(!rect)return;
    const pixels=this.renderer.getPixelRatio(),width=Math.ceil(rect.width*pixels),height=Math.ceil(rect.height*pixels);
    if(!this.mirrorTarget){
      this.mirrorTarget=new T.WebGLRenderTarget(width,height,{depthBuffer:true,stencilBuffer:false,generateMipmaps:false});
      this.mirrorTarget.texture.colorSpace=T.SRGBColorSpace;
      // A real mirror reverses the live rear camera horizontally. Composite its
      // small render target rather than reversing 3D face winding/culling.
      this.mirrorTarget.texture.repeat.x=-1;this.mirrorTarget.texture.offset.x=1;
      const geometry=new T.PlaneGeometry(2,2),material=new T.MeshBasicMaterial({map:this.mirrorTarget.texture,depthTest:false,depthWrite:false,toneMapped:false});
      this.geometries.add(geometry);this.materials.add(material);
      const screen=new T.Mesh(geometry,material);screen.position.z=-.5;this.mirrorScene.add(screen);
    }else if(this.mirrorTarget.width!==width||this.mirrorTarget.height!==height)this.mirrorTarget.setSize(width,height);
    const definition=classicVehicleByID(visual.id);
    visual.root.updateWorldMatrix(true,true);
    this.mirrorCamera.position.set(0,definition.dimensions[1]+.08,-definition.dimensions[2]*.5-.18).applyMatrix4(visual.body.matrixWorld);
    const backward=new T.Vector3(0,0,-1).transformDirection(visual.body.matrixWorld);
    this.mirrorCamera.up.set(0,1,0).transformDirection(visual.body.matrixWorld);
    this.mirrorCamera.lookAt(backward.multiplyScalar(500).add(this.mirrorCamera.position));
    this.mirrorCamera.aspect=rect.width/rect.height;this.mirrorCamera.updateProjectionMatrix();
    const exteriorVisible=visual.exterior.visible,cockpitVisible=visual.cockpit?.visible;
    visual.exterior.visible=true;if(visual.cockpit)visual.cockpit.visible=false;
    if(this.sky){this.sky.position.copy(this.mirrorCamera.position);this.sky.scale.setScalar(this.mirrorCamera.far*.92);}
    this.localCamera.copy(this.mirrorCamera.position).multiplyScalar(.1);
    if(this.glow)(this.glow.material as T.ShaderMaterial).uniforms.classicHeight.value=height;
    this.cullWorld(this.mirrorCamera);
    const beforeCalls=this.renderer.info.render.calls,beforeTriangles=this.renderer.info.render.triangles;
    this.renderer.setRenderTarget(this.mirrorTarget);this.renderer.render(this.scene,this.mirrorCamera);
    this.renderer.setRenderTarget(null);
    this.renderer.autoClear=false;
    this.renderer.setViewport(rect.x,this.height-rect.y-rect.height,rect.width,rect.height);
    this.renderer.setScissor(rect.x,this.height-rect.y-rect.height,rect.width,rect.height);this.renderer.setScissorTest(true);
    this.renderer.render(this.mirrorScene,this.mirrorScreenCamera);
    this.renderer.setScissorTest(false);this.renderer.setViewport(0,0,this.width,this.height);this.renderer.autoClear=true;
    visual.exterior.visible=exteriorVisible;if(visual.cockpit)visual.cockpit.visible=cockpitVisible??false;
    if(this.sky){this.sky.position.copy(this.camera.position);this.sky.scale.setScalar(this.camera.far*.92);}
    this.localCamera.copy(this.camera.position).multiplyScalar(.1);
    if(this.glow)(this.glow.material as T.ShaderMaterial).uniforms.classicHeight.value=this.height*pixels;
    this.canvas.dataset.mirrorFrame=JSON.stringify({frame:this.frameCounter+1,live:true,rect,
      drawCalls:this.renderer.info.render.calls-beforeCalls,triangles:this.renderer.info.render.triangles-beforeTriangles,
      farMetres:this.mirrorCamera.far/10,eye:this.mirrorCamera.position.toArray().map(n=>n/10)});
  }

  render(frame: ClassicSceneFrame): ChasePose | null {
    if(this.disposed)return null;
    const dt=this.last?clamp((frame.now-this.last)/1000,0,.05):1/60;this.last=frame.now;
    if(this.canvas.clientWidth&&Math.abs(this.canvas.clientWidth-this.width)>.5)this.resize();
    this.elapsed+=dt;
    if(!frame.reduced)this.mixer?.update(dt);
    this.updateProps(frame);
    const present=new Set<number>();
    for(const base of frame.cars){
      const car=base as VisualCar,id=car.vehicle??this.config.vehicles[car.seat%this.config.vehicles.length];present.add(car.seat);
      let visual=this.vehicles.get(car.seat);
      if(!visual||visual.id!==id){
        if(visual){this.scene.remove(visual.root);this.releaseVehicleMaterials(visual.root);}
        visual=this.makeVehicle(id,car.seat);this.vehicles.set(car.seat,visual);this.scene.add(visual.root);
      }
      visual.exterior.visible=true;if(visual.cockpit)visual.cockpit.visible=false;
      const contact=classicRoadAt(this.road,car.x,car.y,finite(car.height),finite(car.routeIndex,-1));
      const height=finite(car.height,contact.height);
      const pitch=finite(car.pitch,Math.atan(-(contact.nx*Math.cos(car.angle)+contact.nz*Math.sin(car.angle))/Math.max(.1,contact.ny)));
      const roll=finite(car.roll);
      const pitchTarget=frame.reduced?0:clamp(finite(car.motion?.longAccel)*.004,-.045,.045);
      const rollTarget=frame.reduced?0:clamp(finite(car.motion?.lateralAccel)*.004,-.05,.05);
      visual.pitchVelocity+=((pitchTarget-visual.pitch)*80-visual.pitchVelocity*16)*dt;
      visual.rollVelocity+=((rollTarget-visual.roll)*80-visual.rollVelocity*16)*dt;
      visual.pitch=clamp(visual.pitch+visual.pitchVelocity*dt,-.055,.055);visual.roll=clamp(visual.roll+visual.rollVelocity*dt,-.06,.06);
      visual.root.position.set(car.x,height,car.y);visual.root.rotation.y=Math.PI/2-car.angle;
      applyClassicRoadAttitude(visual.road.rotation,pitch,roll);visual.suspension.rotation.set(-visual.pitch,0,-visual.roll,"YXZ");
      const params=classicVehicle(id),u=(car.vx*Math.cos(car.angle)+car.vy*Math.sin(car.angle))/10;
      const steering=clamp(finite(car.motion?.steering,car.steer),-1,1)/(1+.000055*u*u);
      const delta=steering*Math.atan(params.wheelbase/Math.max(1,params.turningRadius));
      visual.displayedSteering=steering;
      if(!frame.reduced)visual.spin+=u*dt;
      for(const wheel of visual.wheels){
        wheel.turn.rotation.y=wheel.front?-delta:0;
        wheel.node.quaternion.copy(wheel.base).multiply(new T.Quaternion().setFromAxisAngle(new T.Vector3(1,0,0),visual.spin/wheel.radius));
      }
    }
    for(const [seat,visual]of this.vehicles)if(!present.has(seat)){
      this.scene.remove(visual.root);this.releaseVehicleMaterials(visual.root);this.vehicles.delete(seat);
    }
    const followed=(frame.cars.find(car=>car.seat===frame.follow)??frame.cars[0])as VisualCar|undefined;
    if(!followed)return null;
    const distanceMode=Number.isInteger(frame.drawDistance)&&frame.drawDistance!>=0&&frame.drawDistance!<CLASSIC_DRAW_DISTANCES.length?frame.drawDistance!:2;
    const far=Math.min(this.sceneFar,CLASSIC_DRAW_DISTANCES[distanceMode]*10);
    if(this.camera.far!==far){this.camera.far=far;this.camera.updateProjectionMatrix();}
    this.canvas.dataset.drawDistance=String(distanceMode);this.canvas.dataset.drawDistanceMetres=String(far/10);
    this.canvas.dataset.drawDistanceLimitMetres=String(CLASSIC_DRAW_DISTANCES[distanceMode]);
    const pose=this.pose(followed,frame,dt);
    this.localCamera.copy(this.camera.position).multiplyScalar(.1);
    if(this.sky){this.sky.position.copy(this.camera.position);this.sky.scale.setScalar(this.camera.far*.92);}
    this.cullWorld(this.camera);
    for(const animation of this.textureAnimations){
      if(frame.reduced)continue;
      if(animation.frames){
        const index=Math.floor(this.elapsed/animation.interval!)%animation.frames.length;
        if(index!==animation.current){animation.material.map=animation.frames[index];animation.current=index;}
      }else if(animation.scroll)animation.scroll.offset.y=(this.elapsed*animation.speed!)%1;
    }
    if(this.glow){const u=(this.glow.material as T.ShaderMaterial).uniforms;u.classicTime.value=this.elapsed;u.classicFog.value=this.fogDensity;u.classicReduced.value=frame.reduced?1:0;}
    this.renderer.info.reset();this.renderer.render(this.scene,this.camera);
    const visual=this.vehicles.get(followed.seat)!;
    this.canvas.dataset.mirrorVisible=String(!!frame.mirrorVisible);
    if(frame.mirrorVisible)this.renderMirror(visual);else delete this.canvas.dataset.mirrorFrame;
    this.canvas.dataset.classic="3";this.canvas.dataset.geometry=this.road.geometry;this.canvas.dataset.vehicle=visual.id;
    this.canvas.dataset.followSeat=String(followed.seat);
    this.canvas.dataset.steering=String(visual.displayedSteering);
    this.canvas.dataset.renderedAt=String(performance.now());this.canvas.dataset.frameCounter=String(++this.frameCounter);
    this.canvas.dataset.triangles=String(this.renderer.info.render.triangles);this.canvas.dataset.drawCalls=String(this.renderer.info.render.calls);
    this.projectVehicle(visual.cockpit?.visible?visual.cockpit:visual.exterior);
    return pose;
  }

  private projectVehicle(body: T.Object3D) {
    this.box.setFromObject(body);
    let minX=Infinity,minY=Infinity,maxX=-Infinity,maxY=-Infinity;
    for(let index=0;index<8;index++){
      this.corner.set(index&1?this.box.max.x:this.box.min.x,index&2?this.box.max.y:this.box.min.y,index&4?this.box.max.z:this.box.min.z).project(this.camera);
      if(this.corner.z< -1||this.corner.z>1)continue;
      const x=(this.corner.x+1)*this.width*.5,y=(1-this.corner.y)*this.height*.5;
      minX=Math.min(minX,x);maxX=Math.max(maxX,x);minY=Math.min(minY,y);maxY=Math.max(maxY,y);
    }
    if(Number.isFinite(minX)&&maxX>minX&&maxY>minY)this.canvas.dataset.vehicleRect=JSON.stringify({x:minX,y:minY,width:maxX-minX,height:maxY-minY});
    else delete this.canvas.dataset.vehicleRect;
  }

  get stats() { return {calls:this.renderer.info.render.calls,triangles:this.renderer.info.render.triangles}; }

  dispose(): void {
    if(this.disposed)return;this.disposed=true;
    this.mixer?.stopAllAction();this.mixer?.uncacheRoot(this.world);
    this.propClips?.stop();this.propMixer?.uncacheRoot(this.world);
    this.geometries.forEach(geometry=>geometry.dispose());
    this.materials.forEach(material=>material.dispose());
    const images=new Set<unknown>();
    this.textures.forEach(texture=>{images.add(texture.source.data);texture.dispose();});
    for(const image of images)if(image&&typeof (image as {close?:unknown}).close==="function")try{(image as ImageBitmap).close();}catch{/* An already closed shared bitmap needs no second release. */}
    this.environmentTarget?.dispose();this.mirrorTarget?.dispose();this.renderer.dispose();this.renderer.forceContextLoss();
    this.vehicles.clear();this.templates.clear();this.cockpitEyes.clear();this.painted.clear();this.originals.clear();
    this.props.clear();
    for(const key of ["classic","geometry","vehicle","steering","renderedAt","frameCounter","vehicleRect","triangles","drawCalls","decodedImages","followSeat","cameraClearance","cameraMode","cameraModeName","drawDistance","drawDistanceMetres","drawDistanceLimitMetres","mirrorVisible","mirrorFrame","propsCatalog","propsCount","propsBroken","propAnimations"])delete this.canvas.dataset[key];
  }
}
