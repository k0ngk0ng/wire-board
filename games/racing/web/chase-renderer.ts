import * as T from "three";
import { RoomEnvironment } from "three/examples/jsm/environments/RoomEnvironment.js";
import { chasePose, type ChasePose } from "./chase-camera";
import { createVehicle, type VehicleModel } from "./vehicle-model";
import { themeFor, type SceneTheme } from "./scene-theme";
import { seatColors, type Car, type Track } from "./types";
export type SceneFrame = {
  cars: Car[];
  follow: number;
  rear: boolean;
  reduced: boolean;
  now: number;
};
function canvasTexture(
  width: number,
  height: number,
  draw: (ctx: CanvasRenderingContext2D) => void,
) {
  const c = document.createElement("canvas");
  c.width = width;
  c.height = height;
  draw(c.getContext("2d")!);
  const t = new T.CanvasTexture(c);
  t.colorSpace = T.SRGBColorSpace;
  return t;
}
function roadTexture(theme: SceneTheme) {
  const t = canvasTexture(256, 512, (c) => {
    c.fillStyle = theme.road;
    c.fillRect(0, 0, 256, 512);
    let seed = 419;
    for (let i = 0; i < 6000; i++) {
      seed = (Math.imul(seed, 1664525) + 1013904223) | 0;
      const x = (seed >>> 0) % 256;
      seed = (Math.imul(seed, 1664525) + 1013904223) | 0;
      c.fillStyle = i % 2 ? "#ffffff03" : "#00000006";
      c.fillRect(x, (seed >>> 0) % 512, 1, 1);
    }
    c.fillStyle = "#dce3db";
    c.fillRect(6, 0, 3, 512);
    c.fillRect(247, 0, 3, 512);
    c.fillStyle = theme.night ? "#e7d9a8" : "#f1d48b";
    c.fillRect(126, 0, 3, 160);
    c.fillRect(126, 256, 3, 160);
  });
  t.wrapS = T.ClampToEdgeWrapping;
  t.wrapT = T.RepeatWrapping;
  t.anisotropy = 4;
  return t;
}
function road(
  track: Track,
  width: number,
  y: number,
  material: T.Material,
  textured = false,
) {
  const vertices: number[] = [],
    uv: number[] = [],
    indices: number[] = [];
  let distance = 0;
  for (let i = 0; i <= track.points.length; i++) {
    const p = track.points[i % track.points.length],
      a = track.points[(i + track.points.length - 1) % track.points.length],
      b = track.points[(i + 1) % track.points.length];
    const dx = b.x - a.x,
      dz = b.y - a.y,
      l = Math.hypot(dx, dz),
      nx = -dz / l,
      nz = dx / l;
    if (i > 0) {
      const q = track.points[(i - 1) % track.points.length];
      distance += Math.hypot(p.x - q.x, p.y - q.y);
    }
    vertices.push(
      p.x + (nx * width) / 2,
      y,
      p.y + (nz * width) / 2,
      p.x - (nx * width) / 2,
      y,
      p.y - (nz * width) / 2,
    );
    uv.push(0, distance / 240, 1, distance / 240);
    if (i < track.points.length) {
      const k = i * 2;
      indices.push(k, k + 2, k + 1, k + 1, k + 2, k + 3);
    }
  }
  const g = new T.BufferGeometry();
  g.setAttribute("position", new T.Float32BufferAttribute(vertices, 3));
  g.setAttribute("uv", new T.Float32BufferAttribute(uv, 2));
  g.setIndex(indices);
  g.computeVertexNormals();
  const m = new T.Mesh(g, material);
  m.receiveShadow = true;
  return m;
}
function cube(
  w: number,
  h: number,
  d: number,
  color: T.ColorRepresentation,
  x: number,
  y: number,
  z: number,
) {
  const m = new T.Mesh(
    new T.BoxGeometry(w, h, d),
    new T.MeshStandardMaterial({ color, roughness: 0.8 }),
  );
  m.position.set(x, y, z);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}
