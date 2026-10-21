import * as T from "three";
import { CAR_MODELS, bodySurface } from "./car-definitions";
import { mergeGeometries } from "three/examples/jsm/utils/BufferGeometryUtils.js";
export type VehicleModel = {
  root: T.Group;
  wheels: T.Group[];
  frontWheels: T.Group[];
  brakes: T.MeshBasicMaterial;
  skin: number;
};
const box = (
  w: number,
  h: number,
  d: number,
  material: T.Material,
  x: number,
  y: number,
  z: number,
) => {
  const mesh = new T.Mesh(new T.BoxGeometry(w, h, d), material);
  mesh.position.set(x, y, z);
  mesh.castShadow = true;
  mesh.receiveShadow = true;
  return mesh;
};
function shell(points: number[][], depth: number, material: T.Material) {
  const s = new T.Shape();
  points.forEach(([x, y], i) => (i ? s.lineTo(x, y) : s.moveTo(x, y)));
  s.closePath();
  const g = new T.ExtrudeGeometry(s, {
    depth,
    bevelEnabled: true,
    bevelSegments: 2,
    steps: 1,
    bevelSize: 0.65,
    bevelThickness: 0.65,
    curveSegments: 3,
  });
  g.translate(0, 0, -depth / 2);
  g.computeVertexNormals();
  const m = new T.Mesh(g, material);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}
