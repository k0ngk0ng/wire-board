const militaryArt = new Set([
  "fire_slash",
  "thunder_slash",
  "analeptic",
  "fire_attack",
  "iron_chain",
  "supply_shortage",
  "fan",
  "guding_blade",
  "vine",
  "silver_lion",
  "hualiu",
]);
const windGenerals = new Set([
  "caoren",
  "xiahouyuan",
  "huangzhong",
  "weiyan",
  "xiaoqiao",
  "zhoutai",
  "zhangjiao",
  "yuji",
]);
const fireGenerals = new Set([
  "dianwei",
  "xunyu",
  "wolong",
  "pangtong",
  "taishici",
  "yuanshao",
  "yanliangwenchou",
  "pangde",
]);
const thicketGenerals = new Set([
  "caopi",
  "xuhuang",
  "menghuo",
  "zhurong",
  "sunjian",
  "lusu",
  "dongzhuo",
  "jiaxu",
]);
const mountainGenerals = new Set([
  "zhanghe",
  "dengai",
  "jiangwei",
  "liushan",
  "sunce",
  "erzhang",
  "caiwenji",
  "zuoci",
]);
const godGenerals = new Set([
  "shenguanyu",
  "shenlvmeng",
  "shenzhouyu",
  "shenzhugeliang",
  "shencaocao",
  "shenlvbu",
  "shenzhaoyun",
  "shensimayi",
]);
export const generalArt = (assets: string, id: string) =>
  `${assets}/sanguosha/${id.startsWith("heg_") ? "v9" : id.startsWith("jie_") ? "v8" : godGenerals.has(id) ? "v7" : mountainGenerals.has(id) ? "v6" : thicketGenerals.has(id) ? "v5" : fireGenerals.has(id) ? "v4" : windGenerals.has(id) ? "v3" : "v1"}/generals/${id}.webp`;

const hegemonyCards = new Set([
  "heg_nullification",
  "await_exhausted",
  "known_both",
  "befriend_attacking",
  "six_swords",
  "triblade",
]);
export const cardArt = (assets: string, kind: string) =>
  `${assets}/sanguosha/${hegemonyCards.has(kind) ? "v9" : militaryArt.has(kind) ? "v2" : "v1"}/cards/${kind}.webp`;
