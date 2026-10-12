export type Options = {
  auctionSeconds: number;
  turnSeconds: number;
  tradeSeconds: number;
};
export type Space = {
  id: number;
  name: string;
  kind: string;
  color: string;
  price: number;
  group: number;
  rents: number[];
  buildingCost: number;
  tax: number;
};
export type Property = { owner: number; mortgaged: boolean; buildings: number };
export type Player = {
  cash: number;
  position: number;
  bankrupt: boolean;
  jailed: boolean;
  jailTurns: number;
  jailCards: string[];
};
export type Card = {
  id: number;
  title: string;
  text: string;
  effect: string;
  amount?: number;
  destination?: number;
};
export type Bundle = {
  cash?: number;
  properties?: number[] | null;
  jailCards?: string[] | null;
  redeem?: number[] | null;
};
export type Offer = {
  id: number;
  from: number;
  to: number;
  give: Bundle;
  want: Bundle;
  expires: number;
  fromFee: number;
  toFee: number;
  valid: boolean;
  reason: string;
};
export type PropertyControls = {
  mortgage: boolean;
  redeem: boolean;
  build: boolean;
  sell: boolean;
  sellGroup: boolean;
  mortgageValue: number;
  redeemCost: number;
  buildCost: number;
  sellValue: number;
  groupSellValue: number;
  buildKind: string;
  buildingAuction: boolean;
};
export type Controls = {
  active: boolean;
  roll: boolean;
  end: boolean;
  buy: boolean;
  auction: boolean;
  bid: boolean;
  auctionPass: boolean;
  trade: boolean;
  jailPay: boolean;
  bankrupt: boolean;
  minBid: number;
  cashAvailable: number;
  jailCards: string[];
  bidTargets: number[];
  properties: Record<number, PropertyControls>;
};
export type Motion = {
  id: number;
  kind: "roll" | "move" | "jump";
  player: number;
  from: number;
  to: number;
  steps: number;
  dice: number[];
};
export type View = {
  motionSerial: number;
  motion?: Motion[];
  players: Player[];
  properties: Property[];
  board: Space[];
  turn: number;
  phase: string;
  window: number;
  dice: number[];
  rollSerial: number;
  utilityDice: number[];
  doubles: number;
  houses: number;
  hotels: number;
  auction?: {
    kind: string;
    property: number;
    bid: number;
    bidder: number;
    expires: number;
  } | null;
  debt?: {
    debtor: number;
    creditor: number;
    amount: number;
    reason: string;
  } | null;
  transfer?: { player: number; property: number } | null;
  lastCard?: (Card & { deck: string; player: number; sequence: number }) | null;
  cards: Record<string, Card[]>;
  offers: Offer[];
  controls: Controls;
  eliminated?: number[];
};
export type Send = (
  type: string,
  extra?: Record<string, unknown>,
) => Promise<void>;