export class ChaseRenderer {
  readonly renderer: T.WebGLRenderer;
  private scene = new T.Scene();
  private camera = new T.PerspectiveCamera(64, 1, 2, 50000);
  private cars = new Map<number, VehicleModel>();
  private theme: SceneTheme;
  private sun: T.DirectionalLight;
  private env: T.WebGLRenderTarget;
  private disposed = false;
  private last = 0;
  private width = 1;
  private height = 1;
  readonly mode = "webgl";
  constructor(
    private canvas: HTMLCanvasElement,
    private track: Track,
  ) {
    this.theme = themeFor(track.id);
    this.renderer = new T.WebGLRenderer({
      canvas,
      antialias: true,
      alpha: false,
      powerPreference: "high-performance",
    });
    this.renderer.setPixelRatio(Math.min(1.5, window.devicePixelRatio || 1));
    this.renderer.outputColorSpace = T.SRGBColorSpace;
    this.renderer.toneMapping = T.ACESFilmicToneMapping;
    this.renderer.toneMappingExposure = this.theme.night ? 1.05 : 1;
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = T.PCFSoftShadowMap;
    this.scene.background = new T.Color(this.theme.fog);
    this.scene.fog = new T.Fog(this.theme.fog, 1500, 7000);
    const pmrem = new T.PMREMGenerator(this.renderer),
      room = new RoomEnvironment();
    this.env = pmrem.fromScene(room, 0.04);
    this.scene.environment = this.env.texture;
    this.scene.environmentIntensity = this.theme.night ? 0.38 : 0.85;
    room.dispose();
    pmrem.dispose();
    const sky = canvasTexture(16, 256, (c) => {
      const g = c.createLinearGradient(0, 0, 0, 256);
      g.addColorStop(0, this.theme.sky);
      g.addColorStop(0.55, this.theme.horizon);
      g.addColorStop(1, this.theme.fog);
      c.fillStyle = g;
      c.fillRect(0, 0, 16, 256);
    });
    const dome = new T.Mesh(
      new T.SphereGeometry(15000, 24, 12),
      new T.MeshBasicMaterial({
        map: sky,
        side: T.BackSide,
        depthWrite: false,
        fog: false,
      }),
    );
    dome.position.set(track.width / 2, 0, track.height / 2);
    this.scene.add(dome);
    this.scene.add(
      new T.HemisphereLight(
        this.theme.night ? "#b4caff" : "#d4e9ff",
        this.theme.ground,
        this.theme.night ? 0.75 : 1.8,
      ),
    );
    this.sun = new T.DirectionalLight(
      this.theme.night ? "#c2d8ff" : "#fff1d3",
      this.theme.sun,
    );
    this.sun.castShadow = true;
    this.sun.shadow.mapSize.set(1024, 1024);
    Object.assign(this.sun.shadow.camera, {
      left: -300,
      right: 300,
      top: 300,
      bottom: -300,
      near: 5,
      far: 1400,
    });
    this.sun.shadow.bias = -0.0008;
    this.sun.shadow.normalBias = 1;
    this.scene.add(this.sun, this.sun.target);
    const ground = new T.Mesh(
      new T.PlaneGeometry(track.width + 12000, track.height + 12000),
      new T.MeshStandardMaterial({ color: this.theme.ground, roughness: 1 }),
    );
    ground.rotation.x = -Math.PI / 2;
    ground.position.set(track.width / 2, -0.8, track.height / 2);
    ground.receiveShadow = true;
    this.scene.add(ground);
    this.scene.add(
      road(
        track,
        track.roadWidth + 34,
        -0.15,
        new T.MeshStandardMaterial({
          color: this.theme.night ? "#48525e" : "#aa9d83",
          roughness: 0.95,
        }),
      ),
    );
    this.scene.add(
      road(
        track,
        track.roadWidth,
        0.2,
        new T.MeshStandardMaterial({
          map: roadTexture(this.theme),
          roughness: this.theme.night ? 0.6 : 0.9,
          metalness: this.theme.night ? 0.06 : 0.03,
        }),
        true,
      ),
    );
    const a = track.points[0],
      b = track.points[1],
      heading = Math.atan2(b.y - a.y, b.x - a.x);
    const sunDisc = new T.Mesh(
      new T.SphereGeometry(this.theme.night ? 130 : 230, 20, 12),
      new T.MeshBasicMaterial({
        color: this.theme.night ? "#e0e7dd" : "#fff0c9",
        fog: false,
        toneMapped: false,
      }),
    );
    sunDisc.position.set(
      a.x + Math.cos(heading) * 9500 - Math.sin(heading) * 2300,
      3200,
      a.y + Math.sin(heading) * 9500 + Math.cos(heading) * 2300,
    );
    this.scene.add(sunDisc);
    this.scenery();
    this.landscape();
    this.markers();
    this.resize();
  }
  private scenery() {
    const windows = canvasTexture(64, 128, (c) => {
      c.fillStyle = "#182736";
      c.fillRect(0, 0, 64, 128);
      for (let y = 5; y < 128; y += 12)
        for (let x = 4; x < 64; x += 10) {
          c.fillStyle =
            (x * 7 + y * 13) % 5 === 0
              ? "#18232e"
              : this.theme.night
                ? (x + y) % 3
                  ? "#b9b6a0"
                  : "#699ba6"
                : "#698a9c";
          c.fillRect(x, y, 5, 7);
        }
    });
    const buildingMat = new T.MeshStandardMaterial({
      map: windows,
      emissiveMap: windows,
      emissive: this.theme.night ? "#8094aa" : "#000000",
      emissiveIntensity: this.theme.night ? 0.65 : 0,
      roughness: 0.63,
      metalness: 0.3,
    });
    for (const [i, p] of (this.track.props ?? []).entries()) {
      const group = new T.Group();
      group.position.set(p.x, 0, p.y);
      if (p.kind === "building") {
        const w = p.radius * 1.4,
          body = new T.Mesh(new T.BoxGeometry(w, p.height, w), buildingMat);
        body.scale.y = 0.7;
        body.position.y = p.height * 0.35;
        body.castShadow = true;
        body.receiveShadow = true;
        group.add(body);
        group.add(cube(w + 2, 3, w + 2, "#142230", 0, p.height * 0.7, 0));
        const band = new T.Mesh(
          new T.BoxGeometry(w + 1, 1.8, w + 1),
          new T.MeshBasicMaterial({
            color: i % 2 ? this.theme.edge : this.theme.accent,
            toneMapped: false,
          }),
        );
        band.position.y = 16;
        group.add(band);
      } else if (p.kind === "container") {
        const colors = ["#326f82", "#ad594c", "#c29b45", "#506473"];
        for (let level = 0; level < Math.round(p.height / 22); level++) {
          group.add(
            cube(
              p.radius * 1.4,
              20,
              p.radius * 1.4,
              colors[(i + level) % 4],
              0,
              10 + level * 21,
              0,
            ),
          );
          for (let z = -20; z <= 20; z += 8)
            group.add(
              cube(p.radius * 1.42, 18, 1, "#233b45", 0, 10 + level * 21, z),
            );
        }
      } else if (p.kind === "rock") {
        const rock = new T.Mesh(
          new T.DodecahedronGeometry(p.radius, 0),
          new T.MeshStandardMaterial({
            color: i % 2 ? "#a86b49" : "#bf865a",
            roughness: 1,
            flatShading: true,
          }),
        );
        rock.scale.y = p.height / (p.radius * 2);
        rock.position.y = p.height * 0.45;
        rock.rotation.y = i;
        rock.castShadow = true;
        group.add(rock);
      } else {
        const trunk = new T.Mesh(
          new T.CylinderGeometry(2, 3.3, p.height * 0.65, 7),
          new T.MeshStandardMaterial({ color: "#665444" }),
        );
        trunk.position.y = p.height * 0.325;
        trunk.castShadow = true;
        group.add(trunk);
        if (p.kind === "palm")
          for (let k = 0; k < 7; k++) {
            const leaf = new T.Mesh(
              new T.ConeGeometry(5, 32, 4),
              new T.MeshStandardMaterial({
                color: k % 2 ? "#437467" : "#507951",
                roughness: 1,
              }),
            );
            leaf.position.set(Math.cos(k) * 9, p.height - 8, Math.sin(k) * 9);
            leaf.rotation.z = 0.9;
            leaf.rotation.y = k;
            leaf.castShadow = true;
            group.add(leaf);
          }
        else
          for (let k = 0; k < 3; k++) {
            const crown = new T.Mesh(
              new T.ConeGeometry(p.radius * (1 - k * 0.2), p.height * 0.5, 9),
              new T.MeshStandardMaterial({
                color: k % 2 ? "#355c4e" : "#406c55",
                roughness: 1,
              }),
            );
            crown.position.y = p.height * (0.42 + k * 0.19);
            crown.castShadow = true;
            group.add(crown);
          }
      }
      this.scene.add(group);
    }
  }
  private landscape() {
    const { width: w, height: h, id } = this.track;
    if (id === "coast" || id === "harbor") {
      const sea = new T.Mesh(
        new T.PlaneGeometry(14000, 18000, 1, 1),
        new T.MeshStandardMaterial({
          color: id === "coast" ? "#397f93" : "#1d3d56",
          metalness: 0.5,
          roughness: 0.3,
        }),
      );
      sea.rotation.x = -Math.PI / 2;
      sea.position.set(w + 7000, -1, h / 2);
      this.scene.add(sea);
    }
    for (let i = 0; i < 22; i++) {
      const theta = (i * Math.PI * 2) / 22,
        r = Math.max(w, h) * 0.8,
        x = w / 2 + Math.cos(theta) * r,
        z = h / 2 + Math.sin(theta) * r;
      if (id === "metro" || id === "harbor") {
        const height = 180 + ((i * 67) % 380);
        this.scene.add(
          cube(150, height, 180, this.theme.fog, x, height / 2, z),
        );
      } else {
        const mountain = new T.Mesh(
          new T.ConeGeometry(650 + (i % 3) * 180, 500 + (i % 4) * 160, 7),
          new T.MeshStandardMaterial({
            color: id === "canyon" ? "#b17c59" : "#5f7b7c",
            flatShading: true,
            roughness: 1,
          }),
        );
        mountain.position.set(x, 160, z);
        mountain.rotation.y = i;
        this.scene.add(mountain);
      }
    }
  }
  private markers() {
    const pts = this.track.points;
    for (let i = 0; i < pts.length; i += 3) {
      const p = pts[i],
        b = pts[(i + 1) % pts.length],
        angle = Math.atan2(b.y - p.y, b.x - p.x),
        nx = -Math.sin(angle),
        nz = Math.cos(angle);
      for (const side of [-1, 1]) {
        const x = p.x + nx * (this.track.roadWidth / 2 + 8) * side,
          z = p.y + nz * (this.track.roadWidth / 2 + 8) * side;
        const post = cube(1.8, 5, 1.8, "#b8c5b7", x, 2.5, z);
        this.scene.add(post);
        const marker = new T.Mesh(
          new T.BoxGeometry(2.2, 1.1, 2.2),
          new T.MeshBasicMaterial({
            color: this.theme.edge,
            toneMapped: false,
          }),
        );
        marker.position.set(x, 5, z);
        this.scene.add(marker);
      }
    }
    const a = pts[0],
      b = pts[1],
      heading = Math.atan2(b.y - a.y, b.x - a.x),
      start = new T.Group();
    start.position.set(a.x, 0.6, a.y);
    start.rotation.y = -heading;
    const tex = canvasTexture(256, 32, (c) => {
      for (let y = 0; y < 2; y++)
        for (let x = 0; x < 16; x++) {
          c.fillStyle = (x + y) % 2 ? "#e2e9df" : "#172332";
          c.fillRect(x * 16, y * 16, 16, 16);
        }
    });
    const line = new T.Mesh(
      new T.PlaneGeometry(10, this.track.roadWidth),
      new T.MeshStandardMaterial({ map: tex, roughness: 0.8 }),
    );
    line.rotation.x = -Math.PI / 2;
    start.add(line);
    // A starting arch is clear of the road and tall enough to drive beneath.
    for (const side of [-1, 1])
      start.add(
        cube(
          7,
          70,
          7,
          "#293946",
          0,
          35,
          side * (this.track.roadWidth / 2 + 15),
        ),
      );
    start.add(cube(8, 10, this.track.roadWidth + 38, "#202f40", 0, 70, 0));
    const trim = new T.Mesh(
      new T.BoxGeometry(8.2, 2, this.track.roadWidth + 39),
      new T.MeshBasicMaterial({ color: this.theme.accent, toneMapped: false }),
    );
    trim.position.y = 74;
    start.add(trim);
    this.scene.add(start);
  }
  resize() {
    const r = this.canvas.getBoundingClientRect();
    this.width = Math.max(1, r.width);
    this.height = Math.max(1, r.height);
    this.renderer.setSize(this.width, this.height, false);
    this.camera.aspect = this.width / this.height;
    this.camera.updateProjectionMatrix();
  }
  render(frame: SceneFrame): ChasePose | null {
    if (this.disposed) return null;
    const dt = Math.min(0.05, Math.max(0, (frame.now - this.last) / 1000));
    this.last = frame.now;
    for (const car of frame.cars) {
      let model = this.cars.get(car.seat);
      if (!model || model.skin !== car.skin) {
        if (model) {
          this.scene.remove(model.root);
          this.disposeTree(model.root);
        }
        model = createVehicle(
          seatColors[car.seat % seatColors.length],
          car.skin,
        );
        this.cars.set(car.seat, model);
        this.scene.add(model.root);
      }
      model.root.position.set(car.x, 0, car.y);
      model.root.rotation.y = -car.angle;
      model.brakes.color.set(car.brake || car.drift ? "#ff4d52" : "#a91b31");
      model.frontWheels.forEach((w) => (w.rotation.y = -car.steer * 0.35));
      if (!frame.reduced)
        model.wheels.forEach((w) => {
          for (const mesh of w.children)
            mesh.rotation.z -= (car.speed * dt) / 4.6;
        });
    }
    const followed =
      frame.cars.find((c) => c.seat === frame.follow) ?? frame.cars[0];
    if (!followed) return null;
    const pose = chasePose(
      frame.reduced ? { ...followed, speed: 0 } : followed,
      frame.rear,
      this.width / this.height,
    );
    this.camera.position.set(pose.eye.x, pose.eye.y, pose.eye.z);
    this.camera.lookAt(pose.target.x, pose.target.y, pose.target.z);
    this.camera.fov = pose.fov;
    this.camera.updateProjectionMatrix();
    this.sun.position.set(followed.x + 300, 550, followed.y + 250);
    this.sun.target.position.set(followed.x, 0, followed.y);
    this.renderer.render(this.scene, this.camera);
    return pose;
  }
  private disposeTree(root: T.Object3D) {
    const geometries = new Set<T.BufferGeometry>(),
      materials = new Set<T.Material>(),
      textures = new Set<T.Texture>();
    root.traverse((o) => {
      if (o instanceof T.Mesh) {
        geometries.add(o.geometry);
        for (const m of Array.isArray(o.material) ? o.material : [o.material])
          materials.add(m);
      }
    });
    for (const m of materials)
      for (const value of Object.values(m))
        if (value instanceof T.Texture) textures.add(value);
    geometries.forEach((g) => g.dispose());
    textures.forEach((t) => t.dispose());
    materials.forEach((m) => m.dispose());
  }
  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    this.disposeTree(this.scene);
    this.env.dispose();
    this.renderer.dispose();
    this.renderer.forceContextLoss();
  }
  get stats() {
    return {
      calls: this.renderer.info.render.calls,
      triangles: this.renderer.info.render.triangles,
    };
  }
}
