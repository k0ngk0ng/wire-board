import type { CatanState } from "./types";

// Native image coordinates calibrated against the printed component outlines
// in the pinned 2025 rulebooks (base p11, extension p6). The source and final
// hex centers define a similarity transform: no independent axis stretching.
const artwork = {
  long: {
    width: 401,
    height: 1259,
    first: [198.82, 176.98],
    last: [198.12, 1087.14],
  },
  short: {
    width: 393,
    height: 936,
    first: [195.41, 169.88],
    last: [195.41, 768.82],
  },
  extended: {
    width: 346,
    height: 824,
    first: [175.4, 148.34],
    last: [170.17, 675.66],
  },
};

export function catanRiverImages(g: CatanState) {
  return (g.rivers?.map.channels || []).map((channel, index) => {
    const kind = index === 0 ? "long" : index === 1 ? "short" : "extended";
    const art = artwork[kind];
    const first = g.tiles[channel.tiles[0]];
    const last = g.tiles[channel.tiles[channel.tiles.length - 1]];
    const sx = art.last[0] - art.first[0],
      sy = art.last[1] - art.first[1];
    const dx = last.x - first.x,
      dy = last.y - first.y;
    const divisor = sx * sx + sy * sy;
    const a = (dx * sx + dy * sy) / divisor;
    const b = (dy * sx - dx * sy) / divisor;
    const e = first.x - a * art.first[0] + b * art.first[1];
    const f = first.y - b * art.first[0] - a * art.first[1];
    return {
      ...art,
      kind,
      tiles: channel.tiles,
      matrix: [a, b, -b, a, e, f],
      transform: `matrix(${a} ${b} ${-b} ${a} ${e} ${f})`,
    };
  });
}

export function catanCoinReason(
  g: CatanState,
  seat: number,
  color: number,
  buy: boolean,
) {
  const r = g.rivers,
    p = g.players[seat];
  if (
    !r ||
    !p ||
    p.eliminated ||
    !p.resources ||
    color < 0 ||
    color >= g.bank.length ||
    (color >= 5 && (buy || !g.citiesKnights))
  )
    return "当前不能兑换金币";
  if (buy) {
    if (r.bought >= 2) return "本次行动已购买两张资源";
    if (r.gold[seat] < 2) return "需要2金币";
    if (g.bank[color] <= 0) return "银行没有这种资源";
  } else {
    if (p.resources[color] < p.rates[color])
      return `需要${p.rates[color]}张同类资源`;
    if (r.bank <= 0 && r.goldRule !== "ledger") return "金币供给不足";
  }
  return "";
}
