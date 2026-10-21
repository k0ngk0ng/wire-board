export type CarModel = {
  id: string;
  name: string;
  body: number[][];
  roof: number[][];
  roofWidth: number;
  tail: "strip" | "rect" | "round" | "oval" | "ell";
  head: "round" | "slit" | "quad";
  wing: "duck" | "wide" | "bridge" | "none";
  vents: boolean;
};
// Original stylized meshes, not scans or extracted manufacturer/game assets.
export const CAR_MODELS: CarModel[] = [
  {
    id: "porsche-911",
    name: "保时捷 911 Turbo",
    body: [
      [-23, 10.5, 7.8],
      [-20, 12.1, 10.5],
      [-14, 12.4, 11],
      [-5, 11.1, 10.1],
      [7, 11.1, 9.4],
      [16, 12, 9.5],
      [21, 10.9, 8.7],
      [23, 9, 6.8],
    ],
    roof: [
      [-15, 10],
      [-9, 15],
      [-4, 18],
      [4, 17.4],
      [12, 10],
    ],
    roofWidth: 18.6,
    tail: "strip",
    head: "round",
    wing: "duck",
    vents: true,
  },
  {
    id: "lamborghini-countach",
    name: "兰博基尼 Countach",
    body: [
      [-23, 11.5, 9.4],
      [-20, 12.5, 10.5],
      [-13, 12.5, 10.7],
      [-5, 11.3, 10.2],
      [7, 11.8, 8.6],
      [17, 11.7, 6.9],
      [21, 10.6, 5.8],
      [23, 9.5, 5],
    ],
    roof: [
      [-11, 10.5],
      [-7, 15],
      [3, 15],
      [12, 8.6],
    ],
    roofWidth: 19.2,
    tail: "rect",
    head: "slit",
    wing: "wide",
    vents: true,
  },
  {
    id: "ferrari-f40",
    name: "法拉利 F40",
    body: [
      [-23, 11.8, 8.6],
      [-20, 12.5, 10.2],
      [-13, 12.3, 10.5],
      [-5, 11.5, 10],
      [7, 11.7, 8.8],
      [16, 12.3, 8],
      [21, 10.8, 6.3],
      [23, 9.6, 5.5],
    ],
    roof: [
      [-12, 10.2],
      [-7, 16],
      [4, 16],
      [12, 9],
    ],
    roofWidth: 18.2,
    tail: "round",
    head: "slit",
    wing: "bridge",
    vents: true,
  },
  {
    id: "nissan-r34",
    name: "日产 GT-R R34",
    body: [
      [-23, 10.8, 8.6],
      [-20, 12, 9.5],
      [-13, 12, 10.7],
      [-5, 11.8, 10.8],
      [7, 11.8, 10.6],
      [16, 12, 10.1],
      [21, 11.2, 9.2],
      [23, 10, 7.6],
    ],
    roof: [
      [-12, 10.4],
      [-7, 18],
      [6, 18],
      [14, 10.2],
    ],
    roofWidth: 19.2,
    tail: "round",
    head: "quad",
    wing: "wide",
    vents: false,
  },
  {
    id: "toyota-supra",
    name: "丰田 Supra A80",
    body: [
      [-23, 10, 7.4],
      [-20, 12.2, 10.1],
      [-13, 12.3, 10.8],
      [-5, 11.5, 10],
      [7, 11.7, 9.6],
      [16, 12.1, 9],
      [21, 10.6, 7.6],
      [23, 8.7, 5.8],
    ],
    roof: [
      [-14, 10.1],
      [-7, 16.6],
      [3, 17],
      [12, 9.6],
    ],
    roofWidth: 18.5,
    tail: "oval",
    head: "slit",
    wing: "bridge",
    vents: false,
  },
  {
    id: "bmw-m3",
    name: "宝马 M3 E46",
    body: [
      [-23, 10.1, 8.1],
      [-20, 11.8, 10],
      [-13, 12, 10.8],
      [-5, 11.6, 11],
      [7, 11.5, 10.7],
      [16, 11.9, 10],
      [21, 10.6, 8.6],
      [23, 9.2, 6.8],
    ],
    roof: [
      [-12, 10.4],
      [-7, 18.3],
      [5, 18.3],
      [13, 10.5],
    ],
    roofWidth: 19,
    tail: "ell",
    head: "quad",
    wing: "none",
    vents: false,
  },
];
export function bodySurface(model: CarModel): {
  vertices: number[];
  indices: number[];
} {
  const vertices: number[] = [],
    indices: number[] = [];
  for (const [x, w, h] of model.body) {
    const ring = [
      [3.6, -w * 0.9],
      [h - 1.5, -w],
      [h, -w * 0.78],
      [h + 0.3, 0],
      [h, w * 0.78],
      [h - 1.5, w],
      [3.6, w * 0.9],
      [3.1, 0],
    ];
    for (const [y, z] of ring) vertices.push(x, y, z);
  }
  for (let i = 0; i < model.body.length - 1; i++)
    for (let j = 0; j < 8; j++) {
      const a = i * 8 + j,
        b = (i + 1) * 8 + j,
        an = i * 8 + ((j + 1) % 8),
        bn = (i + 1) * 8 + ((j + 1) % 8);
      indices.push(a, an, b, an, bn, b);
    }
  for (const end of [0, model.body.length - 1]) {
    const center = vertices.length / 3;
    vertices.push(model.body[end][0], 6, 0);
    for (let j = 0; j < 8; j++) {
      const a = end * 8 + j,
        b = end * 8 + ((j + 1) % 8);
      if (end === 0) indices.push(center, b, a);
      else indices.push(center, a, b);
    }
  }
  return { vertices, indices };
}
