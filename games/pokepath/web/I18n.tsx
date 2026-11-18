import { Children, cloneElement, createContext, isValidElement, useContext, useMemo, type ReactNode } from "react";

const pairs = "網网 設设 圖图 體体 顯显 傷伤 語语 題题 關关 閉闭 開开 啟启 隊队 倉仓 庫库 實实 單单 數数 據据 當当 無无 選选 擇择 夥伙 寶宝 夢梦 種种 類类 預预 萬万 聖圣 節节 經经 獲获 總总 幣币 擁拥 閃闪 暈晕 減减 燒烧 詛诅 記记 錄录 紀纪 場场 徑径 擊击 敗败 敵敌 屬属 態态 範范 圍围 揮挥 掃扫 進进 繼继 續续 刪删 儲储 輸输 讀读 個个 資资 訊讯 訓训 練练 詳详 買买 賣卖 裝装 備备 級级 戰战 鬥斗 暫暂 領领 項项 顏颜 標标 籤签 線线 歸归 還还 賦赋 護护 隱隐 隨随 機机 優优 負负 載载 務务 時时 間间 陣阵 賞赏 勵励 點点 傳传 飛飞 聲声 靜静 樂乐 後后 這这 應应 該该 殺杀 額额 凍冻 結结 鎖锁 達达 換换 輕轻 貓猫 龍龙 葉叶 頭头 極极 劍剑 氣气 獨独 強强 盜盗 獸兽 遊游 會会 讓让 並并 墊垫 擲掷 覽览 檢检 驗验 艦舰 與与 稱称 製制 處处 試试 確确 認认 裡里 獎奖 彈弹 檔档 區区 動动 僅仅 終终 環环 轟轰 編编 輯辑 視视";
const simplified = new Map(pairs.split(" ").map(pair => [pair[0], pair[1]]));
const normalize = (value: string) => [...value].map(character => simplified.get(character) ?? character).join("");
const clean = (value: string) => value.replace(/\[\/?[A-Z_]+\]/g, "");
type Locale = { language: number; fact: (path: string) => string; translate: (value: string) => string; localize: (value: ReactNode) => ReactNode };
const context = createContext<Locale>({ language: 8, fact: () => "", translate: value => value, localize: value => value });
type Trie = { next: Map<string, Trie>; value?: string[] };

