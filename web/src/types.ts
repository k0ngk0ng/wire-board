export type User = { id: string; name: string };
export type Seat = User & { ready: boolean; left: boolean; bot?: boolean };
export type ChatMessage = {
  spectator?: boolean;
  id: string;
  sender: User;
  text: string;
  sentAt: number;
};
export type Card = {
  id: number;
  tier: number;
  points: number;
  color: number;
  cost: number[];
};
export type Noble = { id: number; cost: number[] };
export type GemPlayer = {
  eliminated?: boolean;
  tokens: number[];
  bonus: number[];
  reserved?: Card[];
  reservedCount?: number;
  cards: Card[];
  nobles: Noble[];
  score: number;
};
export type Ticket = {
  id: number;
  a: number;
  b: number;
  points: number;
  complete: boolean;
};
export type RailPlayer = {
  eliminated?: boolean;
  hand?: number[];
  handCount: number;
  tickets?: Ticket[];
  ticketCount: number;
  trains: number;
  score: number;
  routeScore: number;
  ticketScore: number;
  longest: number;
  bonus: number;
  completed?: number;
};
export type Game = {
  kind: string;
  turn: number;
  phase: string;
  round: number;
  finished: boolean;
  winners: number[];
  log: string[];
  splendor?: {
    bank: number[];
    market: Card[][];
    nobles: Noble[];
    players: GemPlayer[];
    remaining: number[];
    lastRound: boolean;
  };
  rail?: {
    hiddenDrawId?: number;
    hiddenDrawEvents?: { id: number; player: number }[];
    face: number[];
    faceVersion?: number[];
    players: RailPlayer[];
    owners: Record<string, number>;
    pending?: Ticket[];
    setup: boolean;
    setupReady?: boolean[];
    drawn: number;
    lastRemaining: number;
    ticketsRemaining: number;
    remaining: number;
  };
};
export type Room = {
  spectating?: boolean;
  spectatorCount?: number;
  chat?: ChatMessage[] | null;
  turnDeadline?: number;
  id: string;
  name: string;
  kind: string;
  host: string;
  capacity: number;
  seats: Seat[];
  status: string;
  locked: boolean;
  version: number;
  you: number;
  game?: Game;
  updated: number;
};
export type State = {
  user: User;
  rooms: Room[];
  room?: Room;
  serverNow: number;
  receivedAt: number;
  assetsBaseURL?: string;
};
export type City = {
  id: number;
  name: string;
  x: number;
  y: number;
  label?: number[];
};
export type Route = {
  id: number;
  a: number;
  b: number;
  length: number;
  color: number;
  segments?: { x: number; y: number; angle: number }[];
};
export type Catalog = { cities: City[]; routes: Route[]; tickets: Ticket[] };
export type Act = (action: Record<string, unknown>) => Promise<void>;
