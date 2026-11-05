export const characters = [
  "幻海",
  "藏马",
  "桑原",
  "幽助",
  "飞影",
  "仙水",
  "阵",
  "树",
  "户愚吕兄",
  "酎",
  "户愚吕弟",
] as const;
export const arenas = [
  { id: 0, name: "默认场地" },
  { id: 2, name: "灵界" },
  { id: 4, name: "亚空间 1" },
  { id: 6, name: "魔界" },
  { id: 8, name: "洞窟" },
  { id: 10, name: "魔性之森" },
  { id: 12, name: "亚空间 2" },
  { id: 14, name: "山顶" },
] as const;
export function defaults(capacity: number) {
  if (capacity === 2) return [3, 9];
  if (capacity === 3) return [3, 1, 9];
  if (capacity === 4) return [3, 1, 0, 10];
  throw Error("比赛支持 2 至 4 人");
}