function locale(language: number, dictionary: unknown, catalog: unknown): Locale {
  const words = new Map<string, string[]>();
  const english = new Map<string, string[]>();
  const visited = new WeakSet<object>();
  const add = (values: string[]) => {
    if (!values[8] || !/[\u3000-\u9fff]/.test(values[8])) return;
    if (!words.has(normalize(values[8]))) words.set(normalize(values[8]), values);
    const plain = values.map(clean);
    if (!words.has(normalize(plain[8]))) words.set(normalize(plain[8]), plain);
    if (values[0]) english.set(values[0].toLowerCase().trim(), values);
  };
  const visit = (value: unknown) => {
    if (!value || typeof value !== "object" || visited.has(value)) return;
    visited.add(value);
    if (Array.isArray(value) && value.length >= 9 && value.every(entry => typeof entry === "string")) add(value);
    else for (const child of Object.values(value)) visit(child);
  };
  visit(dictionary);
  visit(catalog);
  const platformWords = [
    ["Community Discord", "Discord", "Discord", "Discord da comunidade", "Discord", "Community-Discord", "コミュニティのDiscord", "커뮤니티 디스코드", "社區 DISCORD", "Discord"],
    ["Pause / Resume", "Pausa / Continuar", "Pause / Reprendre", "Pausar / Continuar", "Pausa / Riprendi", "Pause / Fortsetzen", "一時停止 / 再開", "일시 정지 / 계속", "暫停／繼續", "Pauza / Wznów"]
  ];
  for (const values of platformWords) words.set(normalize(values[8]), values);
  const aliases: Record<string, string> = { "傷害統計": "wave data", "伤害统计": "wave data", "載入隊伍": "load team", "訓練家": "profile", "倉庫": "box", "背包": "inventory", "詳情": "info", "資料": "info", "夥伴": "pokémon", "升級": "level up", "添加": "add", "移除": "remove", "裝備": "equip", "卸下": "unequip", "保存": "save", "套用": "load", "下一波": "next wave", "暫停": "pause", "繼續": "resume", "速度": "speed", "波次": "wave", "空位": "empty", "已鎖定": "locked", "關閉": "off", "開": "on", "全部": "all", "標題": "title", "預設": "default", "姓名": "name", "名稱": "name", "路線": "route", "圖鑑": "tutorials", "主題": "theme", "字體": "font", "原版": "original", "經典": "classic", "萬聖節": "halloween", "單色": "monochrome", "總音量": "volume master", "音樂": "volume music", "音乐": "volume music", "音效": "audio", "特效": "effect", "操作": "keyboard shortcuts", "社區": "community discord", "編輯器": "wave creator", "波次編輯器": "wave creator", "關閉視窗": "close window", "空格": "space", "視窗": "window", "界面音效": "volume ui", "戰鬥音效": "volume effects", "確定": "confirm", "取消": "cancel", "搜索": "search", "顯示傷害": "show damage", "顯示生命值數字": "health numbers", "地圖效果": "map effects", "場地指示器": "field indicator", "形狀指示器": "shape indicator", "快速目標選擇器": "fast target selector", "控制條": "crowd control bars", "顯示粒子": "show particles", "停用彩色標記": "disable tag colors" };
  for (const [key, en] of Object.entries(aliases)) { const value = english.get(en); if (value && !words.has(normalize(key))) words.set(normalize(key), value); }
  const root: Trie = { next: new Map() };
  for (const [key, value] of words) {
    let node = root;
    for (const character of key) { if (!node.next.has(character)) node.next.set(character, { next: new Map() }); node = node.next.get(character)!; }
    node.value = value;
  }
  const cache = new Map<string, string>();
  const selected = (values: string[]) => values[language] || values[0] || values[8] || "";
  const translate = (value: string) => {
    if (cache.has(value)) return cache.get(value)!;
    const source = normalize(value);
    const exact = words.get(source);
    if (exact) { const result = selected(exact); cache.set(value, result); return result; }
    let result = "";
    for (let offset = 0; offset < source.length;) {
      let node = root;
      let match: { value: string[]; end: number } | undefined;
      for (let position = offset; position < source.length; position++) {
        const next = node.next.get(source[position]);
        if (!next) break;
        node = next;
        if (node.value) match = { value: node.value, end: position + 1 };
      }
      if (match) { result += selected(match.value); offset = match.end; }
      else { result += value[offset]; offset++; }
    }
    if (cache.size > 10000) cache.clear();
    cache.set(value, result);
    return result;
  };
  const localize = (value: ReactNode): ReactNode => {
    if (typeof value === "string") return translate(value);
    if (Array.isArray(value)) return Children.map(value, localize);
    if (!isValidElement<Record<string, unknown>>(value) || value.props["data-i18n-literal"]) return value;
    const props: Record<string, unknown> = {};
    for (const key of ["aria-label", "title", "placeholder", "alt"]) if (typeof value.props[key] === "string") props[key] = translate(value.props[key]);
    if (value.props.children !== undefined) props.children = localize(value.props.children as ReactNode);
    return cloneElement(value, props);
  };
  const fact = (path: string): string => {
    let value: unknown = dictionary;
    for (const key of path.split(".")) value = value && typeof value === "object" ? (value as Record<string, unknown>)[key] : undefined;
    return Array.isArray(value) && value.every(entry => typeof entry === "string") ? selected(value) : typeof value === "string" ? value : "";
  };
  return { language, translate, localize, fact };
}

export function I18nProvider({ language, dictionary, catalog, children }: { language: number; dictionary?: unknown; catalog?: unknown; children: ReactNode }) {
  const value = useMemo(() => locale(language, dictionary, catalog), [language, dictionary, catalog]);
  return <context.Provider value={value}>{children}</context.Provider>;
}
export const useLocalize = () => useContext(context).localize;
export const useTranslate = () => useContext(context).translate;

export const useLanguage = () => useContext(context).language;
export const useFact = () => useContext(context).fact;
export const useTextFact = (path: string) => useContext(context).fact(path);