function mergeDirect(group: T.Group) {
  const buckets = new Map<T.Material, T.BufferGeometry[]>();
  for (const child of [...group.children])
    if (child instanceof T.Mesh && !Array.isArray(child.material)) {
      child.updateMatrix();
      const geo = child.geometry.index
        ? child.geometry.toNonIndexed()
        : child.geometry.clone();
      geo.applyMatrix4(child.matrix);
      const bucket = buckets.get(child.material) ?? [];
      bucket.push(geo);
      buckets.set(child.material, bucket);
      group.remove(child);
      child.geometry.dispose();
    }
  for (const [material, geos] of buckets) {
    const geometry = mergeGeometries(geos, false);
    geos.forEach((g) => g.dispose());
    if (geometry) {
      const mesh = new T.Mesh(geometry, material);
      mesh.castShadow = true;
      mesh.receiveShadow = true;
      group.add(mesh);
    }
  }
}
export function createVehicle(color: string, skin: number): VehicleModel {
  const root = new T.Group(),
    wheels: T.Group[] = [],
    frontWheels: T.Group[] = [],
    model = CAR_MODELS[skin] ?? CAR_MODELS[0];
  const paint = new T.MeshPhysicalMaterial({
    color,
    metalness: 0.52,
    roughness: 0.25,
    flatShading:
      model.id === "lamborghini-countach" || model.id === "ferrari-f40",
    clearcoat: 1,
    clearcoatRoughness: 0.16,
  });
  const trim = new T.MeshStandardMaterial({
      color: "#10151b",
      roughness: 0.65,
      metalness: 0.3,
    }),
    metal = new T.MeshStandardMaterial({
      color: "#bdc4cd",
      metalness: 0.8,
      roughness: 0.25,
    });
  const glass = new T.MeshPhysicalMaterial({
    color: "#243c4f",
    metalness: 0.52,
    roughness: 0.12,
    clearcoat: 1,
  });
  const roof = Math.max(...model.roof.map((p) => p[1]));
  const surface = bodySurface(model),
    body = new T.BufferGeometry();
  body.setAttribute(
    "position",
    new T.Float32BufferAttribute(surface.vertices, 3),
  );
  body.setIndex(surface.indices);
  body.setAttribute(
    "uv",
    new T.Float32BufferAttribute(
      new Array((surface.vertices.length / 3) * 2).fill(0),
      2,
    ),
  );
  body.computeVertexNormals();
  const bodyMesh = new T.Mesh(body, paint);
  bodyMesh.castShadow = true;
  root.add(bodyMesh);
  root.add(box(41, 2, 24, trim, 0, 3, 0));
  root.add(shell(model.roof, model.roofWidth, glass));
  root.add(
    box(
      model.id === "porsche-911" ? 8 : 11,
      0.6,
      model.roofWidth - 0.5,
      paint,
      -1,
      roof + 0.4,
      0,
    ),
  );
  // Roof pillars, rear deck, hood and mirrors keep a recognizable coupe silhouette.
  if (model.vents)
    for (let x = -20; x <= -13; x += 2)
      root.add(box(0.7, 0.3, 13, trim, x, 10.8, 0));
  for (const side of [-1, 1]) {
    const pillar = box(1.25, 7, 1.2, paint, -8, 13, side * 9.6);
    pillar.rotation.z = -0.55;
    root.add(pillar);
    root.add(box(1.1, 6.5, 1.1, paint, 4, 13, side * 9.6));
    root.add(box(3.2, 1.5, 3, paint, 8, 10.2, side * 12));
    root.add(box(27, 1, 1.1, paint, -1, 4.4, side * 12));
    root.add(box(4, 0.7, 0.5, metal, -4, 9, side * 12));
  }
  const tireMat = new T.MeshStandardMaterial({
    color: "#111315",
    roughness: 0.9,
  });
  for (const x of [-14, 14])
    for (const z of [-12, 12]) {
      const wheel = new T.Group();
      wheel.position.set(x, 4.6, z);
      const rubber = new T.Mesh(
        new T.CylinderGeometry(4.6, 4.6, 3.1, 16),
        tireMat,
      );
      rubber.rotation.x = Math.PI / 2;
      rubber.castShadow = true;
      wheel.add(rubber);
      const rim = new T.Mesh(
        new T.CylinderGeometry(3.15, 3.15, 3.25, 12),
        metal,
      );
      rim.rotation.x = Math.PI / 2;
      wheel.add(rim);
      const hub = new T.Mesh(new T.CylinderGeometry(1, 1, 3.5, 12), trim);
      hub.rotation.x = Math.PI / 2;
      wheel.add(hub);
      for (let k = 0; k < 5; k++) {
        const spoke = box(0.65, 5.5, 3.35, trim, 0, 0, 0);
        spoke.rotation.z = (k * Math.PI) / 5;
        wheel.add(spoke);
      }
      root.add(wheel);
      wheels.push(wheel);
      if (x > 0) frontWheels.push(wheel);
    }
  const lights = new T.MeshBasicMaterial({
    color: "#e1f5ff",
    toneMapped: false,
  });
  const brakes = new T.MeshBasicMaterial({
    color: "#f73e42",
    toneMapped: false,
  });
  if (model.id === "ferrari-f40")
    root.add(box(0.6, 4.4, 21, trim, -23.1, 7.2, 0));
  const roundLamp = (
    x: number,
    y: number,
    z: number,
    radius: number,
    material: T.Material,
  ) => {
    const lamp = new T.Mesh(
      new T.CylinderGeometry(radius, radius, 0.7, 18),
      material,
    );
    lamp.rotation.z = Math.PI / 2;
    lamp.position.set(x, y, z);
    root.add(lamp);
  };
  for (const side of [-1, 1]) {
    if (model.head === "round") {
      roundLamp(21.8, 8.7, side * 8.5, 2.8, trim);
      roundLamp(22.2, 8.7, side * 8.5, 2.2, lights);
    } else if (model.head === "quad")
      for (const offset of [5.5, 9])
        roundLamp(22.2, 8, side * offset, 1.4, lights);
    else root.add(box(0.5, 1, 6, lights, 22.4, 6.5, side * 7));
    if (model.tail === "round" || model.tail === "oval")
      for (const offset of [5.8, 9.2]) {
        roundLamp(
          -23.2,
          7.3,
          side * offset,
          model.id === "nissan-r34" && offset > 8 ? 2.1 : 1.65,
          trim,
        );
        roundLamp(
          -23.6,
          7.3,
          side * offset,
          model.id === "nissan-r34" && offset > 8 ? 1.7 : 1.3,
          brakes,
        );
      }
    else if (model.tail === "rect")
      for (const offset of [4.5, 7.2, 9.9])
        root.add(box(0.6, 1.6, 2.1, brakes, -23.4, 7.7, side * offset));
    else if (model.tail === "ell") {
      root.add(box(0.6, 2.8, 3.2, brakes, -23.1, 8, side * 9));
      root.add(box(0.6, 1.2, 6, brakes, -23.1, 7, side * 7.5));
    } else root.add(box(0.7, 1.4, 9.6, brakes, -23.2, 8, side * 5));
    root.add(box(2, 1.2, 2, metal, -23.3, 3.5, side * 7.5));
  }
  root.add(box(0.6, 2.2, 11, trim, 22.8, 4.8, 0));
  root.add(box(0.5, 2.2, 9, trim, -23.2, 4.9, 0));
  if (model.id === "bmw-m3")
    for (const z of [-1.8, 1.8]) root.add(box(0.7, 2.3, 2.5, trim, 23, 7.4, z));
  if (model.wing === "duck") {
    root.add(box(6, 1, 25, paint, -19.5, 12, 0));
    root.add(box(4, 1, 21, trim, -19.5, 12.6, 0));
  }
  if (model.wing === "wide" || model.wing === "bridge") {
    const height =
      model.id === "ferrari-f40" ? 18 : model.id === "toyota-supra" ? 20 : 17;
    root.add(
      box(
        3.8,
        1.2,
        26,
        model.id === "ferrari-f40" ? paint : trim,
        -19,
        height,
        0,
      ),
    );
    for (const side of [-1, 1])
      root.add(
        box(
          model.wing === "bridge" ? 4 : 1.5,
          height - 10,
          1.3,
          paint,
          -19,
          (height + 10) / 2,
          side * (model.wing === "bridge" ? 11.5 : 8.5),
        ),
      );
  }
  if (model.id === "ferrari-f40" || model.id === "lamborghini-countach")
    for (const side of [-1, 1]) root.add(box(6, 0.8, 3, trim, 14, 9, side * 6));
  wheels.forEach(mergeDirect);
  mergeDirect(root);
  return { root, wheels, frontWheels, brakes, skin };
}
