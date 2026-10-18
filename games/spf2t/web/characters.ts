export type CharacterMeta = {
  id: string;
  fallbackName: string;
  portrait: string;
  accent: string;
  ink: string;
};

// Names come from the server View. This file only maps stable ids to art and
// colors, so adding a character on the server without art still renders.
export const CHARACTER_META: Record<string, CharacterMeta> = {
  morrigan: {
    id: "morrigan",
    fallbackName: "莫莉卡",
    portrait: "portraits/morrigan.webp",
    accent: "#8e6aa8",
    ink: "#f3e9ff",
  },
  chunli: {
    id: "chunli",
    fallbackName: "春丽",
    portrait: "portraits/chunli.webp",
    accent: "#4f7fb5",
    ink: "#eaf3ff",
  },
  ryu: {
    id: "ryu",
    fallbackName: "隆",
    portrait: "portraits/ryu.webp",
    accent: "#b56d4f",
    ink: "#fff0e8",
  },
  ken: {
    id: "ken",
    fallbackName: "肯",
    portrait: "portraits/ken.webp",
    accent: "#c45c5c",
    ink: "#fff0ed",
  },
  hsienko: {
    id: "hsienko",
    fallbackName: "泪泪",
    portrait: "portraits/hsienko.webp",
    accent: "#5f9a8a",
    ink: "#e9fff8",
  },
  donovan: {
    id: "donovan",
    fallbackName: "多诺万",
    portrait: "portraits/donovan.webp",
    accent: "#b79a52",
    ink: "#fff8df",
  },
  felicia: {
    id: "felicia",
    fallbackName: "菲莉西亚",
    portrait: "portraits/felicia.webp",
    accent: "#bf7fa8",
    ink: "#fff0f8",
  },
  sakura: {
    id: "sakura",
    fallbackName: "樱",
    portrait: "portraits/sakura.webp",
    accent: "#c98b57",
    ink: "#fff3e6",
  },
};

export const characterMeta = (id?: string): CharacterMeta =>
  (id && CHARACTER_META[id]) || {
    id: id || "unknown",
    fallbackName: "未知",
    portrait: "",
    accent: "#5b6270",
    ink: "#f0f2f6",
  };

export const portraitURL = (base: string, id: string) =>
  `${base.replace(/\/$/, "")}/spf2t/v1/${characterMeta(id).portrait}`;

export const portraitSources = (base: string, id: string) => {
  const meta = characterMeta(id);
  if (!meta.portrait) return [];
  const local = `/spf2t/v1/${meta.portrait}`;
  return base ? [portraitURL(base, id), local] : [local];
};
