// Card IDs are independent of terrain IDs: 5 is paper in a hand and desert on the map.
export const catanCardNames = [
  "木材",
  "砖块",
  "羊毛",
  "粮食",
  "矿石",
  "纸张",
  "布料",
  "钱币",
];
export const catanCardColors = [
  "#286540",
  "#bb633c",
  "#8cac48",
  "#dbad43",
  "#7a8390",
  "#417e3c",
  "#a4b84e",
  "#8795a1",
];
const resources = ["wood", "brick", "wool", "grain", "ore"];
const commodities = ["paper", "cloth", "coin"];
export function catanCardImage(assets: string, card: number, small = false) {
  if (card < 0 || card > 7 || !Number.isInteger(card)) return undefined;
  return card < 5
    ? `${assets}/catan/${small ? "icon" : "resource"}-${resources[card]}-v1.webp`
    : `${assets}/catan/cities-knights/commodity-${commodities[card - 5]}-v1.webp`;
}
export function catanCardSupply(count: number, fiveSix: boolean) {
  return Array.from({ length: count }, (_, card) =>
    card < 5 ? (fiveSix ? 24 : 19) : fiveSix ? 18 : 12,
  );
}

export function catanBankReason(
  give: number[],
  take: number[],
  hand: number[],
  bank: number[],
  rates: number[],
) {
  if (!give.some((n) => n > 0) || !take.some((n) => n > 0))
    return "请选择给出和换取的牌";
  if (![take, hand, bank, rates].every((v) => v.length === give.length))
    return "牌桌状态更新中";
  let units = 0;
  for (let i = 0; i < give.length; i++) {
    if (give[i] > hand[i]) return `${catanCardNames[i]}持有不足`;
    if (take[i] > bank[i]) return `银行的${catanCardNames[i]}不足`;
    if (give[i] > 0 && take[i] > 0) return "不能交换同一种牌";
    if (rates[i] <= 0 || give[i] % rates[i] !== 0)
      return `${catanCardNames[i]}需按${rates[i]}:1交换`;
    units += give[i] / rates[i];
  }
  return units === take.reduce((a, b) => a + b, 0)
    ? ""
    : `本次可从银行换取${units}张牌`;
}
