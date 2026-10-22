// Metadata extracted from data/classic-2002.json. Source numbers are retained;
// these are not claims about later regional map numbering. Tests check every
// entry against the original archive inventory without bundling map geometry.
export type ClassicMapOption = {
  id: string;
  name: string;
  family: string;
  sourceNumber: number;
  sourceName: string;
  preview: string;
  width: number;
  height: number;
};

const families: { family: string; name: string; sizes: number[][] }[] = [
  {
    family: "Village",
    name: "小区",
    sizes: [
      [0, 520, 440],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
      [9, 600, 520],
    ],
  },
  {
    family: "Forest",
    name: "森林",
    sizes: [
      [0, 600, 520],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
      [9, 600, 520],
      [10, 600, 520],
      [11, 600, 520],
      [12, 600, 520],
      [13, 600, 520],
    ],
  },
  {
    family: "Desert",
    name: "沙漠",
    sizes: [
      [0, 600, 520],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
      [9, 600, 520],
    ],
  },
  {
    family: "Ice",
    name: "冰原",
    sizes: [
      [0, 600, 520],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
    ],
  },
  {
    family: "Factory",
    name: "工厂",
    sizes: [
      [0, 600, 520],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
      [9, 600, 520],
      [10, 600, 520],
      [11, 600, 520],
      [12, 600, 520],
    ],
  },
  {
    family: "Mine",
    name: "矿山",
    sizes: [
      [0, 600, 520],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
      [9, 600, 520],
      [10, 600, 520],
      [11, 600, 520],
      [12, 600, 520],
      [13, 600, 520],
      [14, 600, 520],
    ],
  },
  {
    family: "Patrit",
    name: "海盗船",
    sizes: [
      [0, 600, 520],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
      [9, 600, 520],
      [10, 600, 520],
      [11, 600, 520],
      [12, 600, 520],
      [13, 600, 520],
    ],
  },
  {
    family: "Ocean",
    name: "海洋",
    sizes: [
      [0, 600, 520],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
      [9, 600, 520],
      [10, 600, 520],
      [11, 600, 520],
      [12, 600, 520],
      [13, 600, 520],
      [14, 600, 520],
      [15, 600, 520],
    ],
  },
  {
    family: "Xmas",
    name: "圣诞",
    sizes: [
      [0, 600, 520],
      [1, 600, 520],
      [2, 600, 520],
      [3, 600, 520],
      [4, 600, 520],
      [5, 600, 520],
      [6, 600, 520],
      [7, 600, 520],
      [8, 600, 520],
      [9, 600, 520],
      [10, 600, 520],
    ],
  },
];

export const classicMapGroups = families.map(({ family, name, sizes }) => ({
  family,
  label: name,
  maps: sizes.map(([sourceNumber, width, height]): ClassicMapOption => {
    const number = String(sourceNumber).padStart(2, "0");
    const id = `classic2002-${family.toLowerCase()}${number}`;
    return {
      id,
      name: `经典${name} ${number}`,
      family,
      sourceNumber,
      sourceName: `Map/${family}${number}`,
      preview: `${id}.png`,
      width,
      height,
    };
  }),
}));

export const classicMapByID: Record<string, ClassicMapOption> =
  Object.fromEntries(
    classicMapGroups.flatMap((group) => group.maps.map((map) => [map.id, map])),
  );
export const classicMapNames: Record<string, string> = Object.fromEntries(
  Object.values(classicMapByID).map((map) => [map.id, map.name]),
);
export function previewSources(assets: string, filename: string): string[] {
  const path = `/bubbleparty/v4/classic/previews/${filename}`;
  return assets ? [assets.replace(/\/$/, "") + path, path] : [path];
}

// These IDs describe the existing game layouts. They are not aliases for
// similarly named maps in the source archive.
export const currentMapOptions = [
  { id: "community10", name: "经典小区10", family: "Village" },
  { id: "forest", name: "森林公园", family: "Forest" },
  { id: "desert", name: "沙丘营地", family: "Desert" },
  { id: "waterpark", name: "水乐园", family: "Ocean" },
];
export const mapChoiceGroups = classicMapGroups.map((group) => ({
  ...group,
  maps: [
    ...group.maps,
    ...currentMapOptions.filter((map) => map.family === group.family),
  ],
}));
