import * as T from "three";
import type { Options, View, Stroke, Vec } from "./types";
import { localPoint, sampleFlight } from "./timeline";
import { contactPoint } from "./prediction";
import { cameraSetup, racketFromScreen } from "./view-space";
export type CourtFrame = {
  ball?: Vec;
  predicted?: boolean;
  inputEnabled?: boolean;
  racketScreenX: number | null;
  data: View;
  seat: number;
  time: number;
  now: number;
  gesture: { x: number; y: number; down: boolean };
  swing?: { at: number; stroke: Stroke; contact?: Vec; connected?: boolean };
  reduced: boolean;
};
export type CourtRenderer = {
  render: (frame: CourtFrame) => void;
  resize: () => void;
  dispose: () => void;
  stats: () => { calls: number; triangles: number };
};
export function createCourt(
  canvas: HTMLCanvasElement,
  court: Options["court"],
  players: number,
): CourtRenderer {
  const viewport = canvas.parentElement!;
  const renderer = new T.WebGLRenderer({
    canvas,
    antialias: true,
    alpha: false,
  });
  renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = T.PCFSoftShadowMap;
  renderer.outputColorSpace = T.SRGBColorSpace;
  renderer.toneMapping = T.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.12;
  const themes = {
    club: {
      title: "A · 现代球馆",
      description:
        "奶油白、鼠尾草绿与赤陶红。纯球拍、柔和日光、清楚的击球轮廓。",
      bg: "#dce4d9",
      floor: "#cba87a",
      table: "#2c796f",
      edge: "#174c45",
      home: "#e9e7cd",
      away: "#bd5b43",
      short: "#234f45",
      wall: "#e6e8dd",
      accent: "#d5e7a0",
      colors: ["#e9e7d8", "#2c796f", "#bd5b43", "#cba87a"],
    },
    arena: {
      title: "B · 街机赛场",
      description:
        "深蓝竞技空间、珊瑚红球台、青绿色边光。清晰的球拍轮廓与抽球速度感。",
      bg: "#101f33",
      floor: "#23384a",
      table: "#78394f",
      edge: "#482b45",
      home: "#58c7c0",
      away: "#ecb259",
      short: "#213146",
      wall: "#16273d",
      accent: "#b4f7df",
      colors: ["#15273c", "#b75868", "#65c8c3", "#edb762"],
    },
    heritage: {
      title: "C · 复古体育馆",
      description:
        "蜂蜜色木地板、墨绿球台和高窗日光。传统胶皮球拍，带生活气息的运动空间。",
      bg: "#dfcbb1",
      floor: "#b88953",
      table: "#316459",
      edge: "#23473e",
      home: "#eee0b8",
      away: "#b96947",
      short: "#455345",
      wall: "#e4d2b5",
      accent: "#ede6a3",
      colors: ["#e6d6bb", "#356859", "#b66b4b", "#b68a57"],
    },
  };
  const themeName = court,
    theme = themes[court];
  let scene: T.Scene,
    camera: T.PerspectiveCamera,
    ball: T.Mesh,
    ballShadow: T.Mesh,
    streak: T.Line,
    racketRig: T.Group,
    rackets: T.Group[] = [];
  let seed = 8;
  const v = (x: number, y: number, z: number) => new T.Vector3(x, y, z);
  function rand() {
    seed = (seed * 1664525 + 1013904223) >>> 0;
    return seed / 4294967296;
  }
  function mat(color: T.ColorRepresentation, roughness = 0.72, metalness = 0) {
    return new T.MeshStandardMaterial({ color, roughness, metalness });
  }
  function box(
    w: number,
    h: number,
    d: number,
    m: T.Material | string,
    x = 0,
    y = 0,
    z = 0,
    parent: T.Object3D = scene,
  ) {
    const o = new T.Mesh(
      new T.BoxGeometry(w, h, d),
      typeof m === "string" ? mat(m) : m,
    );
    o.position.set(x, y, z);
    o.castShadow = true;
    o.receiveShadow = true;
    parent.add(o);
    return o;
  }
  function sphere(
    r: number,
    m: T.Material | string,
    x = 0,
    y = 0,
    z = 0,
    parent: T.Object3D = scene,
    sx = 1,
    sy = 1,
    sz = 1,
  ) {
    const o = new T.Mesh(
      new T.SphereGeometry(r, 24, 16),
      typeof m === "string" ? mat(m) : m,
    );
    o.position.set(x, y, z);
    o.scale.set(sx, sy, sz);
    o.castShadow = true;
    o.receiveShadow = true;
    parent.add(o);
    return o;
  }
  function rod(
    a: T.Vector3,
    b: T.Vector3,
    r: number,
    m: T.Material | string,
    parent: T.Object3D = scene,
    r2 = r,
  ) {
    const dir = b.clone().sub(a),
      o = new T.Mesh(
        new T.CylinderGeometry(r, r2, dir.length(), 12),
        typeof m === "string" ? mat(m) : m,
      );
    o.position.copy(a).add(b).multiplyScalar(0.5);
    o.quaternion.setFromUnitVectors(v(0, 1, 0), dir.normalize());
    o.castShadow = true;
    o.receiveShadow = true;
    parent.add(o);
    return o;
  }
  function line(
    points: T.Vector3[],
    color: string,
    width = 0.025,
    parent: T.Object3D = scene,
  ) {
    for (let i = 1; i < points.length; i++)
      rod(points[i - 1], points[i], width, color, parent);
  }
  function woodTexture() {
    const c = document.createElement("canvas");
    c.width = c.height = 1024;
    const ctx = c.getContext("2d")!;
    ctx.fillStyle = theme.floor;
    ctx.fillRect(0, 0, 1024, 1024);
    for (let i = 0; i < 24; i++) {
      let y = i * 43;
      ctx.fillStyle = i % 2 ? "#ffffff0e" : "#00000008";
      ctx.fillRect(0, y, 1024, 43);
      ctx.fillStyle = "#38271924";
      ctx.fillRect(0, y, 1024, 1);
      for (let j = 0; j < 8; j++) {
        let x = j * 150 + (i % 2) * 75;
        ctx.fillRect(x, y, 1, 43);
      }
      for (let j = 0; j < 22; j++) {
        ctx.strokeStyle = rand() > 0.5 ? "#4f34100b" : "#fff5dc14";
        ctx.beginPath();
        let x = rand() * 1024;
        ctx.moveTo(x, y + rand() * 42);
        ctx.lineTo(x + 100 + rand() * 300, y + rand() * 42);
        ctx.stroke();
      }
    }
    const tex = new T.CanvasTexture(c);
    tex.colorSpace = T.SRGBColorSpace;
    tex.anisotropy = 4;
    return tex;
  }
  function roundedCourt() {
    const floorMat =
      themeName === "arena"
        ? mat(theme.floor)
        : new T.MeshStandardMaterial({ map: woodTexture(), roughness: 0.82 });
    box(15, 0.24, 28, floorMat, 0, -0.14, 0);
    box(
      15.3,
      0.3,
      18.3,
      themeName === "arena" ? "#102030" : "#a4997d",
      0,
      -0.39,
      0,
    );
    line(
      [
        v(-5.8, 0.015, -7),
        v(5.8, 0.015, -7),
        v(5.8, 0.015, 7),
        v(-5.8, 0.015, 7),
        v(-5.8, 0.015, -7),
      ],
      themeName === "arena" ? "#66818b" : "#efe7c7",
      0.025,
    );
  }
  function table() {
    const top = mat(theme.table, 0.4),
      edge = mat(theme.edge, 0.48);
    box(4.2, 0.13, 7.56, top, 0, 2.02, 0);
    box(4.24, 0.15, 7.6, edge, 0, 1.89, 0);
    for (const x of [-2.04, 2.04])
      box(0.035, 0.009, 7.42, "#f2f2dc", x, 2.091, 0);
    for (const z of [-3.7, 3.7])
      box(4.07, 0.009, 0.035, "#f2f2dc", 0, 2.091, z);
    box(0.022, 0.009, 7.4, "#d9e4cf", 0, 2.091, 0);
    const steel = mat(themeName === "arena" ? "#223c51" : "#4d6054", 0.5, 0.25);
    for (const z of [-2.4, 2.4]) {
      for (const x of [-1.85, 1.85]) {
        rod(v(x, 1.85, z), v(x * 1.12, 0.15, z), 0.08, steel);
        sphere(0.12, "#343e37", x * 1.12, 0.1, z, scene, 1, 0.7, 1);
      }
      rod(v(-1.85, 0.65, z), v(1.85, 0.65, z), 0.06, steel);
    }
    rod(v(0, 0.6, -2.4), v(0, 0.6, 2.4), 0.07, steel);
    const netmat = new T.MeshStandardMaterial({
      color: "#edf2df",
      transparent: true,
      opacity: 0.38,
      side: T.DoubleSide,
    });
    const net = new T.Mesh(new T.PlaneGeometry(4.38, 0.43), netmat);
    net.position.set(0, 2.305, 0);
    scene.add(net);
    for (let x = -2.18; x <= 2.2; x += 0.11)
      rod(v(x, 2.09, 0), v(x, 2.52, 0), 0.006, "#e3e7d3");
    for (let y = 2.1; y <= 2.5; y += 0.073)
      rod(v(-2.2, y, 0), v(2.2, y, 0), 0.006, "#e3e7d3");
    rod(v(-2.25, 2.53, 0), v(2.25, 2.53, 0), 0.022, "#f7f3e4");
    for (const x of [-2.28, 2.28]) {
      rod(v(x, 1.6, 0), v(x, 2.58, 0), 0.04, steel);
      box(0.22, 0.1, 0.17, steel, x, 2.06, 0);
    }
  }
  function paddle(parent: T.Object3D, position: T.Vector3, color?: string) {
    const g = new T.Group();
    g.name = "paddle";
    rackets.push(g);
    g.position.copy(position);
    g.rotation.x = -0.55;
    g.rotation.z = -0.55;
    parent.add(g);
    const wood = mat("#d2ad72"),
      rubber = mat(
        color || (themeName === "arena" ? "#f47b60" : "#c64c3e"),
        0.9,
      );
    const disk = new T.Mesh(new T.CylinderGeometry(0.23, 0.23, 0.06, 32), wood);
    disk.rotation.x = Math.PI / 2;
    disk.scale.y = 1;
    disk.scale.z = 1.13;
    g.add(disk);
    const face = new T.Mesh(new T.CircleGeometry(0.216, 32), rubber);
    face.position.z = 0.035;
    face.scale.y = 1.13;
    g.add(face);
    g.userData.face = face;
    rod(v(0, -0.17, 0), v(0, -0.47, 0), 0.046, wood, g);
    return g;
  }
  function bench(x: number, z: number, rotation: number) {
    const g = new T.Group();
    g.position.set(x, 0, z);
    g.rotation.y = rotation;
    scene.add(g);
    const timber = mat(themeName === "heritage" ? "#9d603b" : "#b79764");
    for (const zz of [-0.23, 0, 0.23])
      box(2.6, 0.12, 0.19, timber, 0, 0.58, zz, g);
    for (const xx of [-0.95, 0.95])
      for (const zz of [-0.2, 0.2])
        box(0.085, 0.58, 0.085, "#566452", xx, 0.27, zz, g);
    box(2.6, 0.32, 0.09, timber, 0, 1.03, 0.35, g);
    box(0.55, 0.13, 0.42, theme.away, 0.4, 0.72, 0, g);
    for (const xx of [-0.65, -0.35]) {
      rod(v(xx, 0.66, -0.05), v(xx, 1, -0.05), 0.065, "#c9d8c2", g);
      sphere(0.067, "#f0e8cf", xx, 1, -0.05, g, 1, 0.25, 1);
    }
  }
  function plant(x: number, z: number) {
    const g = new T.Group();
    g.position.set(x, 0, z);
    scene.add(g);
    const pot = new T.Mesh(
      new T.CylinderGeometry(0.34, 0.24, 0.6, 24),
      mat("#b7795b"),
    );
    pot.position.y = 0.3;
    pot.castShadow = true;
    g.add(pot);
    for (let i = 0; i < 9; i++) {
      let angle = i * 2.4;
      let dx = Math.cos(angle) * 0.32,
        dz = Math.sin(angle) * 0.32;
      rod(v(0, 0.5, 0), v(dx, 1.4 + (i % 3) * 0.23, dz), 0.018, "#52724e", g);
      const leaf = sphere(
        0.28,
        i % 2 ? "#557e59" : "#78936b",
        dx,
        1.45 + (i % 3) * 0.2,
        dz,
        g,
        0.6,
        1.7,
        0.22,
      );
      leaf.rotation.z = Math.sin(angle) * 0.7;
      leaf.rotation.y = angle;
    }
  }
  function arenaLights() {
    for (const x of [-6.9, 6.9]) {
      box(0.1, 2.4, 15, "#172b3b", x, 1.2, -0.2);
      box(
        0.05,
        0.055,
        15,
        new T.MeshStandardMaterial({
          color: "#68d5c7",
          emissive: "#41a798",
          emissiveIntensity: 1.1,
        }),
        x - Math.sign(x) * 0.07,
        2.42,
        -0.2,
      );
      for (let z = -5; z < 6; z += 2.5) {
        box(0.08, 0.8, 1.6, "#274658", x - Math.sign(x) * 0.06, 1.3, z);
      }
    }
    for (let i = 0; i < 7; i++) {
      const x = (i - 3) * 1.6;
      box(1.3, 0.38, 0.7, i % 2 ? "#384758" : "#b47464", x, 0.3, -7.1);
      box(1.3, 0.6, 0.15, "#243747", x, 0.75, -7.4);
    }
    for (let x of [-5, 5]) {
      const spot = new T.SpotLight("#e6fff5", 18, 24, Math.PI / 6, 0.8, 1);
      spot.position.set(x, 9, 1);
      spot.target.position.set(0, 0, 0);
      scene.add(spot, spot.target);
      box(0.9, 0.18, 0.4, "#6d908c", x, 7, -6);
    }
  }
  function environment() {
    roundedCourt();
    const upper = box(50, 16, 0.2, theme.wall, 0, 8, -8.7);
    upper.castShadow = false;
    const ceiling = box(
      50,
      0.12,
      40,
      themeName === "arena" ? "#0f2136" : "#eeede2",
      0,
      12,
      1,
    );
    ceiling.castShadow = false;
    for (const x of [-6, 6]) {
      const lamp = box(
        0.16,
        0.04,
        5,
        new T.MeshStandardMaterial({
          color: "#faf4d8",
          emissive: "#fff1cc",
          emissiveIntensity: 0.4,
        }),
        x,
        7.55,
        -1.4,
      );
      lamp.castShadow = false;
    }
    if (themeName === "arena") {
      arenaLights();
      return;
    }
    const backZ = -8.4;
    box(
      15,
      0.4,
      0.3,
      themeName === "club" ? "#698b76" : "#967d5b",
      0,
      0.2,
      backZ,
    );
    box(15, 4.2, 0.25, theme.wall, 0, 2.5, backZ);
    if (themeName === "heritage") {
      for (let x = -5.5; x < 6; x += 3.65) {
        box(2.6, 2.4, 0.04, "#f8eace", x, 3.2, backZ + 0.15);
        for (let xx of [-1.25, 0, 1.25])
          box(0.06, 2.4, 0.07, "#88775a", x + xx, 3.2, backZ + 0.19);
        for (let y of [2.03, 3.2, 4.37])
          box(2.6, 0.06, 0.07, "#88775a", x, y, backZ + 0.2);
      }
      for (let i = 0; i < 16; i++)
        box(15, 0.035, 0.018, "#c5ad87", 0, 0.6 + i * 0.075, backZ + 0.14);
      const clock = new T.Mesh(new T.CircleGeometry(0.34, 40), mat("#f2e8cd"));
      clock.position.set(0, 2.6, backZ + 0.2);
      scene.add(clock);
      line(
        [
          v(-0.14, 2.6, backZ + 0.23),
          v(0, 2.6, backZ + 0.23),
          v(0, 2.82, backZ + 0.23),
        ],
        "#4f5946",
        0.015,
      );
      bench(-5.6, -3, Math.PI / 2);
      bench(5.7, 1, -Math.PI / 2);
      for (let z = -4; z < 6; z += 3.1) {
        const patch = box(
          4,
          0.008,
          1.4,
          new T.MeshStandardMaterial({
            color: "#fff1bb",
            transparent: true,
            opacity: 0.24,
            depthWrite: false,
          }),
          -2.2,
          0.012,
          z,
        );
        patch.rotation.y = -0.5;
        patch.castShadow = false;
      }
    } else {
      for (let x = -6.8; x < 7; x += 0.35)
        box(0.07, 2.4, 0.05, "#b3c1ac", x, 1.55, backZ + 0.15);
      box(5, 0.8, 0.06, "#e2dcc7", 0, 3.4, backZ + 0.16);
      for (const x of [-1.4, 0, 1.4]) {
        const disk = new T.Mesh(
          new T.CircleGeometry(0.21, 40),
          mat(x === 0 ? "#b56d51" : "#6b8b71"),
        );
        disk.position.set(x, 3.4, backZ + 0.21);
        scene.add(disk);
      }
      bench(-5.8, -1.8, Math.PI / 2);
      plant(-6.5, -6.7);
      plant(6.3, -6.4);
    }
    // Side advertising boards are abstract material accents, without logos or tiny text.
    for (const x of [-6.8, 6.8]) {
      box(
        0.12,
        0.85,
        6.1,
        themeName === "club" ? "#dce3d0" : "#69846a",
        x,
        0.45,
        3.7,
      );
      box(0.14, 0.055, 6.12, "#f1e9ce", x, 0.9, 3.7);
    }
  }
  function firstPersonRacket() {
    racketRig = new T.Group();
    scene.add(racketRig);
    scene.add(camera);
    const racket = paddle(racketRig, v(0, 0, 0), "#267c6a");
    racket.rotation.set(-0.1, -0.3, -0.28);
    racket.scale.setScalar(1.68);
    const front = new T.Mesh(
      new T.CircleGeometry(0.213, 48),
      mat("#263b38", 0.93),
    );
    front.position.z = 0.037;
    front.scale.y = 1.13;
    racket.add(front);
    racketRig.traverse((o) => {
      o.castShadow = false;
    });
  }

  scene = new T.Scene();
  scene.background = new T.Color(theme.bg);
  scene.fog = new T.Fog(theme.bg, 32, 60);
  camera = new T.PerspectiveCamera(49, 1, 0.1, 100);
  scene.add(
    new T.HemisphereLight(
      themeName === "arena" ? "#b7cfef" : "#fff5dc",
      themeName === "arena" ? "#18394a" : "#879e83",
      themeName === "arena" ? 1.5 : 2.5,
    ),
  );
  const sun = new T.DirectionalLight(
    themeName === "heritage" ? "#ffe0aa" : "#fff9e9",
    themeName === "arena" ? 1.8 : 3.5,
  );
  sun.position.set(-7, 12, 5);
  sun.castShadow = true;
  sun.shadow.mapSize.set(1024, 1024);
  Object.assign(sun.shadow.camera, {
    left: -11,
    right: 11,
    top: 12,
    bottom: -12,
    near: 1,
    far: 35,
  });
  sun.shadow.normalBias = 0.035;
  sun.shadow.bias = -0.0002;
  sun.shadow.radius = 4;
  scene.add(sun);
  const fill = new T.DirectionalLight(
    themeName === "arena" ? "#71cfdf" : "#c4dbd0",
    themeName === "arena" ? 1.3 : 1,
  );
  fill.position.set(7, 7, -6);
  scene.add(fill);
  environment();
  table();
  for (let i = 0; i < players - 1; i++) {
    const p = paddle(scene, v(0, 2.5, -4), "#bd5548");
    p.scale.setScalar(1.5);
  }
  firstPersonRacket();
  ball = sphere(
    0.055,
    new T.MeshStandardMaterial({
      color: "#fff9df",
      emissive: "#fff5bd",
      emissiveIntensity: 0.32,
    }),
  );
  ball.castShadow = false;
  ball.add(
    new T.Mesh(
      new T.SphereGeometry(0.061, 16, 12),
      new T.MeshBasicMaterial({ color: "#365248", side: T.BackSide }),
    ),
  );
  ballShadow = new T.Mesh(
    new T.CircleGeometry(0.12, 24),
    new T.MeshBasicMaterial({
      color: "#183a32",
      transparent: true,
      opacity: 0.24,
      depthWrite: false,
    }),
  );
  ballShadow.rotation.x = -Math.PI / 2;
  scene.add(ballShadow);
  const trailPoints = new Float32Array(18);
  streak = new T.Line(
    new T.BufferGeometry().setAttribute(
      "position",
      new T.BufferAttribute(trailPoints, 3),
    ),
    new T.LineBasicMaterial({
      color: themeName === "arena" ? "#bafcf0" : "#ffeab0",
      transparent: true,
      opacity: 0.75,
    }),
  );
  scene.add(streak);
  function resize() {
    const w = viewport.clientWidth,
      h = viewport.clientHeight;
    if (!w || !h) return;
    renderer.setSize(w, h, false);
    camera.aspect = w / h;
    const setup = cameraSetup(w, h);
    camera.position.set(0, setup.y, setup.z);
    camera.fov = setup.fov;
    camera.lookAt(0, setup.targetY, -1.1);
    camera.updateProjectionMatrix();
    camera.updateMatrixWorld();
  }
  resize();
  renderer.compile(scene, camera);
  function render(f: CourtFrame) {
    const d = f.data;
    const seat = f.seat >= 0 ? f.seat : 0;
    const team = d.options.teams[seat] ?? 0;
    const facing = team ^ d.ends;
    const scale = 2.75;
    let p = {
      x: 0,
      y: 0.95,
      z: ((d.options.teams[d.expected] ?? 0) ^ d.ends) === 0 ? 1.25 : -1.25,
    };
    if (d.flight) p = sampleFlight(d.flight, f.time);
    if (f.ball) p = f.ball;
    const q = localPoint(p, facing);
    ball.position.set(q.x * scale, q.y * scale, q.z * scale);
    ball.visible = d.phase !== "game_break" && d.phase !== "point_break";
    const depth = -ball.position.clone().applyMatrix4(camera.matrixWorldInverse)
      .z;
    const radius = Math.max(
      0.055,
      Math.min(
        0.12,
        ((2 * Math.tan((camera.fov * Math.PI) / 360) * Math.max(1, depth)) /
          Math.max(1, viewport.clientHeight)) *
          6,
      ),
    );
    ball.scale.setScalar(radius / 0.055);
    const overTable = Math.abs(q.x) <= 0.7825 && Math.abs(q.z) <= 1.39;
    ballShadow.visible = ball.visible;
    ballShadow.position.set(
      q.x * scale,
      (overTable ? 0.76 : 0) * scale + 0.008,
      q.z * scale,
    );
    ballShadow.scale.setScalar(1 + Math.max(0, q.y - 0.76) * 0.5);
    streak.visible =
      ball.visible &&
      !!d.flight &&
      d.phase === "rally" &&
      !f.predicted &&
      !f.reduced;
    if (d.flight && streak.visible) {
      for (let i = 0; i < 6; i++) {
        const a = localPoint(sampleFlight(d.flight, f.time - i * 12), facing);
        trailPoints[i * 3] = a.x * scale;
        trailPoints[i * 3 + 1] = a.y * scale;
        trailPoints[i * 3 + 2] = a.z * scale;
      }
      streak.geometry.attributes.position.needsUpdate = true;
      streak.geometry.computeBoundingSphere();
    }
    const others = d.options.teams.map((_, i) => i).filter((i) => i !== seat);
    let opponent = 0;
    for (let i = 0; i < others.length; i++) {
      const other = others[i],
        racket = rackets[i],
        ally = d.options.teams[other] === team;
      const paddleX = d.paddles[other]?.x ?? 0;
      const base = ally
        ? v(paddleX * 0.9 * scale, 2.4, 4.5)
        : v(-paddleX * 0.9 * scale, 2.6, -4.2);
      opponent++;
      racket.position.copy(base);
      racket.rotation.set(
        -0.1,
        ally ? -0.25 : 0.1,
        ally ? 0.45 : other % 2 ? 0.3 : -0.3,
      );
      const face = racket.userData.face as T.Mesh<
        T.CircleGeometry,
        T.MeshStandardMaterial
      >;
      face.material.color.set(ally ? "#328b78" : "#bd5548");
      const hit = [...d.events]
        .reverse()
        .find(
          (e) =>
            e.type === "hit" &&
            e.seat === other &&
            e.at <= f.time &&
            f.time - e.at < 220,
        );
      if (hit) {
        const at = localPoint(hit.p, facing);
        const k = Math.exp(-(f.time - hit.at) / 85);
        racket.position.lerp(
          v(at.x * scale + 0.15, (at.y + 0.12) * scale, at.z * scale),
          k,
        );
        racket.rotation.z +=
          Math.sin(((f.time - hit.at) / 220) * Math.PI) * 0.7;
      }
    }
    const age = f.swing ? f.now - f.swing.at : Infinity;
    const activeSwing = age >= 0 && age < 380 && !f.gesture.down;
    const incoming = localPoint(contactPoint(d, seat, f.time), facing);
    const serving =
      d.expected === seat && (d.phase === "serve" || d.phase === "serve_toss");
    let pose =
      (serving || f.gesture.down || d.controls.armed) && d.expected === seat
        ? incoming
        : { x: 0, y: 0.82, z: 1.7 };
    if (activeSwing && f.swing?.contact)
      pose = localPoint(f.swing.contact, facing);
    let paddleX = d.paddles[seat]?.x ?? 0;
    if (f.racketScreenX !== null)
      paddleX = racketFromScreen(
        f.racketScreenX,
        pose,
        viewport.clientWidth,
        viewport.clientHeight,
      );
    if (activeSwing) paddleX = f.swing?.stroke.racketX ?? paddleX;
    if (serving) paddleX = 0.34 / 0.9;
    let x = paddleX * 0.9;
    if (activeSwing && f.swing?.connected)
      x += Math.max(-0.06, Math.min(0.06, pose.x - x));
    const k = activeSwing ? Math.sin((age / 380) * Math.PI) : 0;
    racketRig.position.set(
      x * scale,
      (pose.y +
        (activeSwing || f.gesture.down || d.controls.armed || serving
          ? 0.12
          : 0)) *
        scale,
      pose.z * scale - k * 0.08,
    );
    const own = rackets[rackets.length - 1];
    own.rotation.set(pose.y < 1.1 ? -0.75 : -0.25, -0.1, 0.9);
    racketRig.rotation.set(
      k * (f.swing?.stroke.kind === "chop" ? 0.12 : -0.12),
      0,
      k * (f.swing?.stroke.kind === "chop" ? 0.12 : -0.12),
    );

    renderer.render(scene, camera);
    const screen = ball.position.clone().project(camera);
    const contactScreen = new T.Vector3(
      incoming.x * scale,
      incoming.y * scale,
      incoming.z * scale,
    ).project(camera);
    const ownRacket = rackets[rackets.length - 1]
      .getWorldPosition(new T.Vector3())
      .project(camera);
    Object.assign(canvas, {
      __tableTennisFrame: {
        at: f.time,
        predicted: !!f.predicted,
        swingAge: f.swing ? f.now - f.swing.at : null,
        gestureDown: f.gesture.down,
        racket: {
          x: ((ownRacket.x + 1) / 2) * viewport.clientWidth,
          y: ((1 - ownRacket.y) / 2) * viewport.clientHeight,
        },
        controls: d.controls,
        contact: {
          x: ((contactScreen.x + 1) / 2) * viewport.clientWidth,
          y: ((1 - contactScreen.y) / 2) * viewport.clientHeight,
        },
        inputEnabled: !!f.inputEnabled,
        phase: d.phase,
        rally: d.rally,
        shot: d.shot,
        ball: {
          x: ((screen.x + 1) / 2) * viewport.clientWidth,
          y: ((1 - screen.y) / 2) * viewport.clientHeight,
          visible: ball.visible,
        },
        rackets: rackets.length,
        facing,
        calls: renderer.info.render.calls,
      },
    });
  }
  function dispose() {
    scene.traverse((o) => {
      if (o instanceof T.DirectionalLight || o instanceof T.SpotLight)
        o.shadow.dispose();
      const mesh = o as T.Mesh;
      if (mesh.geometry) mesh.geometry.dispose();
      if (mesh.material) {
        for (const material of Array.isArray(mesh.material)
          ? mesh.material
          : [mesh.material]) {
          const m = material as T.MeshStandardMaterial;
          m.map?.dispose();
          material.dispose();
        }
      }
    });
    renderer.dispose();
    // A replacement effect may reuse the same attached canvas. Release the
    // actual context only after this owned canvas has left the document.
    setTimeout(() => {
      if (!canvas.isConnected) renderer.forceContextLoss();
    }, 0);
  }
  return {
    render,
    resize,
    dispose,
    stats: () => ({
      calls: renderer.info.render.calls,
      triangles: renderer.info.render.triangles,
    }),
  };
}
