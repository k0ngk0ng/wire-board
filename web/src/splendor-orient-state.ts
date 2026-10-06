import type { Card, GemPlayer, Room } from "./types";

export const gemNames = [
  "祖母绿",
  "钻石",
  "蓝宝石",
  "缟玛瑙",
  "红宝石",
  "黄金",
];
export const orientNames = {
  gold: "黄金卡",
  copy: "复制卡",
  copy_cascade: "复制连锁卡",
  double: "双宝石卡",
  cascade: "连锁卡",
  sacrifice: "弃牌购买卡",
};
export function gemBonus(card: Card) {
  return card.color < 0 || card.color > 4
    ? 0
    : card.orient === "double"
      ? 2
      : card.bonusCount || 1;
}
export function gemIsCopy(card: Card) {
  return card.orient === "copy" || card.orient === "copy_cascade";
}
export function gemCanAcquire(player: GemPlayer, card: Card) {
  return (
    !gemIsCopy(card) ||
    player.cards.some((c) => c.id !== card.id && gemBonus(c) > 0)
  );
}
export function gemCardName(card: Card) {
  return card.orient
    ? `东方${orientNames[card.orient]}`
    : `${gemNames[card.color]}发展卡`;
}
export function gemCardDescription(card: Card) {
  const bonus = gemBonus(card)
    ? `永久提供 ${gemBonus(card)} 枚${gemNames[card.color]}折扣。`
    : "";
  switch (card.orient) {
    case "gold":
      return "没有颜色。购买时可弃置，提供两枚虚拟黄金，至少使用一枚；不占十枚筹码上限。";
    case "copy":
    case "copy_cascade":
      return (
        (bonus || "取得后与自己已有的永久奖励卡配对，复制其颜色和奖励数量。") +
        (card.orient === "copy_cascade"
          ? "然后免费取得一张公开的一级卡，并结算其效果。"
          : "")
      );
    case "cascade":
      return `${bonus}免费取得一张公开的二级卡，并结算其效果。`;
    case "sacrifice":
      return `${bonus}购买时弃置两张${gemNames[card.sacrificeColor ?? 0]}发展卡，不支付筹码；优先弃置该颜色的复制卡。`;
    default:
      return bonus;
  }
}
export function gemGoldNeeded(
  player: GemPlayer,
  cost: number[],
  spent?: number[],
) {
  const value = player.tradingPosts?.includes(4) ? 2 : 1;
  return cost.reduce(
    (total, need, color) =>
      total +
      Math.ceil(
        Math.max(
          0,
          need - player.bonus[color] - (spent?.[color] ?? player.tokens[color]),
        ) / value,
      ),
    0,
  );
}
export function gemPaymentCards(player: GemPlayer, card: Card) {
  return player.cards.filter((c) =>
    card.orient === "sacrifice"
      ? c.color === (card.sacrificeColor ?? 0)
      : c.orient === "gold",
  );
}
export function gemPaymentError(
  player: GemPlayer,
  card: Card,
  tokens: number[],
  ids: number[],
) {
  if (!gemCanAcquire(player, card)) return "先拥有一张可配对的永久奖励卡";
  if (new Set(ids).size !== ids.length) return "不能重复使用同一张卡";
  const choices = gemPaymentCards(player, card);
  if (ids.some((id) => !choices.some((c) => c.id === id)))
    return "支付卡牌已经失效，请重新选择";
  if (card.orient === "sacrifice") {
    if (ids.length !== 2) return "请选择两张指定颜色的发展卡";
    const copies = choices.filter(gemIsCopy).length;
    if (
      choices.filter((c) => ids.includes(c.id) && gemIsCopy(c)).length !==
      Math.min(2, copies)
    )
      return "必须优先弃置该颜色的复制卡";
    return tokens.some(Boolean) ? "弃牌购买不支付筹码" : "";
  }
  if (
    tokens.length !== 6 ||
    tokens.some(
      (n, color) => !Number.isInteger(n) || n < 0 || n > player.tokens[color],
    )
  )
    return "所持宝石不足";
  if (
    tokens
      .slice(0, 5)
      .some(
        (n, color) => n > Math.max(0, card.cost[color] - player.bonus[color]),
      )
  )
    return "支付了不需要的宝石";
  const virtual = gemGoldNeeded(player, card.cost, tokens) - tokens[5];
  if (!ids.length) return virtual === 0 ? "" : "黄金不足以补足费用";
  if (virtual < ids.length) return "每张黄金卡至少需要使用一枚虚拟黄金";
  return virtual > ids.length * 2 ? "黄金不足以补足费用" : "";
}
export function gemDefaultPayment(
  player: GemPlayer,
  card: Card,
  ids: number[] = [],
  spent?: number[],
) {
  if (card.orient === "sacrifice") return [0, 0, 0, 0, 0, 0];
  const tokens = card.cost.map(
    (need, color) =>
      spent?.[color] ??
      Math.min(Math.max(0, need - player.bonus[color]), player.tokens[color]),
  );
  return [
    ...tokens,
    Math.max(0, gemGoldNeeded(player, card.cost, tokens) - ids.length * 2),
  ];
}
export function gemCanBuy(player: GemPlayer, card: Card) {
  if (!gemCanAcquire(player, card)) return false;
  if (card.orient === "sacrifice")
    return gemPaymentCards(player, card).length >= 2;
  const need = gemGoldNeeded(player, card.cost);
  return (
    need <=
    player.tokens[5] +
      player.cards.filter((c) => c.orient === "gold").length * 2
  );
}

export function gemOrientChoices(room: Room) {
  const g = room.game,
    s = g?.splendor;
  if (
    !g ||
    !s ||
    room.status !== "playing" ||
    room.spectating ||
    g.finished ||
    g.turn !== room.you ||
    room.seats[room.you]?.autoPlay ||
    s.players[room.you]?.eliminated
  )
    return [];
  const effect = s.effects?.[0];
  if (g.phase === "gem_copy" && effect?.kind === "copy") {
    return s.players[room.you].cards.filter(
      (c) => c.id !== effect.card && gemBonus(c) > 0,
    );
  }
  if (g.phase === "gem_free_card" && effect?.kind === "free_card")
    return s.freeCardChoices ?? [];
  return [];
}
