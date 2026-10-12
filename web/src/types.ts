import type {
  CatanNewWorldMap,
  CatanPlayer,
  CatanKnight,
  CatanProgressEvent,
  CatanCitiesKnights,
  CatanRevealedEvent,
  CatanWagon,
  CatanTwoChoice,
  CatanAttackKnight,
  CatanAttackMove,
  CatanAttackCityKnight,
  CatanAttackCity,
  CatanAttack,
  CatanTransport,
  CatanState,
  CatanOptions,
  RoomOptions as catanRoomOptions,
} from "../../games/catan/web/types";
export type {
  CatanNewWorldMap,
  CatanPlayer,
  CatanKnight,
  CatanProgressEvent,
  CatanCitiesKnights,
  CatanRevealedEvent,
  CatanWagon,
  CatanTwoChoice,
  CatanAttackKnight,
  CatanAttackMove,
  CatanAttackCityKnight,
  CatanAttackCity,
  CatanAttack,
  CatanTransport,
  CatanState,
  CatanOptions,
} from "../../games/catan/web/types";
import type {
  Card,
  GemCity,
  Noble,
  SplendorCardEvent,
  SplendorTokenEvent,
  GemPlayer,
  SplendorOptions,
  SplendorState,
  RoomOptions as splendorRoomOptions,
} from "../../games/splendor/web/types";
export type {
  Card,
  GemCity,
  Noble,
  SplendorCardEvent,
  SplendorTokenEvent,
  GemPlayer,
  SplendorOptions,
  SplendorState,
} from "../../games/splendor/web/types";
import type {
  RailMapSpec,
  RailPayment,
  Ticket,
  RailPlayer,
  City,
  Route,
  Catalog,
  RailState,
  RoomOptions as railRoomOptions,
} from "../../games/rail/web/types";
export type {
  RailMapSpec,
  RailPayment,
  Ticket,
  RailPlayer,
  City,
  Route,
  Catalog,
  RailState,
} from "../../games/rail/web/types";
import type {
  CarFeature,
  CarDefinition,
  CarTile,
  CarPlacement,
  CarcassonneState,
  RoomOptions as carcassonneRoomOptions,
} from "../../games/carcassonne/web/types";
export type {
  CarFeature,
  CarDefinition,
  CarTile,
  CarPlacement,
  CarcassonneState,
} from "../../games/carcassonne/web/types";
import type {
  SGOptions,
  SGCard,
  SGGeneral,
  SGPlayer,
  ThreeV3State,
  SanguoshaState,
  RoomOptions as sanguoshaRoomOptions,
} from "../../games/sanguosha/web/types";
export type {
  SGOptions,
  SGCard,
  SGGeneral,
  SGPlayer,
  ThreeV3State,
  SanguoshaState,
} from "../../games/sanguosha/web/types";
import type {
  DotaOrder,
  DotaPlayer,
  DotaCardSpec,
  DotaHeroSpec,
  DotaItemSpec,
  DotaState,
  RoomOptions as dotaRoomOptions,
} from "../../games/dota/web/types";
export type {
  DotaOrder,
  DotaPlayer,
  DotaCardSpec,
  DotaHeroSpec,
  DotaItemSpec,
  DotaState,
} from "../../games/dota/web/types";
export type User = {
  id: string;
  name: string;
  role?: "player" | "admin" | "superadmin";
};
export type Seat = User & {
  ready: boolean;
  left: boolean;
  bot?: boolean;
  autoPlay?: boolean;
};
export type ChatMessage = {
  spectator?: boolean;
  id: string;
  sender: User;
  text: string;
  sentAt: number;
};
export type State = {
  availableGames: string[];
  railMaps: RailMapSpec[];
  user: User;
  rooms: Room[];
  room?: Room;
  serverNow: number;
  receivedAt: number;
  assetsBaseURL?: string;
};
export type Act = (action: Record<string, unknown>) => Promise<void>;
export type GameHeader = {
  data?: unknown;
  actors?: number[];
  kind: string;
  turn: number;
  phase: string;
  round: number;
  finished: boolean;
  winners: number[];
  log: string[];
};
export type RoomBase = {
  options?: unknown;
  result?: {
    rated: boolean;
    players: (User & { ratingDelta?: number; rank?: number; bot?: boolean })[];
  };
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
  closeReason?: string;
  locked: boolean;
  version: number;
  you: number;
  updated: number;
};
export type Game = GameHeader & {
  dota?: DotaState;
  sanguosha?: SanguoshaState;
  carcassonne?: CarcassonneState;
  catan?: CatanState;
  splendor?: SplendorState;
  rail?: RailState;
};
export type Room = RoomBase &
  catanRoomOptions &
  splendorRoomOptions &
  railRoomOptions &
  carcassonneRoomOptions &
  sanguoshaRoomOptions &
  dotaRoomOptions & { game?: Game };
