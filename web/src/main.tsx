import { CatanRiversWorldPreview } from "./catan-rivers-world-preview";
import {
  supportsTwoCatanHelpers,
  catanTwoHelpersNote,
} from "./catan-two-helpers";
import { supportsTwoCatanVariants } from "./catan-two-variants";
import { CatanEventPicker } from "./catan-event-picker";
import {
  CATAN_EVENT_CATALOGUE,
  catanEventsSupported,
} from "./catan-event-options";
import { supportsTwoCatanSeafarers } from "./catan-two-seafarers";
import { CatanTwoScenarioPicker } from "./catan-two-setup";
import { CatanTransportSeat } from "./catan-transport";
import { CatanAttackSeat } from "./catan-attack";
import { twoResponder } from "./catan-two-state";
import { caravanResponder, caravanPlacementStep } from "./catan-caravans-state";
import { CatanRiverSeat } from "./catan-rivers";
import { fishResponder } from "./catan-fishing-state";
import {
  CatanFriendlyRobberPicker,
  CatanFriendlyRobberChoice,
  supportsPublicCatanFriendly,
  CatanFriendlyRobberSeat,
} from "./catan-friendly-robber";
import {
  CatanHarborsPicker,
  CatanHarborsSeat,
  CatanHarborsChoice,
  supportsPublicCatanHarbors,
} from "./catan-harbors";
import { catanRuleContext, catanVictoryTarget } from "./catan-rule-context";
import { CatanCitySeat } from "./catan-city";
import { explorerIsHarbor } from "./catan-explorer-state";
import { CatanCitiesKnightsSetup } from "./catan-cities-knights-setup";
import { CatanBasePicker } from "./catan-base-setup";
import { catanBaseLayoutName } from "./catan-base-layout";
import { CatanWorldEditor } from "./catan-world-editor";
import {
  CatanSeafarersPicker,
  catanScenarioVictory,
  catanScenarioName,
} from "./catan-scenarios";
import { CatanRules } from "./catan-rules";
import { ClothPicture } from "./catan-cloth";
import { CatanWonderSeat } from "./catan-wonders";
import { CatanOptionPicker } from "./catan-helpers";
import {
  CatanScenarioPicker,
  CatanFishingSeaPicker,
  supportsPublicCatanFishingSea,
  supportsPublicCatanFishingSeaExtended,
  CatanCombinationKnightsPicker,
  supportsPublicCatanKnightsCombination,
  supportsPublicExplorerKnights,
  isPublicCatanSea,
  isPublicCatanFlexible,
  isPublicCatanExplorer,
} from "./catan-scenario-setup";
import { AdminDashboard } from "./admin";
import { ShieldCheck } from "lucide-react";
import { SanguoshaOptions } from "./sanguosha-options";
import { SplendorMarket } from "./splendor-market";
import { SplendorPurchase } from "./splendor-purchase";
import { OrientCard } from "./splendor-orient-card";
import { extraNobleName, nobleArtwork } from "./splendor-nobles";
import { gemCanBuy, gemNames } from "./splendor-orient-state";
import { SplendorRules } from "./splendor-rules";
import { SplendorCities } from "./splendor-cities";
import {
  gemNobleEligible,
  splendorResultDescription,
} from "./splendor-city-state";
import {
  SplendorOptionPicker,
  SplendorExpansionBoard,
  gemPostNames,
  splendorRulesLabel,
} from "./splendor-expansions";
import type { SGOptions, SplendorOptions, CatanOptions } from "./types";
import React, {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import { createRoot } from "react-dom/client";
import {
  ArrowRight,
  ArrowLeft,
  Plus,
  Users,
  Lock,
  Check,
  Copy,
  LogOut,
  X,
  Volume2,
  VolumeX,
  TrainFront,
  Gem,
  Clock,
  Wifi,
  WifiOff,
  ChevronRight,
  BookOpen,
  ScrollText,
  Eye,
  RotateCcw,
  Flag,
  Minus,
  ZoomIn,
  ZoomOut,
  Crown,
  Bot,
  Hand,
} from "lucide-react";
import type {
  State,
  Room,
  Card,
  Noble,
  Game,
  Catalog,
  Route,
  Ticket,
  Act,
  RailMapSpec,
  RailPayment,
  RailPlayer,
} from "./types";
import "./style.css";
import "./autoplay.css";
import { cityName } from "./cities";
import {
  RailMapPicker,
  RailPaymentEditor,
  RailMapRules,
  railMapNames,
  routePoints,
  railPaymentValid,
  paymentText,
} from "./rail-expansions";
import { GameAudio } from "./audio";
import { Chat } from "./chat";
import { LogLine } from "./log-line";
import { useRailMapControls } from "./rail-map-controls";
import { useTurnTitle } from "./turn-title";
import { catanResultDescription } from "./catan-results";
import {
  ProfileContext,
  PlayerName,
  ProfilePage,
  Leaderboard,
} from "./profiles";
import { HiddenDrawAnimation } from "./hidden-draw-animation";
import { SplendorCardAnimation } from "./splendor-card-animation";
import { SplendorTokenAnimation } from "./splendor-token-animation";
import { AnimatedSlot } from "./animated-slot";
import { DotaBoard, DotaCover, DotaRules } from "./dota";
import { CatanBoard, catanPhases, catanSeafarerPhases } from "./catan";
import { catanColorIndex, catanSeatColor } from "./catan-player-colors";
import "./splendor-layout.css";
import { CarcassonneBoard } from "./carcassonne";
import { SanguoshaBoard, SanguoshaCover, SanguoshaRules } from "./sanguosha";
const gemDisplayOrder = [1, 2, 0, 4, 3, 5];
const RailMapsContext = createContext<RailMapSpec[]>([]);
const AssetsContext = createContext("");
const gemColors = [
  "#218357",
  "#f7f2dd",
  "#2974bb",
  "#343c43",
  "#c94440",
  "#dcac38",
];
const trainColors = [
  "#a35e91",
  "#eee6d1",
  "#357bac",
  "#e3bc38",
  "#dd8540",
  "#39434a",
  "#c84b43",
  "#568757",
  "#e4ba55",
];
const trainNames = [
  "紫色",
  "白色",
  "蓝色",
  "黄色",
  "橙色",
  "黑色",
  "红色",
  "绿色",
  "万能",
];
const playerColors = ["#337ab3", "#cf5347", "#42956b", "#d4a833", "#414753"];
const gameName = (kind: string) =>
  kind === "dota"
    ? "兵线争锋"
    : kind === "sanguosha"
      ? "三国杀"
      : kind === "carcassonne"
        ? "卡卡颂"
        : kind === "catan"
          ? "卡坦岛"
          : kind === "splendor"
            ? "璀璨宝石"
            : "铁路环游";
const statusName: Record<string, string> = {
  waiting: "等待入座",
  playing: "游戏中",
  finished: "已结算",
  closed: "已结束",
};
async function api(path: string, body?: unknown) {
  const res = await fetch("/api" + path, {
    method: body ? "POST" : "GET",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json();
  if (!res.ok)
    throw Object.assign(new Error(data.error || "请求失败"), {
      status: res.status,
    });
  return data;
}
function Gemstone({ color = 0, size = 23 }: { color?: number; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" aria-hidden="true">
      <path
        d="M10 5h20l8 12-18 20L2 17Z"
        fill={gemColors[color]}
        stroke={color === 1 ? "#aaa389" : "#ffffff66"}
        strokeWidth="1.5"
      />
      <path
        d="M10 5l5 12 5 20 5-20 5-12M2 17h36M10 5l15 12L20 5l-5 12L30 5"
        fill="none"
        stroke={color === 1 ? "#b2ab94" : "#fff"}
        opacity=".45"
      />
    </svg>
  );
}
function Logo() {
  return (
    <a className="logo" href="/" aria-label="围桌首页">
      <img src="/favicon.svg" width="36" height="36" alt="" />
      <span>
        围桌<small>WIRE BOARD</small>
      </span>
    </a>
  );
}
function Cover({ kind, mini = false }: { kind: string; mini?: boolean }) {
  return (
    <div className={`cover ${kind} ${mini ? "mini" : ""}`} aria-hidden="true">
      {kind === "dota" ? (
        <DotaCover />
      ) : kind === "sanguosha" ? (
        <SanguoshaCover />
      ) : kind === "carcassonne" ? (
        <svg preserveAspectRatio="xMidYMid slice" viewBox="0 0 520 260">
          <rect width="520" height="260" fill="#bfd0b7" />
          <circle cx="422" cy="59" r="34" fill="#f3d992" />
          <path d="M0 156Q100 96 218 160T520 142V260H0Z" fill="#8fa77f" />
          <path d="M0 204Q135 154 273 202T520 180V260H0Z" fill="#5d8060" />
          <g transform="translate(160 35) rotate(8 100 100)">
            <rect
              x="5"
              y="8"
              width="204"
              height="204"
              rx="7"
              fill="#294a3c"
              opacity=".2"
            />
            {[
              [0, 0],
              [102, 0],
              [0, 102],
              [102, 102],
            ].map(([x, y], i) => (
              <rect
                key={i}
                x={x}
                y={y}
                width="99"
                height="99"
                rx="4"
                fill={i === 2 ? "#93ad69" : "#abc17d"}
                stroke="#eddbac"
                strokeWidth="3"
              />
            ))}
            <path
              d="M1 48Q52 42 51 99M51 103Q48 151 0 157M103 151Q155 151 154 204"
              fill="none"
              stroke="#f3e4b9"
              strokeWidth="11"
            />
            <path
              d="M104 0H200V59L184 69H118L104 59Z"
              fill="#be9971"
              stroke="#ecdbb2"
              strokeWidth="4"
            />
            <path
              d="M116 61V24H128V33H138V24H150V33H160V24H172V33H184V24H194V61"
              fill="#ead5a3"
              stroke="#866c49"
              strokeWidth="2"
            />
            <path d="M147 61V49Q155 38 163 49V61" fill="#856847" />
            <path
              d="M26 178l9-19 9 19M70 136l9-19 9 19M164 120l9-19 9 19"
              fill="#4f8052"
              stroke="#f0dfad"
              strokeWidth="1.5"
            />
            <g transform="translate(64 67) scale(1.25)">
              <path
                d="M-5-13Q0-19 5-13L5-7L14 0L10 5L5 2L10 14H2L0 7L-2 14H-10L-5 2L-10 5L-14 0L-5-7Z"
                fill="#b95546"
                stroke="#f5dfae"
                strokeWidth="2"
              />
            </g>
            <g transform="translate(158 157) scale(1.25)">
              <path
                d="M-5-13Q0-19 5-13L5-7L14 0L10 5L5 2L10 14H2L0 7L-2 14H-10L-5 2L-10 5L-14 0L-5-7Z"
                fill="#376c89"
                stroke="#f5dfae"
                strokeWidth="2"
              />
            </g>
          </g>
          <path d="M34 27h75M34 33h42" stroke="#55726c" opacity=".6" />
        </svg>
      ) : kind === "catan" ? (
        <svg
          preserveAspectRatio="xMidYMid slice"
          viewBox="0 0 520 260"
          aria-hidden="true"
        >
          <rect width="520" height="260" fill="#527f8a" />
          <circle cx="423" cy="58" r="34" fill="#e5c986" opacity=".8" />
          <path
            d="M0 205Q95 174 180 202T350 203T520 193V260H0Z"
            fill="#386671"
          />
          <path
            d="M43 187h43M391 211h51M84 224h55"
            stroke="#acc8bf"
            strokeWidth="3"
            strokeLinecap="round"
            opacity=".5"
          />
          {[
            [220, 75, 0],
            [295, 75, 3],
            [183, 140, 1],
            [257, 140, 2],
            [331, 140, 4],
            [220, 205, 3],
            [295, 205, 0],
          ].map(([x, y, c], i) => (
            <g key={i}>
              <path
                d={`M${x} ${y - 44}l38 22v44l-38 22-38-22v-44Z`}
                fill={
                  ["#34703d", "#b8643d", "#91ac54", "#ddad44", "#78868d"][c]
                }
                stroke="#f2d899"
                strokeWidth="5"
              />
              <circle cx={x} cy={y} r="14" fill="#f5e8b9" />
              <text
                x={x}
                y={y + 5}
                textAnchor="middle"
                fill="#563c24"
                fontSize="16"
              >
                {[6, 4, 5, 8, 10, 3, 9][i]}
              </text>
            </g>
          ))}
          <g fill="#b95845" stroke="#f0d5a0" strokeWidth="2">
            <path d="M217 121v-21l15-13 15 13v21Z" />
            <path d="M285 186v-21l15-13 15 13v21Z" />
          </g>
          <path d="M34 27h75M34 33h42" stroke="#d7c18a" opacity=".6" />
        </svg>
      ) : kind === "splendor" ? (
        <svg preserveAspectRatio="xMidYMid slice" viewBox="0 0 520 260">
          <defs>
            <linearGradient id="gemsky" x2="0" y2="1">
              <stop stopColor="#204e58" />
              <stop offset="1" stopColor="#0c302e" />
            </linearGradient>
          </defs>
          <rect width="520" height="260" fill="url(#gemsky)" />
          <circle cx="400" cy="78" r="54" fill="#d3b469" opacity=".22" />
          <path
            d="M0 180L70 140l45 30 55-50 65 68 76-55 69 46 60-42 80 65v58H0"
            fill="#41655a"
          />
          <path
            d="M0 218l85-20 91 12 110-20 93 22 141-25v73H0"
            fill="#173d39"
          />
          {[0, 1, 2].map((x) => (
            <g
              key={x}
              transform={`translate(${145 + x * 92},${78 + Math.abs(x - 1) * 16}) rotate(${(x - 1) * 14})`}
            >
              <rect
                width="82"
                height="124"
                rx="7"
                fill="#d2b681"
                stroke="#eed5a5"
                strokeWidth="2"
              />
              <rect
                x="6"
                y="7"
                width="70"
                height="81"
                rx="3"
                fill={["#3b674d", "#2c5778", "#763d37"][x]}
              />
              <path
                d="M16 42h50L41 76Z M16 42l12-17h26l12 17"
                fill={["#4fa574", "#559bd0", "#d87162"][x]}
                stroke="#e7d7ad"
              />
              <path
                d="M28 25l13 51 13-51M16 42h50"
                fill="none"
                stroke="#ffffff77"
              />
              <circle cx="22" cy="105" r="9" fill="#f6ebcf" />
              <circle cx="44" cy="105" r="9" fill="#353f41" />
              <circle cx="65" cy="105" r="9" fill="#ac3937" />
            </g>
          ))}
          {[0, 1, 2, 3, 4].map((x) => (
            <g key={x} transform={`translate(${119 + x * 68},224)`}>
              <ellipse cy="4" rx="23" ry="9" fill="#091f1c" />
              <ellipse
                rx="22"
                ry="9"
                fill={gemColors[x]}
                stroke="#f7e8c8"
                strokeWidth="2"
              />
            </g>
          ))}
          <path d="M34 27h75M34 33h42" stroke="#d7c18a" opacity=".6" />
        </svg>
      ) : (
        <svg preserveAspectRatio="xMidYMid slice" viewBox="0 0 520 260">
          <rect width="520" height="260" fill="#c4d6d3" />
          <circle cx="422" cy="60" r="33" fill="#f8df9d" />
          <path
            d="m0 130 83-86 66 74 64-55 103 94 69-58 135 65v96H0"
            fill="#8dafab"
          />
          <path
            d="m0 163 115-54 73 62 74-37 113 52 78-44 67 49v69H0"
            fill="#5d8c80"
          />
          <path d="M0 233Q150 134 520 221v39H0" fill="#d9bd87" />
          <path
            d="M-30 260Q155 140 550 219M-20 278Q150 156 550 237"
            fill="none"
            stroke="#564d44"
            strokeWidth="5"
          />
          <path
            d="M-30 271Q155 147 550 228"
            fill="none"
            stroke="#564d44"
            strokeWidth="23"
            strokeDasharray="4 14"
          />
          <g transform="translate(190 154) rotate(-4)">
            <path
              d="M-39-17q-23-8-13-24t-12-30"
              fill="none"
              stroke="#f8f3df"
              strokeWidth="13"
              strokeLinecap="round"
            />
            <path d="M-62 29h159l-9-14H-59Z" fill="#263f43" />
            <rect x="-45" y="-9" width="76" height="32" rx="8" fill="#c65342" />
            <path d="M-35-12v-27h16v27M16 19v-58h42V19" fill="#284e53" />
            <rect x="24" y="-32" width="24" height="20" fill="#e8c479" />
            <path d="M10-40h55" stroke="#e8c479" strokeWidth="5" />
            {[-27, 5, 42, 76].map((x) => (
              <circle
                key={x}
                cx={x}
                cy="28"
                r="12"
                fill="#263b3d"
                stroke="#d6b97b"
                strokeWidth="4"
              />
            ))}
            <path d="m76 13 23 19H66" fill="#c65342" />
          </g>
          <path d="M34 27h75M34 33h42" stroke="#55726c" opacity=".6" />
        </svg>
      )}
    </div>
  );
}
function Modal({
  title,
  children,
  onClose,
  dismissible = true,
  className = "",
}: {
  title: string;
  dismissible?: boolean;
  className?: string;
  children: React.ReactNode;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const prev = document.activeElement as HTMLElement;
    const el = ref.current;
    el?.querySelector<HTMLElement>("input,button,select")?.focus();
    function key(e: KeyboardEvent) {
      if (e.key === "Escape" && dismissible) onClose();
      if (e.key === "Tab" && el) {
        const nodes = Array.from(
          el.querySelectorAll<HTMLElement>(
            'button:not(:disabled),input,select,[tabindex="0"]',
          ),
        );
        if (!nodes.length) return;
        const first = nodes[0],
          last = nodes[nodes.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      prev?.focus();
    };
  }, []);
  return (
    <div
      className="modal-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget && dismissible) onClose();
      }}
    >
      <div
        ref={ref}
        className={`modal ${className}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <div className="modal-head">
          <h2>{title}</h2>
          {dismissible && (
            <button className="icon-button" onClick={onClose} aria-label="关闭">
              <X size={20} />
            </button>
          )}
        </div>
        {children}
      </div>
    </div>
  );
}
function App() {
  const [adminOpen, setAdminOpen] = useState(
    () => new URLSearchParams(location.search).get("admin") === "1",
  );
  const openAdmin = (value: boolean) => {
    setAdminOpen(value);
    const url = new URL(location.href);
    if (value) url.searchParams.set("admin", "1");
    else url.searchParams.delete("admin");
    history.replaceState(null, "", url);
  };
  const [state, setState] = useState<State>();
  useTurnTitle(state?.room);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [online, setOnline] = useState(false);
  const [create, setCreate] = useState("");
  useEffect(() => {
    if (create && state && !state.availableGames.includes(create))
      setCreate("");
  }, [state?.availableGames, create]);
  const [join, setJoin] = useState<Room>();
  const [joinMode, setJoinMode] = useState<"join" | "watch">("join");
  const [invitedRoom, setInvitedRoom] = useState(
    () =>
      new URLSearchParams(location.search).get("room")?.trim().toLowerCase() ||
      "",
  );
  const invitationAttempt = useRef("");
  const [rules, setRules] = useState(false);
  const [journalRoom, setJournalRoom] = useState("");
  const [closeTableRoom, setCloseTableRoom] = useState("");
  const [profileID, setProfileID] = useState("");
  const [showLeaderboard, setShowLeaderboard] = useState(false);
  const [sound, setSound] = useState(
    () => localStorage.getItem("wb_sound") !== "off",
  );
  const [notice, setNotice] = useState("");
  const seq = useRef(0);
  const previousTurn = useRef("");
  const previousRoom = useRef<Room | undefined>(undefined);
  const audio = useRef<GameAudio>(new GameAudio());
  useEffect(() => {
    setCloseTableRoom("");
    if (state?.room?.status === "playing") {
      window.scrollTo({ top: 0, left: 0, behavior: "instant" });
    }
  }, [state?.room?.id, state?.room?.status]);
  const refresh = async () => {
    const n = ++seq.current;
    try {
      const data = await api("/state");
      if (n === seq.current) {
        if (
          previousRoom.current?.status === "playing" &&
          !previousRoom.current.spectating &&
          !data.room
        ) {
          setNotice("你已离开这张牌桌，可以创建或加入其他房间。");
        }
        previousRoom.current = data.room;
        setState({ ...data, receivedAt: performance.now() });
      }
    } catch (e) {
      if ((e as { status: number }).status === 401) {
        previousRoom.current = undefined;
        setState(undefined);
      } else setError((e as Error).message);
    } finally {
      setLoaded(true);
    }
  };
  useEffect(() => {
    void refresh();
  }, []);
  useEffect(() => {
    if (!state?.user.id) return;
    let stopped = false;
    let socket: WebSocket;
    let timer: ReturnType<typeof setTimeout>;
    let retryDelay = 250;
    const connect = () => {
      socket = new WebSocket(
        `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/ws`,
      );
      socket.onopen = () => {
        retryDelay = 250;
        setOnline(true);
      };
      socket.onmessage = () => void refresh();
      socket.onclose = () => {
        setOnline(false);
        if (!stopped) {
          timer = setTimeout(connect, retryDelay + Math.random() * 150);
          retryDelay = Math.min(retryDelay * 2, 2000);
        }
      };
      socket.onerror = () => socket.close();
    };
    connect();
    const poll = setInterval(() => void refresh(), 15000);
    const focus = () => void refresh();
    window.addEventListener("focus", focus);
    return () => {
      stopped = true;
      clearTimeout(timer);
      clearInterval(poll);
      socket?.close();
      window.removeEventListener("focus", focus);
    };
  }, [state?.user.id]);
  useEffect(() => {
    if (!state?.room?.game) {
      previousTurn.current = "";
      return;
    }
    const r = state.room;
    const sg = r.game!.sanguosha;
    const dota = r.game!.dota;
    const fleetActor =
      twoResponder(r) ??
      caravanResponder(r) ??
      fishResponder(r) ??
      r.game!.catan?.cardEvent?.players[0] ??
      r.game!.catan?.citiesKnights?.pending?.players[0] ??
      r.game!.catan?.seafarers?.pirateIslands?.raid?.rewards[0];
    const key = `${r.id}:${r.game!.round}:${r.game!.turn}:${r.status}:${dota?.sequence || 0}:${dota?.actors?.includes(r.you) || false}:${sg?.pending?.id || 0}:${sg?.pending?.canRespond || false}:${fleetActor ?? -1}:${r.game!.catan?.two?.sequence ?? 0}:${r.game!.catan?.two?.pending ? "two_build" : r.game!.catan?.two?.trade ? "two_trade" : ""}:${r.game!.catan?.caravans?.pending?.kind ?? ""}:${r.game!.catan?.caravans?.sequence ?? 0}:${caravanPlacementStep(r.game!.catan) ?? 0}:${r.game!.catan?.citiesKnights?.pending?.kind ?? ""}:${r.game!.catan?.cardEvent?.kind ?? ""}:${r.game!.catan?.cardEvent ? r.game!.catan.rollId : 0}`;
    if (key !== previousTurn.current && sound) {
      if ((r.game!.finished || r.status === "closed") && previousTurn.current) {
        void audio.current.play("finish");
      } else if (
        r.status === "playing" &&
        !r.seats[r.you]?.autoPlay &&
        (dota
          ? dota.actors?.includes(r.you)
          : sg?.pending
            ? sg.pending.canRespond
            : (fleetActor ?? r.game!.turn) === r.you)
      ) {
        void audio.current.play("turn");
      }
    }
    previousTurn.current = key;
  }, [state?.room?.version, sound]);
  const run = async (fn: () => Promise<unknown>) => {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await fn();
      await refresh();
    } catch (e) {
      setError((e as Error).message);
      await refresh();
    } finally {
      setBusy(false);
    }
  };
  const command = async (
    room: Room,
    type: string,
    extra: Record<string, unknown> = {},
  ) => {
    const body = {
      type,
      version: room.version,
      nonce: Array.from(crypto.getRandomValues(new Uint8Array(16)), (x) =>
        x.toString(16).padStart(2, "0"),
      ).join(""),
      ...extra,
    };
    try {
      return await api(`/rooms/${room.id}`, body);
    } catch (e) {
      if (e instanceof TypeError) return api(`/rooms/${room.id}`, body);
      throw e;
    }
  };
  const act: Act = async (action) => {
    if (!state?.room) return;
    if (state.room.seats[state.room.you]?.autoPlay) {
      setNotice("当前由电脑托管，取消托管后即可继续操作。");
      return;
    }
    await run(async () => {
      await command(state.room!, "action", { action });
      if (sound)
        void audio.current.play(
          action.type === "take"
            ? "take"
            : action.type === "buy"
              ? "buy"
              : action.type === "reserve"
                ? "reserve"
                : "move",
        );
    });
  };
  const clearInvitation = () => {
    const url = new URL(location.href);
    url.searchParams.delete("room");
    history.replaceState(null, "", url);
    setInvitedRoom("");
    invitationAttempt.current = "";
  };
  useEffect(() => {
    if (!state || !invitedRoom || busy || join) return;
    if (state.room?.id === invitedRoom) {
      clearInvitation();
      return;
    }
    if (state.room) {
      const blocked = `occupied:${state.room.id}`;
      if (invitationAttempt.current !== blocked) {
        invitationAttempt.current = blocked;
        setNotice("你正在另一张牌桌，请先离开当前牌桌，再进入邀请的房间。");
      }
      return;
    }
    if (invitationAttempt.current === invitedRoom) return;
    invitationAttempt.current = invitedRoom;
    const target = state.rooms.find((r) => r.id === invitedRoom);
    if (!target || !["waiting", "playing"].includes(target.status)) {
      setNotice("邀请的房间已结束或不存在，可以在大厅加入其他牌桌。");
      clearInvitation();
      return;
    }
    if (target.status === "waiting" && target.seats.length >= target.capacity) {
      setNotice("邀请的房间已满，暂时无法入座；开局后可以观战。");
      clearInvitation();
      return;
    }
    const mode = target.status === "playing" ? "watch" : "join";
    setNotice("");
    if (target.locked) {
      setJoinMode(mode);
      setJoin(target);
      clearInvitation();
      return;
    }
    void run(async () => {
      if (mode === "watch") {
        await api(`/rooms/${target.id}/watch`, {});
        setNotice("这局已经开始，已为你进入观战。");
      } else {
        try {
          await command(target, "join");
        } catch (e) {
          if ((e as { status?: number }).status !== 409) throw e;
          const latest: State = await api("/state");
          if (latest.room?.id !== target.id) {
            const updated = latest.rooms.find((r) => r.id === target.id);
            if (!updated) throw e;
            if (updated.status === "playing") {
              await api(`/rooms/${target.id}/watch`, {});
              setNotice("这局已经开始，已为你进入观战。");
            } else await command(updated, "join");
          }
        }
      }
      clearInvitation();
    });
  }, [state, invitedRoom, busy, join]);
  const roomCommand = (type: string, extra: Record<string, unknown> = {}) =>
    void run(() => command(state!.room!, type, extra));
  const enableAudio = () => {
    void audio.current.unlock();
  };
  const previewAudio = async () => {
    setSound(true);
    localStorage.setItem("wb_sound", "on");
    const played = await audio.current.play("preview");
    setNotice(
      played
        ? "已播放试听音效。如未听到，请检查设备音量和浏览器标签页是否静音。"
        : "浏览器未能播放音效，请检查站点声音权限后再次试听。",
    );
  };
  if (!loaded)
    return (
      <div className="loading">
        <Logo />
        <p>正在铺好桌布…</p>
      </div>
    );
  if (!state)
    return (
      <Auth
        busy={busy}
        error={error}
        invited={!!invitedRoom}
        submit={(body, register) => {
          if (sound) enableAudio();
          void run(() => api(register ? "/register" : "/login", body));
        }}
      />
    );
  const room = state.room;
  const canAdmin =
    state.user.role === "admin" || state.user.role === "superadmin";
  const autoPlay = !!room?.seats[room.you]?.autoPlay;
  const canAutoPlay =
    room?.status === "playing" &&
    !room.spectating &&
    room.you >= 0 &&
    !room.seats[room.you]?.left &&
    !room.game?.sanguosha?.players[room.you]?.dead;
  const boardBusy = busy || autoPlay;
  return (
    <ProfileContext.Provider value={setProfileID}>
      <AssetsContext.Provider value={state.assetsBaseURL || ""}>
        <RailMapsContext.Provider value={state.railMaps || []}>
          <div
            className={state.assetsBaseURL ? "with-artwork" : ""}
            style={
              state.assetsBaseURL
                ? ({
                    "--splendor-cards": `url("${state.assetsBaseURL}/splendor/cards.webp")`,
                    "--splendor-nobles": `url("${state.assetsBaseURL}/splendor/nobles.webp")`,
                    "--splendor-tokens": `url("${state.assetsBaseURL}/splendor/tokens.webp")`,
                    "--rail-cards": `url("${state.assetsBaseURL}/rail/train-cards.webp")`,
                  } as React.CSSProperties)
                : undefined
            }
            onPointerDown={() => {
              if (sound) enableAudio();
            }}
            onKeyDown={() => {
              if (sound) enableAudio();
            }}
          >
            <header className="topbar">
              <Logo />
              <nav>
                <span className="nav-active">
                  {adminOpen ? "管理后台" : room ? "游戏牌桌" : "桌游大厅"}
                </span>
                <span className="nav-caption">让相聚，多一局。</span>
              </nav>
              <div className="header-right">
                {canAdmin && (
                  <button
                    className={`subtle admin-nav-button ${adminOpen ? "active" : ""}`}
                    onClick={() => openAdmin(!adminOpen)}
                  >
                    <ShieldCheck size={16} />
                    {adminOpen ? "返回游戏" : "管理后台"}
                  </button>
                )}
                <button
                  className="subtle"
                  onClick={() => setShowLeaderboard(true)}
                >
                  积分榜
                </button>
                <span
                  className={`connection ${online ? "" : "offline"}`}
                  title={online ? "实时连接正常" : "正在重连，座位与进度已保留"}
                >
                  {online ? <Wifi size={15} /> : <WifiOff size={15} />}
                  <span>{online ? "已连接" : "重连中"}</span>
                </span>
                <button
                  className="icon-button"
                  aria-label={sound ? "关闭音效" : "开启音效"}
                  title={
                    sound ? "音效已开启，点击静音" : "音效已关闭，点击开启"
                  }
                  onClick={() => {
                    setSound(!sound);
                    localStorage.setItem("wb_sound", sound ? "off" : "on");
                    if (!sound) void previewAudio();
                  }}
                >
                  {sound ? <Volume2 size={18} /> : <VolumeX size={18} />}
                </button>
                <button
                  className="sound-preview"
                  onClick={() => void previewAudio()}
                >
                  试听音效
                </button>
                <button
                  className="avatar"
                  aria-label="我的战绩与好友"
                  onClick={() => setProfileID(state.user.id)}
                >
                  {state.user.name[0]}
                </button>
                <span className="username">
                  <PlayerName user={state.user} />
                </span>
                <button
                  className="icon-button"
                  aria-label="退出登录"
                  onClick={() => void run(() => api("/logout", {}))}
                >
                  <LogOut size={17} />
                </button>
              </div>
            </header>
            {error && (
              <div className="toast error" role="alert">
                {error}
                <button aria-label="关闭错误" onClick={() => setError("")}>
                  <X size={16} />
                </button>
              </div>
            )}
            {notice && (
              <div className="toast" role="status">
                {notice}
                <button aria-label="关闭提示" onClick={() => setNotice("")}>
                  <X size={16} />
                </button>
              </div>
            )}
            {adminOpen ? (
              canAdmin ? (
                <AdminDashboard
                  user={state.user}
                  onBack={() => openAdmin(false)}
                  onChange={refresh}
                />
              ) : (
                <main className="admin-entry-denied">
                  <ShieldCheck size={32} />
                  <h2>此页面仅向管理员开放</h2>
                  <button className="outline" onClick={() => openAdmin(false)}>
                    返回大厅
                  </button>
                </main>
              )
            ) : !room ? (
              <Lobby
                state={state}
                onCreate={setCreate}
                onJoin={(r) => {
                  setJoinMode("join");
                  if (r.locked) setJoin(r);
                  else void run(() => command(r, "join"));
                }}
                onWatch={(r) => {
                  setJoinMode("watch");
                  if (r.locked) setJoin(r);
                  else void run(() => api(`/rooms/${r.id}/watch`, {}));
                }}
                busy={busy}
              />
            ) : (
              <main
                className={`room-page ${room.game ? "playing" : ""} game-${room.kind}`}
              >
                <div className="room-heading">
                  <div>
                    <span className="eyebrow">
                      {gameName(room.kind)} <span> / </span> 牌桌{" "}
                      {room.id.toUpperCase()}
                    </span>
                    <h1>{room.name}</h1>
                  </div>
                  <div className="room-tools">
                    {canAutoPlay && (
                      <button
                        className={`subtle autoplay-control ${autoPlay ? "active" : ""}`}
                        aria-pressed={autoPlay}
                        disabled={busy}
                        onClick={() =>
                          roomCommand("autoplay", { enabled: !autoPlay })
                        }
                      >
                        {autoPlay ? <Hand size={16} /> : <Bot size={16} />}
                        {autoPlay ? "取消托管" : "开启托管"}
                      </button>
                    )}
                    {room.spectating && (
                      <button
                        className="subtle"
                        disabled={busy}
                        onClick={() =>
                          void run(() =>
                            api(`/rooms/${room.id}/watch`, { leave: true }),
                          )
                        }
                      >
                        <ArrowLeft size={16} />
                        离开观战
                      </button>
                    )}
                    {room.game && (
                      <button
                        className="subtle"
                        aria-haspopup="dialog"
                        onClick={() => setJournalRoom(room.id)}
                      >
                        <ScrollText size={16} />
                        对局日志
                      </button>
                    )}
                    <button className="subtle" onClick={() => setRules(true)}>
                      <BookOpen size={16} />
                      玩法速查
                    </button>
                    <button
                      className="subtle"
                      onClick={async () => {
                        try {
                          await navigator.clipboard.writeText(
                            `${location.origin}/?room=${room.id}`,
                          );
                          setNotice("房间链接已复制，发给朋友即可");
                        } catch {
                          setNotice(`房间编号：${room.id}`);
                        }
                      }}
                    >
                      <Copy size={16} />
                      邀请朋友
                    </button>
                    {!room.spectating && room.status !== "playing" && (
                      <button
                        className="subtle"
                        disabled={busy}
                        onClick={() => roomCommand("leave")}
                      >
                        <ArrowLeft size={16} />
                        离开房间
                      </button>
                    )}
                    {room.status === "playing" &&
                      !room.spectating &&
                      room.host === state.user.id && (
                        <button
                          className="subtle danger"
                          disabled={busy}
                          aria-haspopup="dialog"
                          onClick={() => setCloseTableRoom(room.id)}
                        >
                          结束牌桌
                        </button>
                      )}
                  </div>
                </div>
                {room.spectating && (
                  <div className="spectator-notice">
                    <Eye size={16} />
                    观战中 · 仅展示公开牌面与玩家信息
                  </div>
                )}
                {autoPlay && canAutoPlay && (
                  <div className="autoplay-notice" role="status">
                    <Bot size={21} aria-hidden="true" />
                    <div>
                      <strong>电脑正在为你托管</strong>
                      <span>离开或关闭网页也会继续，回来后可随时接手。</span>
                    </div>
                    <button
                      className="outline"
                      disabled={busy}
                      onClick={() =>
                        roomCommand("autoplay", { enabled: false })
                      }
                    >
                      恢复手动操作
                    </button>
                  </div>
                )}
                {room.status === "waiting" ? (
                  <Waiting
                    room={room}
                    busy={busy}
                    host={!room.spectating && room.host === state.user.id}
                    command={roomCommand}
                  />
                ) : room.status === "closed" ? (
                  <div className="closed-panel">
                    <Flag size={40} />
                    <h2>
                      {room.closeReason === "unpublished"
                        ? "这款桌游已下架"
                        : room.closeReason === "admin"
                          ? "管理员已结束牌桌"
                          : room.closeReason === "inactive"
                            ? "牌桌已因闲置关闭"
                            : "这张牌桌已结束"}
                    </h2>
                    <p>
                      {room.closeReason === "unpublished"
                        ? "等待中的牌桌已关闭，可以返回大厅选择其他游戏。"
                        : room.closeReason === "admin"
                          ? "本局已中止，不计胜负与积分。"
                          : room.closeReason === "inactive"
                            ? "连续 24 小时无人操作，系统已自动关闭牌桌；本局不计胜负与积分。"
                            : "休息一下，或者准备下一局。"}
                    </p>
                    {!room.spectating &&
                      room.host === state.user.id &&
                      state.availableGames.includes(room.kind) && (
                        <button
                          className="primary"
                          disabled={busy}
                          onClick={() => roomCommand("rematch")}
                        >
                          <RotateCcw size={18} />
                          再开一局
                        </button>
                      )}
                  </div>
                ) : (
                  <>
                    {!room.game?.sanguosha && !room.game?.dota && (
                      <Players room={room} />
                    )}
                    {room.game?.finished && (
                      <Results
                        room={room}
                        host={
                          !room.spectating &&
                          room.host === state.user.id &&
                          state.availableGames.includes(room.kind)
                        }
                        busy={busy}
                        command={roomCommand}
                      />
                    )}
                    <div className="game-layout">
                      <div className="game-main">
                        {room.kind === "dota" ? (
                          <DotaBoard
                            room={room}
                            act={act}
                            busy={boardBusy}
                            assets={state.assetsBaseURL || ""}
                          />
                        ) : room.kind === "sanguosha" ? (
                          <SanguoshaBoard
                            room={room}
                            act={act}
                            busy={boardBusy}
                            assets={state.assetsBaseURL || ""}
                          />
                        ) : room.kind === "carcassonne" ? (
                          <CarcassonneBoard
                            room={room}
                            act={act}
                            busy={boardBusy}
                            assets={state.assetsBaseURL || ""}
                          />
                        ) : room.kind === "catan" ? (
                          <CatanBoard
                            room={room}
                            act={act}
                            resetOpening={() =>
                              run(() => command(room, "catan_explorer_reset"))
                            }
                            busy={boardBusy}
                            assets={state.assetsBaseURL || ""}
                          />
                        ) : room.kind === "splendor" ? (
                          <SplendorBoard
                            room={room}
                            act={act}
                            busy={boardBusy}
                          />
                        ) : (
                          <RailBoard room={room} act={act} busy={boardBusy} />
                        )}
                      </div>
                      <aside className="game-sidebar">
                        <Turn
                          room={room}
                          serverNow={state.serverNow}
                          receivedAt={state.receivedAt}
                        />
                      </aside>
                    </div>
                  </>
                )}
              </main>
            )}
            {!room && !adminOpen && (
              <footer>
                围桌 WIRE BOARD <span>好游戏，和好朋友一起。</span>
                <span>私人牌桌 · 自动保存</span>
              </footer>
            )}
            {create && (
              <Create
                kind={create}
                availableGames={state.availableGames}
                busy={busy}
                onClose={() => setCreate("")}
                onSubmit={(body) =>
                  void run(async () => {
                    await api("/rooms", body);
                    setCreate("");
                  })
                }
              />
            )}
            {join && (
              <Modal
                title={`${joinMode === "watch" ? "观战" : "加入"}「${join.name}」`}
                onClose={() => setJoin(undefined)}
              >
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    const f = new FormData(e.currentTarget);
                    void run(async () => {
                      if (joinMode === "watch")
                        await api(`/rooms/${join.id}/watch`, {
                          password: f.get("password"),
                        });
                      else
                        await command(
                          state.rooms.find((r) => r.id === join.id) || join,
                          "join",
                          {
                            password: f.get("password"),
                          },
                        );
                      setJoin(undefined);
                    });
                  }}
                >
                  <label>
                    房间密码
                    <input
                      name="password"
                      type="password"
                      required
                      autoComplete="off"
                    />
                  </label>
                  <button className="primary wide" disabled={busy}>
                    {joinMode === "watch" ? "开始观战" : "加入牌桌"}
                    <ArrowRight size={18} />
                  </button>
                </form>
              </Modal>
            )}
            {room?.status === "playing" &&
              !room.spectating &&
              room.host === state.user.id &&
              closeTableRoom === room.id && (
                <Modal
                  title="结束当前牌桌？"
                  onClose={() => setCloseTableRoom("")}
                >
                  <div className="close-table-summary">
                    <span className="close-table-icon" aria-hidden="true">
                      <Flag size={24} />
                    </span>
                    <div>
                      <strong>{room.name}</strong>
                      <span>{gameName(room.kind)} · 对局进行中</span>
                    </div>
                  </div>
                  <p>
                    结束后本局不计胜负，也不加减积分。所有玩家可以离开牌桌，或由房主重新开局。
                  </p>
                  <div className="modal-actions">
                    <button
                      className="outline"
                      onClick={() => setCloseTableRoom("")}
                    >
                      继续游戏
                    </button>
                    <button
                      className="primary confirm-danger"
                      disabled={busy}
                      onClick={() => {
                        setCloseTableRoom("");
                        roomCommand("close");
                      }}
                    >
                      确认结束
                    </button>
                  </div>
                </Modal>
              )}
            {showLeaderboard && (
              <Modal
                title="围桌 · 积分排行榜"
                onClose={() => setShowLeaderboard(false)}
              >
                <Leaderboard />
              </Modal>
            )}
            {profileID && (
              <Modal title="玩家页面" onClose={() => setProfileID("")}>
                <ProfilePage
                  key={profileID}
                  id={profileID}
                  self={state.user.id}
                />
              </Modal>
            )}
            {room?.game && journalRoom === room.id && (
              <Modal title="对局日志" onClose={() => setJournalRoom("")}>
                <p>最近的行动在最上方，最多保留 80 条记录。</p>
                <div className="journal journal-dialog">
                  <ol tabIndex={0} aria-label="对局行动记录">
                    {room.game.log
                      .slice(-80)
                      .reverse()
                      .map((line, i) => (
                        <li key={`${room.version}-${i}`}>
                          <LogLine
                            line={line}
                            seats={room.seats}
                            seatColors={
                              room.game?.catan
                                ? room.seats.map((_, seat) =>
                                    catanSeatColor(room.game!.catan!, seat),
                                  )
                                : undefined
                            }
                          />
                        </li>
                      ))}
                    {!room.game.log.length && <li>牌已洗好，祝你好运。</li>}
                  </ol>
                </div>
              </Modal>
            )}
            {room && (
              <Chat
                key={`${state.user.id}:${room.id}`}
                room={room}
                userId={state.user.id}
                send={async (text, nonce) => {
                  await api(`/rooms/${room.id}/chat`, { text, nonce });
                  await refresh();
                }}
              />
            )}
            {rules && room && (
              <Modal
                title={`${gameName(room!.kind)} · 玩法速查`}
                className="rules-modal"
                onClose={() => setRules(false)}
              >
                <Rules
                  kind={room!.kind}
                  room={room}
                  sgOptions={
                    room.game?.sanguosha?.options || room.sanguoshaOptions
                  }
                  railMap={
                    room!.game?.rail?.mapInfo ||
                    state.railMaps?.find(
                      (m) => m.id === (room!.railMap || "usa"),
                    )
                  }
                />
              </Modal>
            )}
          </div>
        </RailMapsContext.Provider>
      </AssetsContext.Provider>
    </ProfileContext.Provider>
  );
}
function Auth({
  busy,
  error,
  invited,
  submit,
}: {
  busy: boolean;
  error: string;
  invited: boolean;
  submit: (body: unknown, register: boolean) => void;
}) {
  const [register, setRegister] = useState(false);
  return (
    <main className="auth-page">
      <section className="auth-story">
        <Logo />
        <div className="auth-copy">
          <span className="eyebrow">A TABLE FOR YOUR PEOPLE</span>
          <h1>
            把朋友，约到
            <br />
            同一张桌上。
          </h1>
          <p>
            收集一颗宝石，连接一座城市。
            <br />
            从一局好游戏，开始今晚的相聚。
          </p>
          <div className="auth-covers">
            <Cover kind="splendor" />
            <Cover kind="rail" />
          </div>
          <div className="auth-caption">
            <span>
              <Gem size={17} />
              卡牌与策略
            </span>
            <span>
              <TrainFront size={17} />
              探索与冒险
            </span>
            <span>经典桌游 · 好友相聚 · 无限好时光</span>
          </div>
        </div>
        <small>YOUR FRIENDS. YOUR TABLE. YOUR NEXT MOVE.</small>
      </section>
      <section className="auth-form">
        <div className="auth-form-inner">
          <span className="eyebrow">WELCOME TO WIRE BOARD</span>
          <h2>{register ? "留一个你的座位" : "欢迎回来，入座吧。"}</h2>
          <p>
            {register
              ? "使用朋友分享的邀请码，加入这间私人桌游室。"
              : invited
                ? "朋友在牌桌等你，登录后将自动进入邀请的房间。"
                : "桌布铺好了，就等你和朋友们。"}
          </p>
          <div className="auth-tabs">
            <button
              className={!register ? "active" : ""}
              onClick={() => setRegister(false)}
            >
              登录
            </button>
            <button
              className={register ? "active" : ""}
              onClick={() => setRegister(true)}
            >
              创建账号
            </button>
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const f = new FormData(e.currentTarget);
              submit(Object.fromEntries(f.entries()), register);
            }}
          >
            <label>
              玩家昵称
              <input
                name="name"
                placeholder="你的桌游昵称"
                required
                minLength={2}
                maxLength={20}
                autoComplete="username"
              />
            </label>
            <label>
              密码
              <input
                name="password"
                type="password"
                placeholder="至少 8 位"
                required
                minLength={8}
                maxLength={72}
                autoComplete={register ? "new-password" : "current-password"}
              />
            </label>
            {register && (
              <label>
                邀请码
                <input
                  name="invite"
                  placeholder="向朋友获取邀请码"
                  required
                  autoComplete="off"
                />
              </label>
            )}
            {error && (
              <p className="inline-error" role="alert">
                {error}
              </p>
            )}
            <button className="primary wide" disabled={busy}>
              {busy ? "请稍候…" : register ? "创建账号，入座" : "进入桌游大厅"}
              <ArrowRight size={18} />
            </button>
          </form>
          <div className="auth-note">
            <Lock size={14} />
            只属于你和朋友的桌游空间
          </div>
        </div>
      </section>
    </main>
  );
}
function Lobby({
  state,
  onCreate,
  onJoin,
  onWatch,
  busy,
}: {
  state: State;
  onCreate: (v: string) => void;
  onJoin: (r: Room) => void;
  onWatch: (r: Room) => void;
  busy: boolean;
}) {
  const [filter, setFilter] = useState("all");
  useEffect(() => {
    if (filter !== "all" && !state.availableGames.includes(filter))
      setFilter("all");
  }, [state.availableGames, filter]);
  const rooms = state.rooms
    .filter(
      (r) =>
        (r.status === "waiting" || r.status === "playing") &&
        (filter === "all" || r.kind === filter),
    )
    .sort((a, b) => b.updated - a.updated);
  const invited = new URLSearchParams(location.search).get("room");
  return (
    <main className="lobby">
      <section className="welcome">
        <div>
          <span className="eyebrow">YOUR NEXT GOOD EVENING</span>
          <h1>
            好朋友，好一局<span>。</span>
          </h1>
          <p>选一款喜欢的桌游，给朋友留个座位。</p>
        </div>
        <div className="welcome-stamp">
          <span>围坐 · 开局 · 尽兴</span>
          <svg
            className="tabletop-mark"
            viewBox="0 0 150 64"
            role="img"
            aria-label="骰子、卡牌与棋子，围桌一起玩"
          >
            <path
              d="M12 54 Q75 66 138 54"
              fill="none"
              stroke="#d7c9ac"
              strokeWidth="2"
            />
            <g transform="translate(13 12) rotate(-12 20 20)">
              <rect width="39" height="39" rx="9" fill="#c9654e" />
              <rect
                x="3"
                y="3"
                width="33"
                height="33"
                rx="7"
                fill="none"
                stroke="#f4d7bd"
                strokeWidth="1"
              />
              {[
                [11, 11],
                [28, 11],
                [19.5, 19.5],
                [11, 28],
                [28, 28],
              ].map(([cx, cy], i) => (
                <circle key={i} cx={cx} cy={cy} r="3" fill="#fff5dc" />
              ))}
            </g>
            <g transform="translate(64 8) rotate(9 16 22)">
              <rect
                x="-6"
                y="3"
                width="31"
                height="43"
                rx="4"
                fill="#ddb45f"
                transform="rotate(-15 10 22)"
              />
              <rect
                width="31"
                height="43"
                rx="4"
                fill="#fcf5e4"
                stroke="#486e87"
                strokeWidth="2"
              />
              <path d="M15.5 10 L24 21.5 L15.5 33 L7 21.5Z" fill="#486e87" />
              <circle cx="6" cy="6" r="1.5" fill="#486e87" />
              <circle cx="25" cy="37" r="1.5" fill="#486e87" />
            </g>
            <g
              fill="#4f8066"
              stroke="#f8f1dd"
              strokeWidth="1.5"
              strokeLinejoin="round"
            >
              <circle cx="120" cy="18" r="9" />
              <path d="M115 26 L125 26 Q124 38 132 45 L132 49 L108 49 L108 45 Q116 38 115 26Z" />
              <rect x="105" y="49" width="30" height="6" rx="3" />
            </g>
          </svg>
          <small>MAKE TIME TO PLAY</small>
        </div>
      </section>
      {!state.availableGames.length && (
        <p className="admin-empty">桌游正在准备中，请稍后再来看看。</p>
      )}
      <section className="game-selection">
        {state.availableGames.map((kind) => (
          <article className="game-feature" key={kind}>
            <Cover kind={kind} />
            <div className="feature-info">
              <div className="feature-kicker">
                {kind === "dota"
                  ? "DEFEND. OUTWIT. ADVANCE."
                  : kind === "sanguosha"
                    ? "CHOOSE YOUR SIDE."
                    : kind === "carcassonne"
                      ? "ONE TILE. A WHOLE WORLD."
                      : kind === "catan"
                        ? "BUILD. TRADE. SETTLE."
                        : kind === "splendor"
                          ? "THE ART OF COLLECTING"
                          : "EVERY ROUTE TELLS A STORY"}
                <span>{kind === "dota" ? "原创桌游" : "基础版"}</span>
              </div>
              <h2>
                {gameName(kind)}
                <small>
                  {kind === "dota"
                    ? "DotA · Lane Tactics"
                    : kind === "sanguosha"
                      ? "Sanguosha"
                      : kind === "carcassonne"
                        ? "Carcassonne"
                        : kind === "catan"
                          ? "CATAN"
                          : kind === "splendor"
                            ? "Splendor"
                            : "Ticket to Ride"}
                </small>
              </h2>
              <p>
                {kind === "dota"
                  ? "暗选行动，协同进军。以熟悉的英雄，赢下一场桌上遗迹之战。"
                  : kind === "sanguosha"
                    ? "执一手好牌，藏一重身份，与好友共赴三国风云。"
                    : kind === "carcassonne"
                      ? "拼接道路、城市与田野，让小小随从写下你的领地故事。"
                      : kind === "catan"
                        ? "掷骰收获资源，交易、铺路，在岛上建立你的城邦。"
                        : kind === "splendor"
                          ? "从宝石商人到财富大师，每一次选择都闪闪发光。"
                          : "从海岸到海岸，让你的铁路连接每一个目的地。"}
              </p>
              <div className="feature-bottom">
                <span>
                  <span>
                    <Users size={15} />
                    {kind === "dota"
                      ? "2 / 4 / 6"
                      : kind === "sanguosha"
                        ? "4–8"
                        : kind === "catan"
                          ? "2–6"
                          : kind === "splendor"
                            ? "2–4"
                            : "2–5"}{" "}
                    人
                  </span>{" "}
                  <span>
                    <Clock size={15} />
                    {kind === "dota"
                      ? "20–40"
                      : kind === "sanguosha"
                        ? "30–60"
                        : kind === "carcassonne"
                          ? "30–45"
                          : kind === "catan"
                            ? "45–90"
                            : kind === "splendor"
                              ? "30"
                              : "45–60"}{" "}
                    分钟
                  </span>
                </span>
                <button className="primary" onClick={() => onCreate(kind)}>
                  <Plus size={17} />
                  开一桌
                </button>
              </div>
            </div>
          </article>
        ))}
      </section>
      <section className="room-list">
        <div className="section-heading">
          <div>
            <h2>
              大厅里的牌桌 <span>{state.rooms.length}</span>
            </h2>
            <p>朋友已经到了？找张桌子坐下吧。</p>
          </div>
          <button
            className="outline"
            disabled={!state.availableGames.length}
            onClick={() => onCreate(state.availableGames[0])}
          >
            <Plus size={17} />
            创建房间
          </button>
        </div>
        <div className="filter-tabs">
          {[
            ["all", "全部桌游"],
            ["splendor", "璀璨宝石"],
            ["rail", "铁路环游"],
            ["catan", "卡坦岛"],
            ["carcassonne", "卡卡颂"],
            ["sanguosha", "三国杀"],
            ["dota", "兵线争锋"],
          ]
            .filter(([k]) => k === "all" || state.availableGames.includes(k))
            .map(([k, label]) => (
              <button
                className={filter === k ? "active" : ""}
                key={k}
                onClick={() => setFilter(k)}
              >
                {label}
              </button>
            ))}
          <span className="live-dot">实时更新</span>
        </div>
        {rooms.length ? (
          <div className="room-table">
            <div className="table-label">
              <span>房间 / 桌游</span>
              <span>桌上的朋友</span>
              <span>状态</span>
              <span />
            </div>
            {rooms.map((r) => (
              <div
                className={`room-row ${invited === r.id ? "invited" : ""}`}
                key={r.id}
              >
                <div className="room-title">
                  <span className={`game-icon ${r.kind}`}>
                    {r.kind === "splendor" ? (
                      <Gem size={24} />
                    ) : (
                      <TrainFront size={24} />
                    )}
                  </span>
                  <div>
                    <h3>
                      {r.name}
                      {r.locked && <Lock size={13} />}
                    </h3>
                    <small>
                      {gameName(r.kind)}
                      {r.kind === "rail" &&
                        ` · ${railMapNames[r.railMap || "usa"]}`}{" "}
                      {r.kind === "sanguosha" &&
                        ` · ${r.sanguoshaOptions?.mode === "hegemony" ? "国战双将" : r.sanguoshaOptions?.deck === "military" ? "标准＋军争" : "身份局"}`}{" "}
                      · {r.id.toUpperCase()}
                      {invited === r.id ? " · 朋友邀请的牌桌" : ""}
                    </small>
                  </div>
                </div>
                <div className="seat-preview">
                  {r.seats.map((p, i) => (
                    <span
                      className="avatar"
                      key={p.id}
                      style={{ background: playerColors[i] }}
                      title={p.name}
                    >
                      <PlayerName user={p}>{p.name[0]}</PlayerName>
                    </span>
                  ))}
                  <small>
                    {r.seats.length} / {r.capacity}
                  </small>
                </div>
                <span className={`status ${r.status}`}>
                  {statusName[r.status]}
                </span>
                {r.status === "playing" ? (
                  <button
                    className="join-button"
                    disabled={busy}
                    onClick={() => onWatch(r)}
                  >
                    <Eye size={16} />
                    观战{r.spectatorCount ? ` · ${r.spectatorCount}` : ""}
                  </button>
                ) : (
                  <button
                    className="join-button"
                    disabled={
                      busy ||
                      r.status !== "waiting" ||
                      r.seats.length >= r.capacity
                    }
                    onClick={() => onJoin(r)}
                  >
                    {r.status === "waiting"
                      ? r.seats.length >= r.capacity
                        ? "已满员"
                        : "加入牌桌"
                      : statusName[r.status]}
                    <ArrowRight size={16} />
                  </button>
                )}
              </div>
            ))}
          </div>
        ) : (
          <div className="empty-room">
            <div className="empty-chairs">
              <span />
              <span />
              <span />
            </div>
            <h3>
              {filter === "all"
                ? "今晚的第一局，由你开始"
                : "还没有这款游戏的牌桌"}
            </h3>
            <p>创建一个房间，邀请朋友一起入座。</p>
            <button
              className="subtle"
              disabled={!state.availableGames.length}
              onClick={() =>
                onCreate(filter === "all" ? state.availableGames[0] : filter)
              }
            >
              摆好第一张牌桌
              <ArrowRight size={16} />
            </button>
          </div>
        )}
      </section>
      <div className="lobby-note">
        <span>
          <Check size={16} />
          每一步自动保存
        </span>
        <span>
          <Wifi size={16} />
          掉线重连，随时回桌
        </span>
        <span>
          <Users size={16} />
          好友相聚，不必排队
        </span>
      </div>
    </main>
  );
}
function Create({
  availableGames,
  kind,
  busy,
  onClose,
  onSubmit,
}: {
  availableGames: string[];
  kind: string;
  busy: boolean;
  onClose: () => void;
  onSubmit: (body: unknown) => void;
}) {
  const maps = useContext(RailMapsContext),
    assets = useContext(AssetsContext);
  const [railMap, setRailMap] = useState("usa");
  const [sgOptions, setSGOptions] = useState<SGOptions>({ deck: "standard" });
  const [catanOptions, setCatanOptions] = useState<CatanOptions>({});
  const [catanBaseLayout, setCatanBaseLayout] = useState("variable");
  const [catanTwoScenario, setCatanTwoScenario] = useState("");
  const [catanScenario, setCatanScenario] = useState("");
  const [catanFishing, setCatanFishing] = useState(false);
  const [catanFishingLakes, setCatanFishingLakes] = useState(true);
  const [catanHarbors, setCatanHarbors] = useState(false);
  const [catanFriendly, setCatanFriendly] = useState(false);
  const [catanSeaKnights, setCatanSeaKnights] = useState(false);
  const [catanEvents, setCatanEvents] = useState(false);
  const [gemOptions, setGemOptions] = useState<SplendorOptions>({});
  const map = maps.find((m) => m.id === railMap);
  const [k, setK] = useState(kind);
  useEffect(() => {
    if (!availableGames.includes(k)) setK(availableGames[0] || "");
  }, [availableGames, k]);
  const [capacity, setCapacity] = useState(4);
  const variantScenario = capacity === 2 ? catanTwoScenario : catanScenario;
  const variantsAvailable =
    capacity === 2
      ? supportsTwoCatanVariants(catanTwoScenario)
      : capacity >= 3 && (capacity <= 4 || catanOptions.fiveSix);
  const variantKnights =
    variantScenario === "cities-knights" || catanSeaKnights;
  const eventsAvailable = catanEventsSupported({
    kind: k,
    capacity,
    catanTwoScenario: capacity === 2 ? catanTwoScenario : undefined,
    catanScenario: capacity === 2 ? catanTwoScenario : catanScenario,
    catanFishing,
    catanCitiesKnights:
      catanSeaKnights || catanScenario === "cities-knights"
        ? { layout: "variable", rules: "" }
        : undefined,
    catanHarbors: { enabled: catanHarbors, rules: "" },
    catanFriendlyRobber: { enabled: catanFriendly, rules: "" },
  });
  useEffect(() => {
    if (!eventsAvailable) setCatanEvents(false);
  }, [eventsAvailable]);
  return (
    <Modal title="摆好一张新牌桌" onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          onSubmit({
            name: f.get("name"),
            password: f.get("password"),
            kind: k,
            railMap: k === "rail" ? railMap : undefined,
            sanguoshaOptions: k === "sanguosha" ? sgOptions : undefined,
            splendorOptions: k === "splendor" ? gemOptions : undefined,
            catanOptions:
              k === "catan"
                ? capacity === 2 &&
                  !isPublicCatanExplorer(catanTwoScenario) &&
                  !supportsTwoCatanHelpers(catanTwoScenario)
                  ? {}
                  : catanOptions
                : undefined,
            catanBaseConfiguration:
              k === "catan" &&
              capacity > 4 &&
              catanOptions.fiveSix &&
              !catanScenario
                ? { layout: catanBaseLayout }
                : undefined,
            catanScenario:
              k === "catan"
                ? capacity === 2
                  ? isPublicCatanFlexible(catanTwoScenario)
                    ? catanTwoScenario
                    : undefined
                  : catanScenario
                : undefined,
            catanFriendlyRobber:
              k === "catan" &&
              variantsAvailable &&
              catanFriendly &&
              supportsPublicCatanFriendly(variantScenario)
                ? { enabled: true }
                : undefined,
            catanHarbors:
              k === "catan" &&
              variantsAvailable &&
              catanHarbors &&
              supportsPublicCatanHarbors(variantScenario)
                ? { enabled: true }
                : undefined,
            catanFishing:
              k === "catan" &&
              ((capacity === 2 &&
                (supportsTwoCatanSeafarers(catanTwoScenario) ||
                  [
                    "cities-knights",
                    "rivers",
                    "caravans",
                    "barbarian-attack",
                  ].includes(catanTwoScenario))) ||
                supportsPublicCatanFishingSea(catanScenario) ||
                isPublicCatanExplorer(catanScenario) ||
                [
                  "rivers",
                  "caravans",
                  "barbarian-attack",
                  "transport",
                ].includes(catanScenario)) &&
              catanFishing,
            catanFishingLakes:
              k === "catan" &&
              isPublicCatanExplorer(catanScenario) &&
              catanFishing &&
              catanFishingLakes,
            catanEvents:
              eventsAvailable && catanEvents
                ? CATAN_EVENT_CATALOGUE
                : undefined,
            catanCitiesKnights:
              k === "catan" &&
              (capacity >= 3 ||
                isPublicCatanFlexible(variantScenario) ||
                supportsTwoCatanSeafarers(variantScenario) ||
                ["caravans", "rivers", "barbarian-attack"].includes(
                  variantScenario,
                )) &&
              catanSeaKnights &&
              supportsPublicCatanKnightsCombination(variantScenario)
                ? { layout: "variable" }
                : undefined,
            catanTwoScenario:
              k === "catan" &&
              capacity === 2 &&
              !isPublicCatanFlexible(catanTwoScenario)
                ? catanTwoScenario
                : undefined,
            capacity,
          });
        }}
      >
        <div className="choose-games">
          {availableGames.map((x) => (
            <button
              type="button"
              className={k === x ? "selected" : ""}
              key={x}
              onClick={() => {
                setK(x);
                if (x === "catan" && capacity === 2 && !catanOptions.fiveSix)
                  setCatanOptions({});
                setCapacity(
                  x === "dota"
                    ? Math.min(6, Math.max(2, capacity - (capacity % 2)))
                    : Math.max(
                        x === "sanguosha"
                          ? 4
                          : x === "catan"
                            ? catanOptions.fiveSix
                              ? 5
                              : ((catanSeaKnights || catanFishing) &&
                                    !isPublicCatanExplorer(catanScenario)) ||
                                  [
                                    "cities-knights",
                                    "fishing",
                                    "barbarian-attack",
                                  ].includes(catanScenario)
                                ? 3
                                : 2
                            : 2,
                        Math.min(
                          capacity,
                          x === "sanguosha"
                            ? 8
                            : x === "rail"
                              ? map?.maxPlayers || 5
                              : x === "carcassonne"
                                ? 5
                                : x === "catan" &&
                                    (catanOptions.fiveSix ||
                                      [
                                        "land-ho",
                                        "spices-for-catan",
                                        "pirate-lairs",
                                        "fish-for-catan",
                                        "explorers-and-pirates",
                                        "barbarian-attack",
                                        "transport",
                                        "rivers-caravans",
                                        "rivers-attack",
                                        "rivers-transport",
                                        "caravans-attack",
                                        "caravans-transport",
                                        "attack-transport",
                                        "rivers-shores",
                                        "rivers-fog",
                                        "rivers-desert",
                                        "rivers-desert-belt",
                                        "rivers-tribe",
                                        "caravans-shores",
                                        "caravans-new-world",
                                        "caravans-islands",
                                        "caravans-tribe",
                                        "caravans-desert",
                                        "rivers-new-world",
                                      ].includes(catanScenario))
                                  ? 6
                                  : 4,
                        ),
                      ),
                );
              }}
            >
              <Cover kind={x} mini />
              <span>
                {gameName(x)}
                {k === x && <Check size={16} />}
              </span>
            </button>
          ))}
        </div>
        {k === "sanguosha" && (
          <SanguoshaOptions value={sgOptions} onChange={setSGOptions} />
        )}
        {k === "catan" && capacity === 2 && (
          <CatanTwoScenarioPicker
            value={catanTwoScenario}
            knightsEnabled={catanSeaKnights}
            fishingEnabled={
              catanFishing &&
              (supportsTwoCatanSeafarers(catanTwoScenario) ||
                [
                  "cities-knights",
                  "rivers",
                  "caravans",
                  "barbarian-attack",
                ].includes(catanTwoScenario))
            }
            target={
              catanVictoryTarget(
                catanTwoScenario,
                catanTwoScenario === "cities-knights" || catanSeaKnights,
              ) + (variantsAvailable && catanHarbors ? 1 : 0)
            }
            onChange={(scenario) => {
              setCatanTwoScenario(scenario);
              if (!supportsTwoCatanVariants(scenario)) {
                setCatanHarbors(false);
                setCatanFriendly(false);
              }
              setCatanScenario(isPublicCatanFlexible(scenario) ? scenario : "");
              if (!isPublicCatanExplorer(scenario)) {
                if (!supportsTwoCatanHelpers(scenario)) setCatanOptions({});
                if (
                  !supportsTwoCatanSeafarers(scenario) &&
                  scenario !== "cities-knights" &&
                  ![
                    "rivers",
                    "caravans",
                    "barbarian-attack",
                    "transport",
                  ].includes(scenario)
                )
                  setCatanFishing(false);
                setCatanFishingLakes(false);
              }
              if (
                !supportsTwoCatanSeafarers(scenario) &&
                !isPublicCatanExplorer(scenario)
              )
                setCatanSeaKnights(false);
            }}
          />
        )}
        {k === "catan" &&
          capacity >= 3 &&
          (capacity <= 4 ||
            (catanOptions.fiveSix &&
              (!catanScenario ||
                catanScenario === "cities-knights" ||
                isPublicCatanSea(catanScenario) ||
                ["rivers", "caravans", "fishing"].includes(catanScenario))) ||
            (catanFishing &&
              supportsPublicCatanFishingSeaExtended(catanScenario)) ||
            [
              "land-ho",
              "spices-for-catan",
              "pirate-lairs",
              "fish-for-catan",
              "explorers-and-pirates",
              "barbarian-attack",
              "transport",
              "rivers-caravans",
              "rivers-attack",
              "rivers-transport",
              "caravans-attack",
              "caravans-transport",
              "attack-transport",
              "rivers-shores",
              "rivers-fog",
              "rivers-desert",
              "rivers-desert-belt",
              "rivers-tribe",
              "caravans-shores",
              "caravans-new-world",
              "caravans-islands",
              "caravans-tribe",
              "caravans-desert",
              "rivers-new-world",
            ].includes(catanScenario)) && (
            <CatanScenarioPicker
              players={capacity}
              fiveSix={!!catanOptions.fiveSix}
              value={catanScenario}
              helpers={!!catanOptions.helpers}
              knights={catanSeaKnights}
              fishing={catanFishing}
              friendly={catanFriendly}
              harbors={catanHarbors}
              onChange={(scenario) => {
                setCatanScenario(scenario);
                if (scenario === "caravans-islands")
                  setCapacity(Math.min(6, Math.max(2, capacity)));
                if (scenario === "caravans-shores")
                  setCapacity(Math.max(2, capacity));
                if (!supportsPublicCatanFriendly(scenario))
                  setCatanFriendly(false);
                if (!supportsPublicCatanHarbors(scenario))
                  setCatanHarbors(false);
                if (
                  !supportsPublicCatanFishingSea(scenario) &&
                  !isPublicCatanExplorer(scenario) &&
                  ![
                    "rivers",
                    "caravans",
                    "barbarian-attack",
                    "transport",
                  ].includes(scenario)
                )
                  setCatanFishing(false);
                if (!supportsPublicCatanKnightsCombination(scenario))
                  setCatanSeaKnights(false);
                if (isPublicCatanFlexible(scenario))
                  setCatanTwoScenario(scenario);
                if (
                  [
                    "rivers-caravans",
                    "rivers-attack",
                    "rivers-transport",
                    "caravans-attack",
                    "caravans-transport",
                    "attack-transport",
                    "rivers-shores",
                    "rivers-fog",
                    "rivers-desert",
                    "rivers-desert-belt",
                    "rivers-tribe",
                    "caravans-shores",
                    "caravans-new-world",
                    "caravans-islands",
                    "caravans-tribe",
                    "caravans-desert",
                    "rivers-new-world",
                  ].includes(scenario || "")
                )
                  setCatanOptions({});
                if (
                  ![
                    "land-ho",
                    "spices-for-catan",
                    "pirate-lairs",
                    "fish-for-catan",
                    "explorers-and-pirates",
                    "barbarian-attack",
                    "transport",
                    "rivers-caravans",
                    "rivers-attack",
                    "rivers-transport",
                    "caravans-attack",
                    "caravans-transport",
                    "attack-transport",
                    "rivers-shores",
                    "rivers-fog",
                    "rivers-desert",
                    "rivers-desert-belt",
                    "rivers-tribe",
                    "caravans-shores",
                    "caravans-new-world",
                    "caravans-islands",
                    "caravans-tribe",
                    "caravans-desert",
                    "rivers-new-world",
                  ].includes(scenario) &&
                  !(
                    catanOptions.fiveSix &&
                    (!scenario ||
                      scenario === "cities-knights" ||
                      isPublicCatanSea(scenario) ||
                      ["rivers", "caravans", "fishing"].includes(scenario))
                  ) &&
                  !(
                    catanFishing &&
                    supportsPublicCatanFishingSeaExtended(scenario)
                  )
                )
                  setCapacity(Math.min(4, capacity));
                if (["rivers", "caravans"].includes(scenario))
                  setCatanOptions(
                    catanOptions.fiveSix ? { fiveSix: true } : {},
                  );
                else if (isPublicCatanExplorer(scenario))
                  setCatanOptions({
                    helpers: catanOptions.helpers,
                    allHelpers: catanOptions.allHelpers,
                  });
                else if (
                  scenario &&
                  scenario !== "fishing" &&
                  !isPublicCatanSea(scenario)
                )
                  setCatanOptions({});
              }}
            />
          )}
        {k === "catan" &&
          ((capacity === 2 &&
            (supportsTwoCatanSeafarers(catanTwoScenario) ||
              [
                "cities-knights",
                "rivers",
                "caravans",
                "barbarian-attack",
              ].includes(catanTwoScenario))) ||
            isPublicCatanExplorer(catanScenario) ||
            ["rivers", "caravans", "barbarian-attack", "transport"].includes(
              variantScenario,
            ) ||
            (capacity >= 3 &&
              (capacity <= 4 ||
                supportsPublicCatanFishingSeaExtended(catanScenario)) &&
              supportsPublicCatanFishingSea(catanScenario))) && (
            <CatanFishingSeaPicker
              transport={variantScenario === "transport"}
              attack={["barbarian-attack", "rivers-attack"].includes(
                variantScenario,
              )}
              caravans={variantScenario === "caravans"}
              rivers={variantScenario === "rivers"}
              knights={capacity === 2 && catanTwoScenario === "cities-knights"}
              value={catanFishing}
              two={
                capacity === 2 && supportsTwoCatanSeafarers(catanTwoScenario)
              }
              shores={variantScenario === "shores"}
              explorer={isPublicCatanExplorer(catanScenario)}
              extended={capacity > 4}
              onChange={setCatanFishing}
              lakes={catanFishingLakes}
              onLakes={setCatanFishingLakes}
            />
          )}
        {k === "catan" &&
          (capacity >= 3 ||
            isPublicCatanFlexible(variantScenario) ||
            supportsTwoCatanSeafarers(variantScenario) ||
            ["caravans", "rivers", "barbarian-attack"].includes(
              variantScenario,
            )) &&
          supportsPublicCatanKnightsCombination(variantScenario) && (
            <CatanCombinationKnightsPicker
              attack={["barbarian-attack", "rivers-attack"].includes(
                variantScenario,
              )}
              riversAttack={variantScenario === "rivers-attack"}
              riversTransport={variantScenario === "rivers-transport"}
              caravansAttack={variantScenario === "caravans-attack"}
              attackTransport={variantScenario === "attack-transport"}
              caravansTransport={variantScenario === "caravans-transport"}
              transport={variantScenario === "transport"}
              rivers={variantScenario === "rivers"}
              riversCaravans={variantScenario === "rivers-caravans"}
              caravans={["caravans", "rivers-caravans"].includes(
                variantScenario,
              )}
              tribe={variantScenario === "tribe"}
              pirateIslands={catanScenario === "pirate_islands"}
              explorer={supportsPublicExplorerKnights(catanScenario)}
              two={capacity === 2}
              intro={catanScenario === "land-ho"}
              fishing={catanFishing || catanScenario === "fishing"}
              harbors={catanHarbors}
              value={catanSeaKnights}
              helpers={!!catanOptions.helpers}
              onChange={setCatanSeaKnights}
            />
          )}
        {k === "catan" &&
          variantsAvailable &&
          supportsPublicCatanHarbors(variantScenario) && (
            <CatanHarborsChoice
              two={capacity === 2}
              value={catanHarbors}
              onChange={setCatanHarbors}
              helpers={!!catanOptions.helpers}
              target={
                catanVictoryTarget(variantScenario, variantKnights) +
                (catanHarbors ? 1 : 0)
              }
            />
          )}
        {k === "catan" &&
          variantsAvailable &&
          supportsPublicCatanFriendly(variantScenario) && (
            <CatanFriendlyRobberChoice
              two={capacity === 2}
              value={catanFriendly}
              onChange={setCatanFriendly}
              knights={variantKnights}
              scenario={capacity === 2 ? "" : catanScenario}
              fishing={catanFishing || variantScenario === "fishing"}
            />
          )}
        {k === "catan" &&
          capacity === 2 &&
          supportsTwoCatanHelpers(catanTwoScenario) && (
            <>
              <CatanOptionPicker
                value={catanOptions}
                onChange={setCatanOptions}
                fiveSixAvailable={false}
                citiesKnights={variantKnights}
                fishing={catanFishing || catanTwoScenario === "fishing"}
                harbors={catanHarbors}
                friendlyRobber={catanFriendly}
              />
              {catanOptions.helpers && (
                <p className="muted small">{catanTwoHelpersNote}</p>
              )}
            </>
          )}
        {k === "catan" &&
          isPublicCatanExplorer(
            capacity === 2 ? catanTwoScenario : catanScenario,
          ) && (
            <CatanOptionPicker
              explorer
              fiveSixAvailable={false}
              value={catanOptions}
              onChange={setCatanOptions}
            />
          )}
        {k === "catan" &&
          capacity !== 2 &&
          (!catanFishing ||
            supportsPublicCatanFishingSeaExtended(catanScenario)) &&
          (!catanScenario ||
            ["cities-knights", "rivers", "caravans", "fishing"].includes(
              catanScenario,
            ) ||
            isPublicCatanSea(catanScenario)) && (
            <CatanOptionPicker
              value={catanOptions}
              helpersAvailable={!["rivers", "caravans"].includes(catanScenario)}
              fishing={catanFishing || catanScenario === "fishing"}
              seafarers={isPublicCatanSea(catanScenario)}
              citiesKnights={
                catanSeaKnights || catanScenario === "cities-knights"
              }
              harbors={catanHarbors}
              friendlyRobber={catanFriendly}
              fiveSixAvailable={
                !catanFishing ||
                supportsPublicCatanFishingSeaExtended(catanScenario)
              }
              onChange={(o) => {
                setCatanOptions(o);
                setCapacity(
                  o.fiveSix ? Math.max(5, capacity) : Math.min(4, capacity),
                );
              }}
            />
          )}
        {k === "catan" &&
          capacity > 4 &&
          catanOptions.fiveSix &&
          !catanScenario && (
            <fieldset className="catan-helper-options catan-scenario-options">
              <legend>基础地图布局</legend>
              <label>
                地图布局
                <select
                  value={catanBaseLayout}
                  onChange={(e) => setCatanBaseLayout(e.target.value)}
                >
                  <option value="variable">随机布局</option>
                  <option value="fixed">固定新手布局</option>
                </select>
              </label>
              <p>
                {catanBaseLayout === "fixed"
                  ? "使用规则书固定地图，随机分配颜色，预放两座村庄和两条道路并领取起始资源。五人局保留剩余颜色的两座中立村庄，直接开始掷骰。"
                  : "随机安排地形，开局后依次选择村庄和道路的位置。"}
              </p>
            </fieldset>
          )}
        {eventsAvailable && (
          <CatanEventPicker
            value={catanEvents}
            explorer={[
              "land-ho",
              "pirate-lairs",
              "fish-for-catan",
              "spices-for-catan",
              "explorers-and-pirates",
            ].includes(catanScenario)}
            transport={catanScenario === "transport"}
            pirateIslands={catanScenario === "pirate_islands"}
            fishing={catanFishing || catanScenario === "fishing"}
            friendly={catanFriendly}
            cloth={
              (capacity === 2 ? catanTwoScenario : catanScenario) === "cloth"
            }
            citiesKnights={variantKnights}
            disabled={busy}
            onChange={setCatanEvents}
          />
        )}
        {k === "splendor" && (
          <SplendorOptionPicker value={gemOptions} onChange={setGemOptions} />
        )}
        {k === "rail" && (
          <RailMapPicker
            maps={maps}
            assets={assets}
            value={railMap}
            onChange={(id) => {
              setRailMap(id);
              setCapacity(
                Math.min(capacity, maps.find((m) => m.id === id)!.maxPlayers),
              );
            }}
          />
        )}
        <label>
          房间名称
          <input
            name="name"
            required
            maxLength={40}
            placeholder="例如：周五晚上的快乐"
            defaultValue="朋友们的牌桌"
          />
        </label>
        <div className="form-grid">
          <label>
            座位上限
            <select
              value={capacity}
              onChange={(e) => {
                const next = Number(e.target.value);
                setCapacity(next);
                if (
                  k === "catan" &&
                  capacity === 2 &&
                  next !== 2 &&
                  !isPublicCatanFlexible(catanScenario)
                ) {
                  setCatanFishing(false);
                  setCatanFishingLakes(false);
                }
                if (
                  k === "catan" &&
                  next === 2 &&
                  !isPublicCatanExplorer(catanScenario)
                )
                  setCatanOptions({});
              }}
            >
              {(k === "dota"
                ? [2, 4, 6]
                : k === "catan" &&
                    ["caravans-new-world", "caravans-islands"].includes(
                      catanScenario,
                    )
                  ? [2, 3, 4, 5, 6]
                  : Array.from(
                      {
                        length:
                          k === "catan" && catanScenario === "caravans-shores"
                            ? 5
                            : k === "catan" &&
                                ["caravans-desert", "caravans-tribe"].includes(
                                  catanScenario,
                                )
                              ? 5
                              : k === "sanguosha"
                                ? 5
                                : k === "catan"
                                  ? [
                                      "land-ho",
                                      "spices-for-catan",
                                      "pirate-lairs",
                                      "fish-for-catan",
                                      "explorers-and-pirates",
                                      "transport",
                                      "rivers-caravans",
                                      "rivers-attack",
                                      "rivers-transport",
                                      "caravans-attack",
                                      "caravans-transport",
                                      "attack-transport",
                                      "rivers-shores",
                                      "rivers-fog",
                                      "rivers-desert",
                                      "rivers-desert-belt",
                                      "rivers-tribe",
                                      "caravans-shores",
                                      "caravans-new-world",
                                      "caravans-islands",
                                      "caravans-tribe",
                                      "caravans-desert",
                                      "rivers-new-world",
                                    ].includes(catanScenario)
                                    ? 5
                                    : catanScenario === "barbarian-attack"
                                      ? 4
                                      : catanOptions.fiveSix ||
                                          (capacity !== 2 && catanSeaKnights) ||
                                          (capacity !== 2 &&
                                            catanFishing &&
                                            !isPublicCatanExplorer(
                                              catanScenario,
                                            )) ||
                                          [
                                            "cities-knights",
                                            "fishing",
                                            "barbarian-attack",
                                          ].includes(catanScenario)
                                        ? 2
                                        : 3
                                  : k === "rail"
                                    ? (map?.maxPlayers || 5) - 1
                                    : k === "carcassonne"
                                      ? 4
                                      : 3,
                      },
                      (_, i) =>
                        k === "catan" && catanScenario === "caravans-shores"
                          ? [2, 3, 4, 5, 6][i]
                          : i +
                            (k === "catan" &&
                            catanScenario === "caravans-shores"
                              ? 4
                              : k === "catan" &&
                                  [
                                    "caravans-desert",
                                    "caravans-tribe",
                                  ].includes(catanScenario)
                                ? 2
                                : k === "sanguosha"
                                  ? 4
                                  : k === "catan"
                                    ? isPublicCatanFlexible(catanScenario)
                                      ? 2
                                      : catanOptions.fiveSix
                                        ? 5
                                        : (capacity !== 2 &&
                                              catanSeaKnights &&
                                              !isPublicCatanExplorer(
                                                catanScenario,
                                              )) ||
                                            (capacity !== 2 &&
                                              catanFishing &&
                                              !isPublicCatanExplorer(
                                                catanScenario,
                                              )) ||
                                            [
                                              "cities-knights",
                                              "fishing",
                                              "barbarian-attack",
                                            ].includes(catanScenario)
                                          ? 3
                                          : 2
                                    : 2),
                    )
              ).map((n) => (
                <option key={n} value={n}>
                  {n} 人
                </option>
              ))}
            </select>
          </label>
          <label>
            房间密码 <small>可选</small>
            <input
              name="password"
              type="password"
              maxLength={72}
              placeholder="留空即可直接加入"
              autoComplete="off"
            />
          </label>
        </div>
        <p className="muted small">
          至少{" "}
          {k === "sanguosha"
            ? 4
            : k === "catan"
              ? capacity === 2 || isPublicCatanFlexible(catanScenario)
                ? 2
                : catanOptions.fiveSix
                  ? 5
                  : 3
              : 2}
          个座位即可开始，不必坐满。一个人也可以在房间里添加电脑玩家体验。
          {k === "dota" && " 兵线争锋需 2、4 或 6 人，开始后再分队。"}
        </p>
        <button className="primary wide" disabled={busy}>
          创建房间
          <ArrowRight size={18} />
        </button>
      </form>
    </Modal>
  );
}
function Waiting({
  room,
  busy,
  host,
  command,
}: {
  room: Room;
  busy: boolean;
  host: boolean;
  command: (t: string, extra?: Record<string, unknown>) => void;
}) {
  const [mapDirty, setMapDirty] = useState(false);
  const maps = useContext(RailMapsContext),
    assets = useContext(AssetsContext);
  const map = maps.find((m) => m.id === (room.railMap || "usa"));
  const seaInfo = room.catanSeafarersChoices?.find(
    (s) => s.id === room.catanSeafarers?.scenario,
  );
  const scenarioSelectable =
    room.kind === "catan" &&
    !room.catanTwoRules &&
    room.capacity >= 3 &&
    (room.capacity <= 4 ||
      (room.catanOptions?.fiveSix &&
        (!room.catanScenario ||
          room.catanScenario === "cities-knights" ||
          isPublicCatanSea(room.catanScenario) ||
          ["rivers", "caravans", "fishing"].includes(
            room.catanScenario || "",
          ))) ||
      (room.catanFishing &&
        supportsPublicCatanFishingSeaExtended(room.catanScenario)) ||
      [
        "land-ho",
        "spices-for-catan",
        "pirate-lairs",
        "fish-for-catan",
        "explorers-and-pirates",
        "barbarian-attack",
        "transport",
        "rivers-caravans",
        "rivers-attack",
        "rivers-transport",
        "caravans-attack",
        "caravans-transport",
        "attack-transport",
        "rivers-shores",
        "rivers-fog",
        "rivers-desert",
        "rivers-desert-belt",
        "rivers-tribe",
        "caravans-shores",
        "caravans-new-world",
        "caravans-islands",
        "caravans-tribe",
        "caravans-desert",
        "rivers-new-world",
      ].includes(room.catanScenario || "")) &&
    (!room.catanOptions?.fiveSix ||
      room.catanFishing ||
      !room.catanScenario ||
      room.catanScenario === "cities-knights" ||
      isPublicCatanSea(room.catanScenario) ||
      ["rivers", "caravans", "fishing"].includes(room.catanScenario || "")) &&
    (!room.catanBaseConfiguration || !room.catanScenario) &&
    (!room.catanCitiesKnights ||
      room.catanScenario === "cities-knights" ||
      room.catanScenario === "fishing" ||
      supportsPublicCatanKnightsCombination(room.catanScenario) ||
      isPublicCatanSea(room.catanScenario)) &&
    (!room.catanSeafarers || isPublicCatanSea(room.catanScenario)) &&
    (!room.catanNewWorldMap ||
      ["new_world", "caravans-new-world"].includes(room.catanScenario || ""));
  const twoLabel =
    room.kind === "catan" && room.catanTwoRules
      ? `双人卡坦${room.catanTwoScenario ? "＋" + catanScenarioName(room.catanTwoScenario) : ""}${room.catanCitiesKnights ? "＋城市与骑士" : ""}${room.catanFishing ? "＋渔夫" : ""}${room.catanOptions?.helpers ? "＋Helpers" : ""}${room.catanHarbors?.enabled ? "＋港口霸主" : ""}${room.catanFriendlyRobber?.enabled ? "＋友善强盗" : ""} · 2 人 · ${catanRuleContext(room).target} 分获胜`
      : "";
  const ready =
    room.seats.every((p) => p.ready) &&
    (!room.catanFriendlyRobber?.enabled ||
      room.seats.length >=
        (room.catanFriendlyRobberAvailability?.minPlayers || 3)) &&
    (room.kind !== "dota" || room.seats.length % 2 === 0) &&
    room.seats.length >=
      (room.kind === "sanguosha"
        ? 4
        : room.kind === "catan"
          ? room.catanTwoRules || isPublicCatanFlexible(room.catanScenario)
            ? 2
            : room.catanOptions?.fiveSix
              ? 5
              : 3
          : 2);
  return (
    <div className="waiting-layout">
      {room.kind === "catan" && room.catanScenario === "rivers-new-world" && (
        <CatanRiversWorldPreview
          onDirty={setMapDirty}
          room={room}
          host={host}
          busy={busy}
          command={command}
        />
      )}
      {room.kind === "catan" && room.catanNewWorldMap && (
        <CatanWorldEditor
          room={room}
          assets={assets}
          host={host}
          busy={busy}
          command={command}
          onDirty={setMapDirty}
        />
      )}

      <div className="waiting-cover">
        <Cover kind={room.kind} />
        <h2>{gameName(room.kind)}</h2>
        <p>
          {room.kind === "dota"
            ? "先入座，再结盟；让每一次出牌推动兵线。"
            : room.kind === "sanguosha"
              ? "与好友过招，也与身份博弈。"
              : room.kind === "carcassonne"
                ? "一块接一块，拼出属于你们的城邦。"
                : room.kind === "catan"
                  ? "一座岛屿，无数种通往胜利的道路。"
                  : room.kind === "splendor"
                    ? "每颗宝石，都是通往胜利的可能。"
                    : "下一站，会是你的目的地吗？"}
        </p>
        <span className="tag">
          {room.kind === "dota"
            ? "1v1 / 2v2 / 3v3 · 开始后分队 · 全部拆遗迹"
            : room.kind === "sanguosha"
              ? room.sanguoshaOptions?.mode === "hegemony"
                ? "基础国战 · 4–8 人 · 60 将 / 108 张牌"
                : `身份局 · 4–8 人 · ${room.sanguoshaOptions?.deck === "military" ? "标准＋军争 / 160 张牌" : "标准 / 108 张牌"}`
              : room.kind === "carcassonne"
                ? "基础版 · 2–5 人 · 包含农民"
                : room.kind === "catan"
                  ? room.catanTwoRules
                    ? twoLabel
                    : `${room.catanScenario ? (seaInfo ? `航海家 · ${seaInfo.name}` : catanScenarioName(room.catanScenario)) : room.catanCitiesKnights ? `城市与骑士 · ${seaInfo ? `航海家 · ${seaInfo.name}` : "随机地图"}` : seaInfo ? `航海家 · ${seaInfo.name}` : room.catanNewWorldMap ? "航海家 · 新世界" : `基础版${room.catanBaseConfiguration ? ` · ${catanBaseLayoutName(room.catanBaseConfiguration.layout)}` : ""}`}${room.catanCitiesKnights ? "＋城市与骑士" : ""}${room.catanFishing ? "＋渔夫" : ""}${room.catanOptions?.helpers ? "＋Helpers" : ""}${room.catanHarbors?.enabled ? "＋港口霸主" : ""}${room.catanFriendlyRobber?.enabled ? "＋友善强盗" : ""} · ${
                        room.catanOptions?.fiveSix
                          ? "5–6 人 · 配对回合"
                          : room.catanFriendlyRobber?.enabled &&
                              room.catanFriendlyRobberAvailability
                                ?.minPlayers === 4
                            ? "4 人"
                            : ["barbarian-attack"].includes(
                                  room.catanScenario || "",
                                )
                              ? "3–6 人 · 五六人配对回合"
                              : [
                                    "land-ho",
                                    "spices-for-catan",
                                    "pirate-lairs",
                                    "fish-for-catan",
                                    "explorers-and-pirates",
                                    "transport",
                                    "rivers-caravans",
                                    "rivers-attack",
                                    "rivers-transport",
                                    "caravans-attack",
                                    "caravans-transport",
                                    "attack-transport",
                                    "rivers-shores",
                                    "rivers-fog",
                                    "rivers-desert",
                                    "rivers-desert-belt",
                                    "rivers-tribe",
                                    "caravans-shores",
                                    "caravans-new-world",
                                    "caravans-islands",
                                    "caravans-tribe",
                                    "caravans-desert",
                                    "rivers-new-world",
                                  ].includes(room.catanScenario || "")
                                ? "2–6 人 · 五六人配对回合"
                                : isPublicCatanFlexible(room.catanScenario)
                                  ? "2–4 人"
                                  : "3–4 人"
                      } · ${catanScenarioVictory(catanRuleContext(room).scenario, catanRuleContext(room).target)}`
                  : room.kind === "splendor"
                    ? `${splendorRulesLabel(room.splendorOptions)} · 2–4 人`
                    : `${map?.name || "美国"}地图 · 2–${map?.maxPlayers || 5} 人`}
        </span>
      </div>
      <div className="waiting-seats">
        <span className="eyebrow">TAKE YOUR SEAT</span>
        <h2>朋友或电脑，到齐就开局。</h2>
        {(twoLabel ||
          (room.kind === "catan" &&
            room.capacity === 2 &&
            isPublicCatanFlexible(room.catanScenario))) && (
          <CatanTwoScenarioPicker
            value={
              isPublicCatanFlexible(room.catanScenario)
                ? room.catanScenario!
                : room.catanTwoScenario || ""
            }
            disabled={!host || busy}
            target={catanRuleContext(room).target}
            fishingEnabled={!!room.catanFishing}
            knightsEnabled={!!room.catanCitiesKnights}
            helpersEnabled={!!room.catanOptions?.helpers}
            variantsEnabled={
              !!room.catanFriendlyRobber?.enabled ||
              !!room.catanHarbors?.enabled
            }
            onChange={(catanTwoScenario) =>
              isPublicCatanFlexible(catanTwoScenario)
                ? command("catan_scenario", { catanScenario: catanTwoScenario })
                : command("catan_two_scenario", { catanTwoScenario })
            }
          />
        )}
        {scenarioSelectable && (
          <CatanScenarioPicker
            value={room.catanScenario || ""}
            players={room.capacity}
            fiveSix={!!room.catanOptions?.fiveSix}
            fishing={!!room.catanFishing}
            friendly={!!room.catanFriendlyRobber?.enabled}
            harbors={!!room.catanHarbors?.enabled}
            helpers={!!room.catanOptions?.helpers}
            knights={
              !!room.catanCitiesKnights &&
              supportsPublicCatanKnightsCombination(room.catanScenario)
            }
            disabled={!host || busy || mapDirty}
            onChange={(catanScenario) =>
              command("catan_scenario", { catanScenario })
            }
          />
        )}
        {room.kind === "catan" &&
          ((!!room.catanTwoRules &&
            room.catanTwoScenario === "cities-knights") ||
            supportsTwoCatanSeafarers(room.catanTwoScenario) ||
            isPublicCatanExplorer(room.catanScenario) ||
            ["rivers", "caravans", "barbarian-attack", "transport"].includes(
              room.catanTwoScenario || room.catanScenario || "",
            ) ||
            ((room.capacity <= 4 ||
              supportsPublicCatanFishingSeaExtended(room.catanScenario)) &&
              supportsPublicCatanFishingSea(room.catanScenario))) && (
            <CatanFishingSeaPicker
              transport={room.catanScenario === "transport"}
              attack={["barbarian-attack", "rivers-attack"].includes(
                room.catanTwoScenario || room.catanScenario || "",
              )}
              caravans={
                (room.catanTwoScenario || room.catanScenario) === "caravans"
              }
              rivers={
                (room.catanTwoScenario || room.catanScenario) === "rivers"
              }
              knights={
                !!room.catanTwoRules &&
                room.catanTwoScenario === "cities-knights"
              }
              value={!!room.catanFishing}
              two={!!room.catanTwoRules}
              shores={
                (room.catanTwoScenario || room.catanScenario) === "shores"
              }
              explorer={isPublicCatanExplorer(room.catanScenario)}
              lakes={!!room.catanFishingLakes}
              onLakes={(enabled) => command("catan_fishing_lakes", { enabled })}
              fixedRequired={
                (["desert", "tribe"].includes(
                  room.catanTwoScenario || room.catanScenario || "",
                ) ||
                  (room.capacity > 4 &&
                    ["islands", "cloth"].includes(room.catanScenario || ""))) &&
                room.catanSeafarers?.layout !== "fixed"
              }
              extended={room.capacity > 4}
              disabled={!host || busy || mapDirty}
              onChange={(enabled) => command("catan_fishing", { enabled })}
            />
          )}
        {room.kind === "catan" &&
          (room.capacity >= 3 ||
            isPublicCatanFlexible(room.catanScenario) ||
            supportsTwoCatanSeafarers(room.catanTwoScenario) ||
            ["caravans", "rivers", "barbarian-attack"].includes(
              room.catanTwoScenario || "",
            )) &&
          supportsPublicCatanKnightsCombination(
            room.catanTwoScenario || room.catanScenario,
          ) && (
            <CatanCombinationKnightsPicker
              attack={["barbarian-attack", "rivers-attack"].includes(
                room.catanTwoScenario || room.catanScenario || "",
              )}
              transport={room.catanScenario === "transport"}
              riversCaravans={room.catanScenario === "rivers-caravans"}
              riversAttack={room.catanScenario === "rivers-attack"}
              riversTransport={room.catanScenario === "rivers-transport"}
              caravansAttack={room.catanScenario === "caravans-attack"}
              attackTransport={room.catanScenario === "attack-transport"}
              caravansTransport={room.catanScenario === "caravans-transport"}
              rivers={
                (room.catanTwoScenario || room.catanScenario) === "rivers"
              }
              caravans={["caravans", "rivers-caravans"].includes(
                room.catanTwoScenario || room.catanScenario || "",
              )}
              tribe={(room.catanTwoScenario || room.catanScenario) === "tribe"}
              pirateIslands={room.catanScenario === "pirate_islands"}
              explorer={supportsPublicExplorerKnights(room.catanScenario)}
              two={room.capacity === 2}
              intro={room.catanScenario === "land-ho"}
              fishing={!!room.catanFishing || room.catanScenario === "fishing"}
              harbors={!!room.catanHarbors?.enabled}
              value={!!room.catanCitiesKnights}
              helpers={!!room.catanOptions?.helpers}
              disabled={!host || busy || mapDirty}
              onChange={(enabled) =>
                command("catan_cities_knights", {
                  catanCitiesKnights: enabled ? { layout: "variable" } : null,
                })
              }
            />
          )}
        {catanEventsSupported(room) && (
          <CatanEventPicker
            value={!!room.catanEvents}
            explorer={[
              "land-ho",
              "pirate-lairs",
              "fish-for-catan",
              "spices-for-catan",
              "explorers-and-pirates",
            ].includes(room.catanScenario || "")}
            transport={room.catanScenario === "transport"}
            pirateIslands={
              (room.catanSeafarers?.scenario || room.catanScenario) ===
              "pirate_islands"
            }
            fishing={room.catanFishing || room.catanScenario === "fishing"}
            friendly={!!room.catanFriendlyRobber?.enabled}
            cloth={
              (room.catanSeafarers?.scenario || room.catanScenario) === "cloth"
            }
            citiesKnights={catanRuleContext(room).citiesKnights}
            disabled={!host || busy || mapDirty}
            onChange={(enabled) => command("catan_events", { enabled })}
          />
        )}
        {room.kind === "catan" && !room.catanTwoRules && (
          <CatanCitiesKnightsSetup room={room} />
        )}
        {room.kind === "catan" &&
          (room.catanTwoRules
            ? supportsTwoCatanVariants(room.catanTwoScenario)
            : supportsPublicCatanFriendly(room.catanScenario)) && (
            <CatanFriendlyRobberPicker
              room={room}
              disabled={!host || busy || mapDirty}
              command={command}
            />
          )}
        {room.kind === "catan" &&
          (room.catanTwoRules
            ? supportsTwoCatanVariants(room.catanTwoScenario)
            : supportsPublicCatanHarbors(room.catanScenario)) && (
            <CatanHarborsPicker
              room={room}
              disabled={!host || busy || mapDirty}
              command={command}
            />
          )}
        {room.kind === "catan" &&
          !room.catanTwoRules &&
          !room.catanScenario && (
            <CatanBasePicker
              room={room}
              disabled={!host || busy || mapDirty}
              command={command}
            />
          )}
        {room.kind === "catan" &&
          (!room.catanTwoRules ||
            supportsTwoCatanSeafarers(room.catanTwoScenario)) &&
          (!room.catanScenario || isPublicCatanSea(room.catanScenario)) && (
            <CatanSeafarersPicker
              room={room}
              hideScenario={
                !!room.catanTwoRules || isPublicCatanSea(room.catanScenario)
              }
              disabled={!host || busy || mapDirty}
              command={command}
            />
          )}
        {room.kind === "catan" &&
          room.catanTwoRules &&
          supportsTwoCatanHelpers(room.catanTwoScenario) && (
            <>
              <CatanOptionPicker
                value={room.catanOptions}
                fiveSixAvailable={false}
                citiesKnights={
                  !!room.catanCitiesKnights ||
                  room.catanTwoScenario === "cities-knights"
                }
                fishing={
                  !!room.catanFishing || room.catanTwoScenario === "fishing"
                }
                harbors={!!room.catanHarbors?.enabled}
                friendlyRobber={!!room.catanFriendlyRobber?.enabled}
                disabled={!host || busy || mapDirty}
                onChange={(catanOptions) =>
                  command("catan_options", { catanOptions })
                }
              />
              {room.catanOptions?.helpers && (
                <p className="muted small">{catanTwoHelpersNote}</p>
              )}
            </>
          )}
        {room.kind === "catan" && isPublicCatanExplorer(room.catanScenario) && (
          <CatanOptionPicker
            explorer
            fiveSixAvailable={false}
            value={room.catanOptions}
            disabled={!host || busy || mapDirty}
            onChange={(catanOptions) =>
              command("catan_options", { catanOptions })
            }
          />
        )}
        {room.kind === "catan" &&
          !room.catanTwoRules &&
          (!room.catanFishing ||
            supportsPublicCatanFishingSeaExtended(room.catanScenario)) &&
          (!room.catanScenario ||
            ["cities-knights", "rivers", "caravans", "fishing"].includes(
              room.catanScenario || "",
            ) ||
            isPublicCatanSea(room.catanScenario)) && (
            <CatanOptionPicker
              value={room.catanOptions}
              helpersAvailable={
                !["rivers", "caravans"].includes(room.catanScenario || "")
              }
              fishing={!!room.catanFishing || room.catanScenario === "fishing"}
              fiveSixAvailable={
                !room.catanFishing ||
                supportsPublicCatanFishingSeaExtended(room.catanScenario)
              }
              seafarers={!!room.catanSeafarers || !!room.catanNewWorldMap}
              citiesKnights={!!room.catanCitiesKnights}
              harbors={!!room.catanHarbors?.enabled}
              friendlyRobber={!!room.catanFriendlyRobber?.enabled}
              disabled={!host || busy || mapDirty}
              onChange={(catanOptions) =>
                command("catan_options", { catanOptions })
              }
            />
          )}
        {room.kind === "splendor" && (
          <SplendorOptionPicker
            value={room.splendorOptions}
            disabled={!host || busy}
            onChange={(splendorOptions) =>
              command("splendor_options", { splendorOptions })
            }
          />
        )}
        {room.kind === "sanguosha" && (
          <>
            <SanguoshaOptions
              value={room.sanguoshaOptions}
              disabled={!host || busy}
              onChange={(sanguoshaOptions) =>
                command("sanguosha_options", { sanguoshaOptions })
              }
            />
            {host && (
              <p className="muted small">更换规则后，所有玩家需要重新准备。</p>
            )}
          </>
        )}
        {room.kind === "rail" && (
          <>
            <RailMapPicker
              maps={maps}
              assets={assets}
              value={room.railMap || "usa"}
              disabled={!host || busy}
              onChange={(railMap) => command("rail_map", { railMap })}
            />
            {host && (
              <p className="muted small">更换地图后，所有玩家需要重新准备。</p>
            )}
          </>
        )}
        <div className="seats-grid">
          {Array.from({ length: room.capacity }, (_, i) => {
            const p = room.seats[i];
            return (
              <div key={i} className={`waiting-seat ${p ? "filled" : ""}`}>
                <span
                  className="avatar"
                  style={{ background: p ? playerColors[i] : undefined }}
                >
                  {p ? p.name[0] : <Plus size={22} />}
                </span>
                <h3>
                  {p ? <PlayerName user={p} /> : "虚位以待"}
                  {p?.id === room.host && <Crown size={14} />}
                  {p?.bot && <small className="bot-badge">AI</small>}
                </h3>
                <span>
                  {p ? (p.ready ? "✓ 已准备" : "等待准备") : "邀请一位朋友"}
                </span>
                {host && p?.bot && (
                  <button
                    className="bot-remove"
                    disabled={busy}
                    aria-label={`移除${p.name}`}
                    onClick={() => command("remove_bot", { target: p.id })}
                  >
                    移除
                  </button>
                )}
              </div>
            );
          })}
        </div>
        {!room.spectating && (
          <>
            <div className="waiting-actions">
              {host && room.seats.length < room.capacity && (
                <button
                  className="outline"
                  disabled={busy}
                  onClick={() => command("add_bot")}
                >
                  添加电脑玩家
                </button>
              )}
              <button
                disabled={busy || mapDirty}
                className={room.seats[room.you]?.ready ? "outline" : "primary"}
                onClick={() => command("ready")}
              >
                {room.seats[room.you]?.ready ? "取消准备" : "我准备好了"}
                <Check size={18} />
              </button>
              {host && (
                <button
                  className="primary gold"
                  disabled={busy || !ready || mapDirty}
                  onClick={() => command("start")}
                >
                  开始游戏
                  <ArrowRight size={18} />
                </button>
              )}
            </div>
          </>
        )}
        <p className="muted small">
          {room.spectating
            ? "等待玩家准备下一局。"
            : host
              ? "电脑自动准备。你准备好后即可开始，也可以继续邀请朋友。"
              : "准备好后，等待房主开始游戏。"}
        </p>
      </div>
    </div>
  );
}
function Players({ room }: { room: Room }) {
  const assets = useContext(AssetsContext);
  const g = room.game!;
  const [nobleOwner, setNobleOwner] = useState<number>();
  useEffect(() => setNobleOwner(undefined), [room.id]);
  return (
    <div className="players-strip">
      {room.seats.map((p, i) => {
        const stats =
          g.splendor?.players[i] ||
          g.rail?.players[i] ||
          g.catan?.players[i] ||
          g.carcassonne?.players[i];
        return (
          <div
            data-player-seat={i}
            className={`player-panel ${g.splendor ? "splendor-player" : ""} ${i === (twoResponder(room) ?? caravanResponder(room) ?? fishResponder(room) ?? g.catan?.cardEvent?.players[0] ?? g.catan?.citiesKnights?.pending?.players[0] ?? g.catan?.seafarers?.pirateIslands?.raid?.rewards[0] ?? g.catan?.seafarers?.tribe?.pending?.player ?? g.catan?.helperPending?.player ?? g.catan?.goldPending?.claims[0]?.player ?? g.turn) && !g.finished ? "current" : ""} ${i === room.you ? "self" : ""} ${stats?.eliminated ? "eliminated" : ""}`}
            key={p.id}
          >
            <span
              className="avatar"
              style={{
                background: g.catan
                  ? catanSeatColor(g.catan, i)
                  : playerColors[i],
                color:
                  g.catan && catanColorIndex(g.catan, i) === 2
                    ? "#564432"
                    : undefined,
              }}
            >
              {p.name[0]}
            </span>
            <div className="player-info">
              <strong>
                <PlayerName user={p} />
                {p.left && (
                  <small>{stats?.eliminated ? "超时离场" : "已离桌"}</small>
                )}
                {i === room.you && <small>你</small>}
                {g.splendor && i === (g.splendor.startPlayer ?? 0) && (
                  <span
                    className="first-player-badge"
                    title="本局先手 · 最后一轮以此座位为起点，所有玩家行动次数相同"
                  >
                    <Flag size={11} aria-hidden="true" />
                    先手
                  </span>
                )}
                {p.bot && <small className="bot-badge">AI</small>}
                {p.autoPlay && (
                  <span className="autoplay-badge" title="由电脑代为行动">
                    <Bot size={12} aria-hidden="true" />
                    托管
                  </span>
                )}
              </strong>
              {g.splendor ? (
                <div
                  className="player-gem-resources"
                  aria-label={`${p.name}的宝石：上方永久折扣，下方现有筹码`}
                >
                  {gemDisplayOrder.map((c) => {
                    const { bonus, tokens } = g.splendor!.players[i];
                    const label =
                      c === 5
                        ? `黄金筹码 ${tokens[c]}`
                        : `${gemNames[c]}：永久折扣 ${bonus[c]}，现有筹码 ${tokens[c]}`;
                    return (
                      <span
                        key={c}
                        className={`player-gem-stack gem-color-${c} ${c === 5 ? "gold-only" : ""}`}
                        style={{ "--gem": gemColors[c] } as React.CSSProperties}
                        role="img"
                        aria-label={label}
                        title={label}
                      >
                        {c !== 5 && (
                          <span className="player-gem-card" aria-hidden="true">
                            <Gemstone color={c} size={13} />
                            <b>{bonus[c]}</b>
                          </span>
                        )}
                        <span className="player-gem-token" aria-hidden="true">
                          <b>{tokens[c]}</b>
                        </span>
                        {c === 5 && (
                          <span
                            className="player-gold-label"
                            aria-hidden="true"
                          >
                            黄金
                          </span>
                        )}
                      </span>
                    );
                  })}
                </div>
              ) : g.carcassonne ? (
                <small>
                  可用随从 <b>{g.carcassonne.players[i].meeples} / 7</b>
                </small>
              ) : g.catan ? (
                <small>
                  {g.catan.citiesKnights ? "资源与商品" : "资源"}{" "}
                  {g.catan.players[i].resourceCount} ·{" "}
                  {g.catan.explorer
                    ? "金币"
                    : g.catan.citiesKnights
                      ? "进步牌"
                      : "发展卡"}{" "}
                  {g.catan.explorer?.economy.gold[i] ??
                    g.catan.citiesKnights?.players[i].progressCount ??
                    g.catan.players[i].devCount}
                  <br />
                  {g.catan.explorer ? (
                    <>
                      船只{" "}
                      {
                        g.catan.explorer.fleet.positions.filter(
                          (at, slot) => Math.floor(slot / 3) === i && at >= 0,
                        ).length
                      }{" "}
                      / 3 · 港口{" "}
                      {
                        g.catan.vertices.filter(
                          (v) =>
                            v.owner === i && explorerIsHarbor(g.catan!, v.id),
                        ).length
                      }
                      {g.catan.citiesKnights && (
                        <CatanCitySeat game={g.catan} seat={i} />
                      )}
                    </>
                  ) : g.catan.citiesKnights ? (
                    <CatanCitySeat game={g.catan} seat={i} />
                  ) : g.catan.seafarers?.pirateIslands ? (
                    <>
                      战舰{" "}
                      {
                        g.catan.edges.filter(
                          (e) => e.owner === i && e.ship && e.warship,
                        ).length
                      }{" "}
                      ·{" "}
                      {g.catan.seafarers.pirateIslands.fortresses[i]
                        .strength === 0
                        ? "✓ 要塞已收复"
                        : `要塞 ${g.catan.seafarers.pirateIslands.fortresses[i].strength}/3`}
                    </>
                  ) : (
                    <>
                      {g.catan.seafarers ? "路线" : "道路"}{" "}
                      {g.catan.players[i].roadLength}
                      {!g.catan.attack && (
                        <> · 骑士 {g.catan.players[i].knights}</>
                      )}
                    </>
                  )}
                  {g.catan.two && g.catan.two.tokenRule !== "none" && (
                    <>
                      {" "}
                      · 筹码 <b>{g.catan.two.tokens[i]}</b>
                    </>
                  )}
                  {g.catan.friendlyRobber?.protectedPlayers.includes(i) && (
                    <>
                      <br />
                      <CatanFriendlyRobberSeat game={g.catan} seat={i} />
                    </>
                  )}
                  {g.catan.transport && (
                    <>
                      <br />
                      <CatanTransportSeat
                        game={g.catan}
                        seat={i}
                        assets={assets}
                      />
                    </>
                  )}
                  {g.catan.attack && (
                    <>
                      <br />
                      <CatanAttackSeat
                        game={g.catan}
                        seat={i}
                        assets={assets}
                      />
                    </>
                  )}
                  {g.catan.rivers && (
                    <>
                      <br />
                      <CatanRiverSeat game={g.catan} seat={i} assets={assets} />
                    </>
                  )}
                  {g.catan.harbors && (
                    <>
                      <br />
                      <CatanHarborsSeat game={g.catan} seat={i} />
                    </>
                  )}
                  {g.catan.seafarers?.cloth && (
                    <>
                      <br />
                      <span className="catan-cloth-seat">
                        <ClothPicture assets={assets} />
                        布匹 {g.catan.seafarers.cloth.held[i]} ·{" "}
                        {Math.floor(g.catan.seafarers.cloth.held[i] / 2)} 分
                      </span>
                    </>
                  )}
                  {g.catan.seafarers?.wonders && (
                    <>
                      <br />
                      <CatanWonderSeat game={g.catan} seat={i} />
                    </>
                  )}
                  {g.catan.seafarers?.tribe && (
                    <>
                      <br />
                      <span className="catan-tribe-seat-ports">
                        部落奖励 {g.catan.seafarers.tribe.points[i]} 分
                      </span>
                      {!!g.catan.seafarers.tribe.heldPorts[i]?.length && (
                        <span className="catan-tribe-seat-ports">
                          {" "}
                          · 暂存港口{" "}
                          {g.catan.seafarers.tribe.heldPorts[i]!.length}
                        </span>
                      )}
                    </>
                  )}
                  {g.catan.paired &&
                    (i === g.catan.paired.primary ||
                      i === g.catan.paired.secondary) && (
                      <>
                        <br />
                        <b>
                          {i === g.catan.paired.primary
                            ? "① 正常回合"
                            : "② 配对行动"}
                        </b>
                      </>
                    )}
                  {g.catan.players[i].helper && (
                    <>
                      <br />
                      助手{" "}
                      {
                        g.catan.helperRules?.find(
                          (h) => h.id === g.catan!.players[i].helper!.id,
                        )?.name
                      }{" "}
                      {g.catan.players[i].helper?.moon ? "☾" : "☀"}
                    </>
                  )}
                </small>
              ) : (
                <small>
                  车厢 {g.rail!.players[i].trains} · 手牌{" "}
                  {g.rail!.players[i].handCount} · 任务{" "}
                  {g.rail!.players[i].ticketCount}
                  {!!g.rail!.mapInfo.stations &&
                    ` · 车站 ${g.rail!.players[i].stations?.length || 0}/${g.rail!.mapInfo.stations}`}
                </small>
              )}
              {g.splendor && (
                <div className="player-gem-summary">
                  <div className="token-summary">
                    <span
                      className="token-total"
                      title="当前持有的宝石总数，包含黄金；回合结束时最多保留 10 枚"
                    >
                      宝石{" "}
                      {g.splendor.players[i].tokens.reduce(
                        (sum, n) => sum + n,
                        0,
                      )}{" "}
                      / 10
                    </span>
                    <span className="reserved-count">
                      预留{" "}
                      {g.splendor.players[i].reserved?.length ??
                        g.splendor.players[i].reservedCount}
                      /3
                    </span>
                  </div>
                </div>
              )}
              {g.splendor?.options?.cities && (
                <div
                  className="player-gem-cities"
                  aria-label={`${p.name}已达成的城市`}
                >
                  {!!g.splendor.cityEligibility?.[i]?.length && (
                    <Check size={13} />
                  )}
                  {g.splendor.cityEligibility?.[i]?.length
                    ? g.splendor.cityEligibility[i]
                        .map((index) => g.splendor!.cities?.[index]?.name)
                        .join("、")
                    : "尚未达成城市"}
                </div>
              )}
              {g.splendor && !g.splendor.options?.cities && (
                <div
                  className="player-nobles"
                  aria-label={`${p.name}已获得的贵族`}
                >
                  <span className="player-nobles-label">
                    <Crown size={13} />
                    贵族 {g.splendor.players[i].nobles.length}
                  </span>
                  {g.splendor.players[i].nobles.length ? (
                    <div className="player-noble-portraits">
                      {g.splendor.players[i].nobles.map((n) => (
                        <button
                          key={n.id}
                          className="noble owned-noble-thumbnail"
                          style={nobleArtwork(n.id, assets)}
                          aria-label={`查看${p.name}的${extraNobleName(n.id) || `贵族 ${n.id}`}，3 分`}
                          aria-haspopup="dialog"
                          onClick={() => setNobleOwner(i)}
                        >
                          <Crown size={18} />
                          <span className="sr-only">
                            {extraNobleName(n.id) || `贵族 ${n.id}`}
                          </span>
                        </button>
                      ))}
                    </div>
                  ) : (
                    <span className="player-nobles-empty">暂无</span>
                  )}
                </div>
              )}
              {!!g.splendor?.players[i].tradingPosts?.length && (
                <div
                  className="player-gem-posts"
                  aria-label={`${p.name}的贸易站`}
                >
                  {g.splendor.players[i].tradingPosts!.map((id) => (
                    <span
                      key={id}
                      title={
                        g.splendor!.tradingPostRules?.find((r) => r.id === id)
                          ?.description
                      }
                    >
                      {gemPostNames[id]}
                    </span>
                  ))}
                </div>
              )}
            </div>
            <div className="player-score">
              {stats?.score}
              <small>分</small>
            </div>
            {i === g.turn && !g.finished && <span className="turn-dot" />}
          </div>
        );
      })}
      {nobleOwner !== undefined && g.splendor && room.seats[nobleOwner] && (
        <Modal
          title={`${room.seats[nobleOwner].name}已获得的贵族`}
          onClose={() => setNobleOwner(undefined)}
        >
          <p>每位贵族提供 3 分，已计入该玩家总分。</p>
          <div className="owned-nobles-detail">
            {g.splendor.players[nobleOwner].nobles.map((n) => (
              <div key={n.id}>
                <NobleCard noble={n} eligible={false} onClick={() => {}} />
                <strong>{extraNobleName(n.id) || `贵族 ${n.id}`} · 3 分</strong>
              </div>
            ))}
          </div>
        </Modal>
      )}
    </div>
  );
}
function Turn({
  room,
  serverNow,
  receivedAt,
}: {
  room: Room;
  serverNow: number;
  receivedAt: number;
}) {
  const g = room.game!;
  const setup = !!g.rail?.setup;
  const autoPlay = !!room.seats[room.you]?.autoPlay;
  const sgActor =
    g.sanguosha?.pending?.player ??
    twoResponder(room) ??
    caravanResponder(room) ??
    fishResponder(room) ??
    g.catan?.cardEvent?.players[0] ??
    g.catan?.citiesKnights?.pending?.players[0] ??
    g.catan?.seafarers?.pirateIslands?.raid?.rewards[0] ??
    g.catan?.seafarers?.tribe?.pending?.player ??
    g.catan?.helperPending?.player ??
    g.catan?.goldPending?.claims[0]?.player ??
    g.turn;
  const mine =
    !room.spectating &&
    (g.dota
      ? !!g.dota.actors?.includes(room.you)
      : g.sanguosha
        ? g.sanguosha.pending
          ? g.sanguosha.pending.canRespond
          : sgActor === room.you
        : twoResponder(room) !== undefined
          ? twoResponder(room) === room.you
          : caravanResponder(room) !== undefined
            ? caravanResponder(room) === room.you
            : fishResponder(room) !== undefined
              ? fishResponder(room) === room.you
              : g.catan?.cardEvent
                ? g.catan.cardEvent.players[0] === room.you
                : g.catan?.citiesKnights?.pending
                  ? g.catan.citiesKnights.pending.players[0] === room.you
                  : g.catan?.seafarers?.pirateIslands?.raid
                    ? g.catan.seafarers.pirateIslands.raid.rewards[0] ===
                      room.you
                    : g.catan?.seafarers?.tribe?.pending
                      ? g.catan.seafarers.tribe.pending.player === room.you
                      : g.catan?.helperPending
                        ? g.catan.helperPending.player === room.you
                        : g.catan?.goldPending
                          ? g.catan.goldPending.claims[0]?.player === room.you
                          : g.phase === "catan_discard"
                            ? (g.catan?.discardDue[room.you] || 0) > 0
                            : setup
                              ? !g.rail?.setupReady?.[room.you]
                              : g.turn === room.you);
  const [tick, setTick] = useState(performance.now());
  const deadline = room.status === "playing" ? room.turnDeadline : 0;
  useEffect(() => {
    if (!deadline) return;
    const timer = setInterval(() => setTick(performance.now()), 250);
    return () => clearInterval(timer);
  }, [deadline]);
  const remaining = deadline
    ? Math.max(
        0,
        Math.min(
          120,
          Math.ceil(
            (deadline - serverNow - Math.max(0, tick - receivedAt)) / 1000,
          ),
        ),
      )
    : 0;
  const expired = !!deadline && remaining === 0;
  const phase: Record<string, string> = {
    ...catanPhases,
    ...(g.catan?.explorer ? { catan_turn: "交易建设，然后开始航行" } : {}),
    ...(g.catan?.seafarers ? catanSeafarerPhases : {}),
    ...(g.catan?.eventDeck ? { catan_roll: "翻开事件牌，再生产资源" } : {}),
    ...(g.catan?.seafarers?.wonders
      ? { catan_robber: catanPhases.catan_robber }
      : {}),
    dota_teams: "确认队伍",
    dota_draft: "选择英雄",
    dota_plan: "暗选行动，全部锁定后揭牌",
    sg_select: "选择武将",
    sg_response: "等待响应",
    sg_play: "选择手牌、技能与目标",
    car_tile: "放置地块",
    car_meeple: "派遣随从",
    turn: "选择一个行动",
    discard: "归还多出的宝石",
    noble: "选择一位贵族",
    gem_post: "选择一个贸易站",
    gem_token: "领取贸易站宝石",
    gem_reserve: "选择一张盲预留卡",
    gem_stronghold: "放置、移动或移除要塞",
    gem_conquest: "选择是否发动征服",
    gem_copy: "选择复制的永久奖励",
    gem_free_card: "免费取得一张发展卡",
    tickets: "选择目的地任务",
    draw: "摸取第二张列车牌",
    finished: "查看本局结算",
  };
  return (
    <div className={`turn-card ${mine ? "mine" : ""}`}>
      <span className="eyebrow">ROUND {String(g.round).padStart(2, "0")}</span>
      <h3>
        {g.finished
          ? "本局已结束"
          : mine && autoPlay
            ? "电脑正在代你行动"
            : g.dota
              ? mine
                ? phase[g.phase]
                : "等待同伴锁定"
              : setup
                ? mine
                  ? "一起选择目的地"
                  : "等待其他人选好"
                : mine
                  ? "轮到你了"
                  : g.sanguosha?.pending?.kind === "nullification"
                    ? "共同响应锦囊"
                    : `${room.seats[sgActor]?.name} ${twoResponder(room) !== undefined || caravanResponder(room) !== undefined || fishResponder(room) !== undefined || g.sanguosha?.pending || g.catan?.attack?.pending || g.catan?.attack?.endPlan || g.catan?.cardEvent || g.catan?.citiesKnights?.pending || g.catan?.seafarers?.pirateIslands?.raid || g.catan?.seafarers?.tribe?.pending || g.catan?.helperPending || g.catan?.goldPending ? "正在响应" : "的回合"}`}
      </h3>
      <p>
        {g.finished
          ? "感谢同桌，好局下次再来。"
          : mine && autoPlay
            ? "可随时取消托管，恢复手动操作。"
            : g.dota
              ? "超时自动托管，回来可随时接管。"
              : setup
                ? "所有人同时选牌，超时由电脑选牌并开启托管。"
                : mine
                  ? phase[g.phase]
                  : "稍等片刻，想想下一步。"}
      </p>
      {!!deadline && (
        <div className={`turn-clock ${remaining <= 20 ? "urgent" : ""}`}>
          <div className="clock-reading">
            <Clock size={20} />
            <time aria-label={`本回合剩余 ${remaining} 秒`}>
              {String(Math.floor(remaining / 60)).padStart(2, "0")}:
              {String(remaining % 60).padStart(2, "0")}
            </time>
            <span>
              {expired
                ? "电脑正在接管"
                : g.catan?.attack?.pending || g.catan?.attack?.endPlan
                  ? "蛮族进攻响应 120 秒"
                  : g.phase === "catan_world_fish"
                    ? "渔场安放 120 秒"
                    : g.phase === "catan_world_ports"
                      ? "港口安放 120 秒"
                      : setup
                        ? "共同选牌限时"
                        : g.sanguosha?.pending && !g.sanguosha.selecting
                          ? "响应限时 20 秒"
                          : twoResponder(room) !== undefined
                            ? "双人选择 120 秒"
                            : caravanResponder(room) !== undefined
                              ? "商队响应 120 秒"
                              : fishResponder(room) !== undefined
                                ? "捕鱼换筹码 120 秒"
                                : g.catan?.cardEvent
                                  ? "事件牌响应 120 秒"
                                  : g.catan?.citiesKnights?.pending
                                    ? "城市与骑士响应 120 秒"
                                    : g.catan?.seafarers?.pirateIslands?.raid
                                      ? "防守奖励 120 秒"
                                      : g.catan?.seafarers?.tribe?.pending
                                        ? "港口安放 120 秒"
                                        : g.catan?.helperPending
                                          ? "助手选择 120 秒"
                                          : g.catan?.goldPending
                                            ? "金矿选择 120 秒"
                                            : "每回合 120 秒"}
            </span>
          </div>
          {expired && !g.finished && (
            <p>超时自动开启托管，玩家回来后可随时取消。</p>
          )}
        </div>
      )}
      {(g.splendor?.lastRound || (g.rail?.lastRemaining ?? -1) >= 0) &&
        !g.finished && (
          <div className="final-round">
            <Flag size={15} />
            最后一轮{g.rail ? ` · 剩余 ${g.rail.lastRemaining} 回合` : ""}
          </div>
        )}
      <div className="save-state">
        <Check size={13} />
        进度已自动保存
      </div>
    </div>
  );
}
function Results({
  room,
  host,
  busy,
  command,
}: {
  room: Room;
  host: boolean;
  busy: boolean;
  command: (t: string) => void;
}) {
  const g = room.game!;
  return (
    <section className="results">
      <div>
        <span className="eyebrow">A GAME WELL PLAYED</span>
        <h2>
          <Crown size={25} />
          {g.winners.map((i) => room.seats[i]?.name).join("、")} 获胜
        </h2>
        <p>
          {g.dota
            ? "敌方遗迹已被摧毁，获胜队伍的所有玩家共同获胜。"
            : g.sanguosha
              ? "身份阵营结算：获胜阵营的成员共同获胜，包括已经阵亡的队友。"
              : g.carcassonne
                ? "地块用尽后结算未完成建筑与田地；总分最高者获胜，同分共同获胜。离场玩家不参与排名。"
                : g.catan
                  ? catanResultDescription(g.catan)
                  : g.splendor
                    ? splendorResultDescription(g.splendor)
                    : "总分 = 路线分 + 目的地净得分 + 本地图奖励。"}
        </p>
      </div>
      {room.result && (
        <div className="result-ratings">
          {room.result.rated ? (
            room.result.players.map((p) => (
              <span key={p.id}>
                <PlayerName user={p} />{" "}
                <b
                  className={
                    (p.ratingDelta || 0) > 0 ? "rating-gain" : "rating-loss"
                  }
                >
                  {(p.ratingDelta || 0) > 0 ? "+" : ""}
                  {p.ratingDelta} 积分
                </b>
              </span>
            ))
          ) : (
            <p>本局不计积分（含电脑或积分上线前开始的对局）。</p>
          )}
        </div>
      )}
      {g.rail && (
        <div className="score-table">
          {room.seats.map((p, i) => {
            const x = g.rail!.players[i];
            return (
              <div key={p.id}>
                <strong>
                  <PlayerName user={p} />
                </strong>
                <span>路线 {x.routeScore}</span>
                <span>
                  任务 {x.ticketScore > 0 ? "+" : ""}
                  {x.ticketScore}
                </span>
                <span>
                  {g.rail!.mapInfo.bonus === "network"
                    ? `网络 ${x.network || 0} 城`
                    : g.rail!.mapInfo.bonus === "tickets"
                      ? `完成 ${x.completed || 0} 张`
                      : `最长 ${x.longest} 节`}{" "}
                  / +{x.bonus}
                  {!!g.rail!.mapInfo.stations &&
                    ` · 车站 +${x.stationScore || 0}`}
                  {g.rail!.map === "india" && ` · 环游 +${x.mandalaScore || 0}`}
                </span>
                <b>{x.score} 分</b>
              </div>
            );
          })}
        </div>
      )}
      {host && (
        <button
          className="primary"
          disabled={busy}
          onClick={() => command("rematch")}
        >
          <RotateCcw size={17} />
          再来一局
        </button>
      )}
    </section>
  );
}
function Cost({ cost }: { cost: number[] }) {
  return (
    <div className="costs">
      {cost.map(
        (n, i) =>
          n > 0 && (
            <span
              className={`cost color-${i}`}
              key={i}
              style={{
                background: gemColors[i],
                color: i === 1 ? "#46443a" : "white",
              }}
              title={gemNames[i]}
            >
              {n}
            </span>
          ),
      )}
    </div>
  );
}
function DevCard({
  card,
  onClick,
  selected = false,
  affordable = false,
}: {
  card: Card;
  onClick?: () => void;
  selected?: boolean;
  affordable?: boolean;
}) {
  const assets = useContext(AssetsContext);
  if (card.orient)
    return (
      <OrientCard
        card={card}
        assets={assets}
        onClick={onClick}
        selected={selected}
        affordable={affordable}
        renderGem={(color) => <Gemstone color={color} size={26} />}
        renderCost={(cost) => <Cost cost={cost} />}
      />
    );
  return (
    <button
      className={`dev-card gem-${card.color} ${selected ? "selected" : ""} ${affordable ? "affordable" : ""}`}
      data-card-id={card.id}
      onClick={onClick}
      aria-label={`${affordable ? "可购买，" : ""}${gemNames[card.color]}发展卡，${card.points}分，费用 ${card.cost
        .map((n, i) => (n ? `${gemNames[i]}${n}` : ""))
        .filter(Boolean)
        .join("，")}`}
      style={
        {
          "--card-color": gemColors[card.color],
          "--cost-columns": Math.min(2, card.cost.filter((n) => n > 0).length),
          "--art-x": `${((card.tier - 1) * 2 + (card.id % 2)) * 20}%`,
          "--art-y": `${[3, 4, 0, 1, 2][card.color] * 20}%`,
        } as React.CSSProperties
      }
    >
      <div className="card-top">
        <strong>{card.points}</strong>
        <Gemstone color={card.color} size={31} />
      </div>
      <div className="card-art">
        <svg viewBox="0 0 160 85">
          <path
            d="M0 80L35 28l23 19 29-34 40 45 33-19v46H0Z"
            fill="currentColor"
            opacity=".35"
          />
          <path
            d="M48 76V40h13V29h14V12h13v17h14v11h13v36M43 77h78"
            fill="none"
            stroke="currentColor"
            opacity=".55"
            strokeWidth="3"
          />
          <path d="M66 77V54q16-18 31 0v23" fill="currentColor" opacity=".5" />
          <circle
            cx={23 + (card.id % 5) * 25}
            cy="20"
            r="10"
            fill="currentColor"
            opacity=".22"
          />
        </svg>
      </div>
      <Cost cost={card.cost} />
      <span className="card-id">
        {["Ⅰ", "Ⅱ", "Ⅲ"][card.tier - 1]} · {String(card.id).padStart(2, "0")}
      </span>
    </button>
  );
}
function NobleCard({
  noble,
  eligible,
  onClick,
}: {
  noble: Noble;
  eligible: boolean;
  onClick: () => void;
}) {
  const assets = useContext(AssetsContext);
  return (
    <button
      className={`noble ${extraNobleName(noble.id) ? "extra-noble" : ""} ${eligible ? "eligible" : ""}`}
      data-noble-id={noble.id}
      style={nobleArtwork(noble.id, assets)}
      onClick={onClick}
      aria-label={`${extraNobleName(noble.id) || `贵族 ${noble.id}`}，3 分，需要 ${noble.cost
        .map((n, i) => (n ? `${gemNames[i]}发展卡${n}张` : ""))
        .filter(Boolean)
        .join("，")}`}
    >
      <span className="noble-points">3</span>
      <svg viewBox="0 0 70 65" aria-hidden="true">
        <path d="M14 65q1-25 21-25t22 25" fill="#637c71" />
        <ellipse cx="35" cy="26" rx="13" ry="17" fill="#d4b889" />
        <path d="M21 26Q10 5 35 6q24 1 14 24l-4-13-22 6" fill="#69583e" />
        <path d="m22 46 13 14 13-14-7-6-6 10-6-10" fill="#f5e7c4" />
        <path d="M18 10h33l-5-7-9 4-9-7-3 8Z" fill="#d7b35b" />
      </svg>
      <Cost cost={noble.cost} />
    </button>
  );
}
function SplendorBoard({
  room,
  act,
  busy,
}: {
  room: Room;
  act: Act;
  busy: boolean;
}) {
  const assets = useContext(AssetsContext);
  const g = room.game!,
    s = g.splendor!,
    p = s.players[room.you] ?? {
      tokens: Array(6).fill(0),
      bonus: Array(5).fill(0),
      reserved: [],
      cards: [],
      nobles: [],
      score: 0,
    },
    mine = !room.spectating && g.turn === room.you && !g.finished;
  const canAfford = (card: Card) =>
    mine &&
    g.phase === "turn" &&
    room.status === "playing" &&
    (!s.strongholds?.[card.id] || s.strongholds[card.id].player === room.you) &&
    gemCanBuy(p, card);
  const [tokens, setTokens] = useState<number[]>(Array(6).fill(0));
  const [selected, setSelected] = useState<Card>();
  const [blindTier, setBlindTier] = useState<number>();
  const [conquestPurchase, setConquestPurchase] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(true);
  const [collection, setCollection] = useState(false);
  const selectCard = (card: Card, conquest = false) => {
    setConquestPurchase(conquest);
    setSelected(card);
  };
  useEffect(() => {
    setTokens(Array(6).fill(0));
    setSelected(undefined);
    setBlindTier(undefined);
  }, [room.version]);
  useEffect(() => {
    if (g.phase === "discard") setDiscardOpen(true);
  }, [g.phase, g.turn]);
  const taking = mine && g.phase === "turn",
    discard = mine && g.phase === "discard";
  return (
    <div className={`splendor-board ${s.options?.orient ? "has-orient" : ""}`}>
      <h2 className="sr-only">璀璨宝石游戏桌面</h2>
      {s.options?.cities ? (
        <SplendorCities
          room={room}
          assets={assets}
          renderGem={(color) => <Gemstone color={color} />}
        />
      ) : (
        <div className="nobles-row">
          <div className="board-section-label">
            <Crown size={18} />
            <span>
              贵族来访<small>满足条件 · 获得 3 分</small>
            </span>
          </div>
          <div className="nobles">
            {s.nobles.map((n) => (
              <NobleCard
                key={n.id}
                noble={n}
                eligible={gemNobleEligible(p, n)}
                onClick={() => {
                  if (mine && g.phase === "noble")
                    void act({ type: "noble", noble: n.id });
                }}
              />
            ))}
          </div>
        </div>
      )}
      <SplendorMarket
        assets={assets}
        room={room}
        busy={busy}
        onReserve={setBlindTier}
        renderCard={(c) => (
          <DevCard
            card={c}
            affordable={canAfford(c)}
            selected={selected?.id === c.id}
            onClick={() => selectCard(c)}
          />
        )}
      />
      <SplendorExpansionBoard
        onConquest={(card) => selectCard(card, true)}
        room={room}
        assets={assets}
        act={act}
        busy={busy}
        renderCard={(card, onClick) => (
          <DevCard card={card} onClick={onClick} />
        )}
        renderCost={(cost) => <Cost cost={cost} />}
        renderGem={(color) => <Gemstone color={color} />}
      />
      <div className="gem-bank">
        <div className="section-line">
          <h3>公共宝石</h3>
          <small>三种各一枚，或同色两枚（该堆至少四枚）</small>
        </div>
        <div className="token-row">
          {gemDisplayOrder.map((i) => {
            const n = s.bank[i];
            return (
              <button
                key={i}
                className={`token-button ${tokens[i] > 0 ? "chosen" : ""}`}
                data-bank-color={i}
                disabled={!taking || busy || i === 5 || n === 0}
                onClick={() =>
                  setTokens(
                    tokens.map((v, c) =>
                      c === i ? (v >= Math.min(2, n) ? 0 : v + 1) : v,
                    ),
                  )
                }
                aria-label={`拿取${gemNames[i]}，供应 ${n}，已选 ${tokens[i]}`}
              >
                <span
                  className={`token color-${i}`}
                  style={
                    {
                      "--gem": gemColors[i],
                      backgroundPosition: `${i * 20}% 0`,
                    } as React.CSSProperties
                  }
                >
                  <Gemstone color={i} size={31} />
                </span>
                <b>{n}</b>
                <small>{gemNames[i]}</small>
                {tokens[i] > 0 && (
                  <span className="token-picked">+{tokens[i]}</span>
                )}
              </button>
            );
          })}
        </div>
        {taking && (
          <div className="bank-actions">
            <span>已选择 {tokens.reduce((a, b) => a + b, 0)} 枚</span>
            <button
              className="subtle"
              onClick={() => setTokens(Array(6).fill(0))}
            >
              重选
            </button>
            <button
              className="primary"
              disabled={busy || tokens.every((n) => n === 0)}
              onClick={() => void act({ type: "take", tokens })}
            >
              拿取宝石
              <Check size={16} />
            </button>
          </div>
        )}
      </div>
      {!room.spectating && (
        <div className="personal-area">
          <div className="section-line">
            <h3>你的商会</h3>
            <button
              className="subtle"
              onClick={() => setCollection(!collection)}
            >
              发展卡 {p.cards.length}
              {!s.options?.cities && ` · 贵族 ${p.nobles.length}`}
              <ChevronRight size={15} />
            </button>
          </div>
          <div className="splendor-wallet" aria-label="你的宝石与永久折扣">
            <div className="wallet-caption">
              <span>
                我的宝石 <small>永久折扣 / 手持筹码</small>
              </span>
              <b>{p.tokens.reduce((sum, n) => sum + n, 0)} / 10</b>
            </div>
            <div className="wallet-colors">
              {gemDisplayOrder.map((i) => (
                <div
                  key={i}
                  className={`wallet-gem color-${i}`}
                  style={
                    {
                      "--gem": gemColors[i],
                      "--ink": i === 1 || i === 5 ? "#332d20" : "#fff",
                    } as React.CSSProperties
                  }
                >
                  <Gemstone color={i} size={24} />
                  <div className="wallet-counts">
                    {i < 5 && (
                      <span
                        className={`wallet-discount ${p.bonus[i] ? "" : "empty"}`}
                        title={`${gemNames[i]}永久折扣 ${p.bonus[i]}`}
                      >
                        <span
                          aria-hidden="true"
                          className="discount-card-icon"
                        />
                        <b>{p.bonus[i]}</b>
                        <span className="sr-only">永久折扣</span>
                      </span>
                    )}
                    <span
                      className="wallet-held"
                      title={`${gemNames[i]}筹码 ${p.tokens[i]}`}
                    >
                      <strong>{p.tokens[i]}</strong>
                      <span
                        aria-hidden="true"
                        className="wallet-token"
                        style={{ backgroundPosition: `${i * 20}% 0` }}
                      />
                      <span className="sr-only">手持筹码</span>
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className="reserved">
            <span>
              预留卡 <b>{p.reserved?.length || 0}/3</b>
            </span>
            {p.reserved?.length ? (
              p.reserved.map((c) => (
                <DevCard
                  key={c.id}
                  card={c}
                  affordable={canAfford(c)}
                  selected={selected?.id === c.id}
                  onClick={() => selectCard(c)}
                />
              ))
            ) : (
              <p>预留心仪的发展卡，也能获得一枚黄金。</p>
            )}
          </div>
          {collection && (
            <div className="collection">
              {p.cards.map((c) => (
                <DevCard key={c.id} card={c} />
              ))}
              {p.nobles.map((n) => (
                <NobleCard key={n.id} noble={n} eligible onClick={() => {}} />
              ))}
            </div>
          )}
        </div>
      )}
      {mine && g.phase === "noble" && (
        <div className="action-prompt">
          多位贵族青睐你的商会，请点击一位符合条件的贵族。
        </div>
      )}
      {discard && (
        <section
          className={`discard-panel ${discardOpen ? "" : "is-collapsed"}`}
          aria-label="归还宝石"
        >
          <header>
            <h3>请归还 {p.tokens.reduce((a, b) => a + b, 0) - 10} 枚宝石</h3>
            <button
              className="subtle"
              aria-expanded={discardOpen}
              aria-controls="discard-controls"
              onClick={() => setDiscardOpen(!discardOpen)}
            >
              {discardOpen ? "查看牌桌" : "继续归还"}
            </button>
          </header>
          {discardOpen && (
            <div id="discard-controls">
              <p>回合结束时最多持有 10 枚，包括黄金。</p>
              <div className="discard-list">
                {p.tokens.map((n, i) => (
                  <div key={i}>
                    <Gemstone color={i} />
                    <span>
                      {gemNames[i]} · 持有 {n}
                    </span>
                    <button
                      className="icon-button"
                      aria-label={`减少归还${gemNames[i]}`}
                      disabled={tokens[i] === 0}
                      onClick={() =>
                        setTokens(tokens.map((v, c) => (c === i ? v - 1 : v)))
                      }
                    >
                      <Minus size={15} />
                    </button>
                    <b>{tokens[i]}</b>
                    <button
                      className="icon-button"
                      aria-label={`增加归还${gemNames[i]}`}
                      disabled={tokens[i] >= n}
                      onClick={() =>
                        setTokens(tokens.map((v, c) => (c === i ? v + 1 : v)))
                      }
                    >
                      <Plus size={15} />
                    </button>
                  </div>
                ))}
              </div>
              <button
                className="primary wide"
                disabled={
                  busy ||
                  tokens.reduce((a, b) => a + b, 0) !==
                    p.tokens.reduce((a, b) => a + b, 0) - 10
                }
                onClick={() => void act({ type: "discard", tokens })}
              >
                归还所选宝石
              </button>
            </div>
          )}
        </section>
      )}
      {blindTier !== undefined && taking && (
        <Modal
          title={`确认盲预留 · ${blindTier >= 3 ? "东方 " : ""}${(blindTier % 3) + 1} 级牌堆`}
          dismissible={!busy}
          onClose={() => setBlindTier(undefined)}
        >
          <p>
            {p.tradingPosts?.includes(2)
              ? s.remaining[blindTier] === 1
                ? "牌堆只剩一张发展卡，查看后预留这一张。"
                : `查看 ${blindTier >= 3 ? "东方 " : ""}${(blindTier % 3) + 1} 级牌堆顶部两张发展卡，选择一张预留，另一张放回牌堆底部。`
              : `随机预留一张 ${blindTier >= 3 ? "东方 " : ""}${(blindTier % 3) + 1} 级发展卡，确认前无法查看牌面。`}
            这会使用本回合的行动。
          </p>
          <p>
            {s.bank[5] > 0
              ? "同时获得 1 枚黄金。如果持有宝石超过 10 枚，需要归还多出的宝石。"
              : "公共供应区没有黄金，本次仅预留卡牌。"}
          </p>
          <div className="form-grid">
            <button
              className="outline"
              disabled={busy}
              onClick={() => setBlindTier(undefined)}
            >
              取消
            </button>
            <button
              className="primary"
              disabled={
                busy || !s.remaining[blindTier] || p.reserved!.length >= 3
              }
              onClick={() =>
                void act({
                  type: "reserve",
                  tier: (blindTier % 3) + 1,
                  choice: blindTier >= 3 ? "orient" : "",
                })
              }
            >
              确认预留
            </button>
          </div>
        </Modal>
      )}
      {selected && (
        <SplendorPurchase
          room={room}
          card={selected}
          conquest={conquestPurchase}
          act={act}
          busy={busy}
          onClose={() => setSelected(undefined)}
          renderCard={(card, onClick) => (
            <DevCard card={card} onClick={onClick} />
          )}
          renderCost={(cost) => <Cost cost={cost} />}
        />
      )}
      {taking && (
        <button
          className="pass-button"
          disabled={busy}
          onClick={() => void act({ type: "pass" })}
        >
          无合法行动时跳过回合
        </button>
      )}
      <SplendorCardAnimation
        room={room}
        assets={assets}
        renderCard={(card) => <DevCard card={card} />}
        renderNoble={(noble) => (
          <NobleCard noble={noble} eligible={false} onClick={() => {}} />
        )}
      />
      <SplendorTokenAnimation
        room={room}
        assets={assets}
        colors={gemColors}
        renderGem={(color) => <Gemstone color={color} size={26} />}
      />
    </div>
  );
}
function TrainCard({
  color,
  count,
  onClick,
  disabled = false,
  label,
}: {
  color: number;
  count?: number;
  onClick?: () => void;
  disabled?: boolean;
  label?: string;
}) {
  const previousCount = useRef(count);
  const [gained, setGained] = useState(0);
  useEffect(() => {
    if (
      count !== undefined &&
      previousCount.current !== undefined &&
      count > previousCount.current
    )
      setGained((n) => n + 1);
    previousCount.current = count;
  }, [count]);
  useEffect(() => {
    if (!gained) return;
    const timer = setTimeout(() => setGained(0), 900);
    return () => clearTimeout(timer);
  }, [gained]);
  return (
    <button
      key={gained}
      data-hand-color={count !== undefined ? color : undefined}
      className={`train-card train-${color} ${gained ? "just-gained" : ""}`}
      onAnimationEnd={() => setGained(0)}
      style={
        {
          "--train": trainColors[color],
          backgroundPosition: `${(color === 8 ? 0 : color + 1) * 12.5}% 0`,
        } as React.CSSProperties
      }
      onClick={onClick}
      disabled={disabled}
      aria-label={
        label ||
        `${trainNames[color]}列车牌${count !== undefined ? ` ${count} 张` : ""}`
      }
    >
      <TrainFront size={28} />
      <div className="train-stripes" />
      {count !== undefined && <b>{count}</b>}
    </button>
  );
}
function TicketCard({
  ticket,
  catalog,
  selected,
  onClick,
  showResult = false,
  showProgress = false,
}: {
  ticket: Ticket;
  catalog: Catalog;
  selected?: boolean;
  onClick?: () => void;
  showResult?: boolean;
  showProgress?: boolean;
}) {
  const assets = useContext(AssetsContext);
  const a = catalog.cities[ticket.a],
    b = catalog.cities[ticket.b];
  return (
    <button
      style={
        assets
          ? {
              backgroundImage: `url("${assets}/${catalog.map.art}/tickets/${ticket.id}.webp")`,
            }
          : undefined
      }
      className={`ticket-card ${catalog.map.height > catalog.map.width ? "portrait-ticket" : ""} ${ticket.options ? "country-ticket" : ""} ${selected ? "selected" : ""} ${showResult ? (ticket.complete ? "completed" : "incomplete") : showProgress && ticket.complete ? "completed" : ""}`}
      onClick={onClick}
    >
      <div className="ticket-label">
        DESTINATION TICKET{" "}
        <span>
          {showResult
            ? ticket.complete
              ? "已完成"
              : "未完成"
            : selected
              ? "✓ 保留"
              : "目的地"}
        </span>
      </div>
      <div className="ticket-route">
        <strong>{cityName(a.name)}</strong>
        <ArrowRight size={16} />
        <strong>{ticket.options ? "多个目的地" : cityName(b.name)}</strong>
      </div>
      <small>
        {a.name} — {b.name}
      </small>
      {ticket.options && (
        <div className="ticket-options">
          {ticket.options.map((o) => (
            <span key={o.to}>
              {cityName(catalog.cities[o.to].name)} · {o.points} 分
            </span>
          ))}
        </div>
      )}
      {ticket.mandala && <span className="ticket-mandala">✓ 环游达成</span>}
      {showProgress && (
        <span className={`ticket-progress ${ticket.complete ? "done" : ""}`}>
          {ticket.complete ? (
            <>
              <Check size={12} />
              已完成
            </>
          ) : (
            "未完成"
          )}
        </span>
      )}
      <div
        className={`ticket-bottom ${assets && catalog.map.id !== "usa" && !showResult && !ticket.options ? "sr-only" : ""}`}
      >
        <svg viewBox="0 0 110 35">
          <path
            d="m4 25 19-12 19 11 27-15 17 14 19-17"
            fill="none"
            stroke="#a38e62"
            strokeDasharray="3 3"
          />
          <circle cx="4" cy="25" r="3" fill="#174e43" />
          <circle cx="105" cy="6" r="3" fill="#c65342" />
        </svg>
        <b>
          {showResult ? (ticket.complete ? "+" : "−") : ""}
          {showResult ? Math.abs(ticket.value ?? ticket.points) : ticket.points}
          <small>{ticket.options && !showResult ? "分起" : "分"}</small>
        </b>
      </div>
    </button>
  );
}
const wagonColors = ["blue", "red", "green", "yellow", "black"];
function RailMap({
  catalog,
  owners,
  selected,
  onSelect,
  highlight,
  players,
  stationMode,
  onStation,
}: {
  catalog: Catalog;
  players: RailPlayer[];
  stationMode: boolean;
  onStation: (city: number) => void;
  owners: Record<string, number>;
  selected?: Route;
  onSelect: (r: Route) => void;
  highlight?: Ticket;
}) {
  const { viewport, zoom, zoomAt, dragging, canvasStyle, mapStyle, handlers } =
    useRailMapControls({
      aspect: catalog.map.width / catalog.map.height,
      minMobileWidth: catalog.map.width < catalog.map.height ? 360 : 620,
      fillMobileHeight: true,
    });
  const [showCities, setShowCities] = useState(
    () => localStorage.getItem("wb_rail_city_names") !== "off",
  );
  const assets = useContext(AssetsContext);
  const [wagonReady, setWagonReady] = useState<boolean[]>([]);
  useEffect(() => {
    let cancelled = false;
    setWagonReady([]);
    if (assets)
      wagonColors.forEach((color, i) => {
        const image = new Image();
        image.onload = () => {
          if (!cancelled)
            setWagonReady((ready) => {
              const next = [...ready];
              next[i] = true;
              return next;
            });
        };
        image.src = `${assets}/rail/wagon-${color}-v2.webp`;
      });
    return () => {
      cancelled = true;
    };
  }, [assets]);
  return (
    <div className="map-frame">
      <div className="map-controls">
        <button
          className="city-toggle"
          aria-pressed={showCities}
          onClick={() => {
            setShowCities(!showCities);
            localStorage.setItem(
              "wb_rail_city_names",
              showCities ? "off" : "on",
            );
          }}
        >
          <Eye size={15} />
          城市名{showCities ? "：开" : "：关"}
        </button>
        <div>
          <button
            className="icon-button"
            aria-label="缩小地图"
            disabled={zoom <= 1}
            onClick={() => zoomAt(zoom - 0.25)}
          >
            <ZoomOut size={17} />
          </button>
          <span>{Math.round(zoom * 100)}%</span>
          <button
            className="icon-button"
            aria-label="放大地图"
            disabled={zoom >= 3}
            onClick={() => zoomAt(zoom + 0.25)}
          >
            <ZoomIn size={17} />
          </button>
        </div>
      </div>
      <div
        ref={viewport}
        className={`map-scroll ${dragging ? "is-dragging" : ""}`}
        {...handlers}
      >
        <div className="map-canvas" style={canvasStyle}>
          <svg
            className="rail-map"
            viewBox={`0 0 ${catalog.map.width} ${catalog.map.height}`}
            style={mapStyle}
            role="group"
            aria-label={`${catalog.map.name}铁路地图，点击路线选择占领`}
          >
            <rect
              width={catalog.map.width}
              height={catalog.map.height}
              fill="#ece8d8"
            />
            {assets && (
              <image
                href={`${assets}/${catalog.map.art}/${catalog.map.id === "usa" ? "map-unlabeled-v4.webp" : "map.webp"}`}
                width={catalog.map.width}
                height={catalog.map.height}
              />
            )}
            {catalog.routes.map((r) => {
              const a = catalog.cities[r.a],
                b = catalog.cities[r.b];
              const owner = owners[r.id],
                claimed = owner !== undefined;
              const active = selected?.id === r.id;
              return (
                <g
                  key={r.id}
                  className="map-route"
                  role="button"
                  tabIndex={0}
                  aria-label={`${cityName(a.name)} 到 ${cityName(b.name)}，${r.length} 节，${r.color < 0 ? "任意单色" : trainNames[r.color]}${claimed ? "，已占领" : ""}`}
                  onClick={() => onSelect(r)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onSelect(r);
                    }
                  }}
                >
                  <title>
                    {cityName(a.name)} → {cityName(b.name)} · {r.length} 节
                    {r.tunnel ? " · 隧道" : ""}
                    {r.ferry ? ` · 万能×${r.ferry}` : ""}
                    {r.mountain ? ` · 山地×${r.mountain}` : ""}
                    {claimed ? ` · 玩家 ${owner + 1}` : ""}
                  </title>
                  {r.segments?.map((segment, i) => (
                    <g
                      key={i}
                      transform={`translate(${segment.x},${segment.y}) rotate(${segment.angle})`}
                    >
                      <rect
                        x="-31"
                        y="-13"
                        width="62"
                        height="26"
                        rx="4"
                        fill="transparent"
                      />
                      {active && (
                        <rect
                          x="-31"
                          y="-15"
                          width="62"
                          height="30"
                          rx="6"
                          fill="#f5cf4740"
                          stroke="#f5cf47"
                          strokeWidth="3"
                        />
                      )}
                      {claimed ? (
                        wagonReady[owner] ? (
                          <image
                            className="claimed-wagon"
                            href={`${assets}/rail/wagon-${wagonColors[owner]}-v2.webp`}
                            x="-33"
                            y="-14"
                            width="66"
                            height="28"
                            preserveAspectRatio="none"
                            pointerEvents="none"
                          />
                        ) : (
                          <g
                            fill={playerColors[owner]}
                            stroke="#17252b"
                            strokeWidth="1.1"
                            pointerEvents="none"
                          >
                            <path d="M-27-6L-22-11H22L27-6V7H-27Z" />
                            <path
                              d="M-22-10H22L25-6H-25Z"
                              fill="#fff"
                              opacity=".28"
                              stroke="none"
                            />
                            <path
                              d="M-27 7H27V10H-27ZM-20 10V13H-13V10M13 10V13H20V10"
                              fill="#253039"
                            />
                            <path
                              d="M-18-4V5M-9-4V5M0-4V5M9-4V5M18-4V5"
                              stroke="#000"
                              opacity=".25"
                            />
                          </g>
                        )
                      ) : (
                        <rect
                          x="-26"
                          y="-8"
                          width="52"
                          height="16"
                          rx="3"
                          fill={
                            assets
                              ? "transparent"
                              : r.color < 0
                                ? "#bbb4a1"
                                : trainColors[r.color]
                          }
                        />
                      )}
                    </g>
                  ))}
                </g>
              );
            })}
            {catalog.cities.map((c) => {
              const countryLit =
                !!highlight &&
                c.kind === "border" &&
                [
                  highlight.a,
                  highlight.b,
                  ...(highlight.options?.map((o) => o.to) || []),
                ].some(
                  (id) =>
                    catalog.cities[id].kind === "country" &&
                    catalog.cities[id].country === c.country,
                );
              if (c.kind === "border" && !countryLit) return null;
              const lit =
                countryLit ||
                (highlight &&
                  (c.id === highlight.a ||
                    c.id === highlight.b ||
                    highlight.options?.some((o) => o.to === c.id)));
              const stationOwner = players.findIndex((p) =>
                p.stations?.includes(c.id),
              );
              const [lx, ly, lw, lh] = c.label || [c.x + 12, c.y - 30, 100, 30];
              return (
                <g
                  key={c.id}
                  className={`map-city ${stationMode ? "station-city" : ""}`}
                  role={stationMode ? "button" : undefined}
                  tabIndex={stationMode ? 0 : undefined}
                  aria-label={
                    stationMode ? `在${cityName(c.name)}建造车站` : undefined
                  }
                  onClick={() => {
                    if (stationMode) onStation(c.id);
                  }}
                  onKeyDown={(e) => {
                    if (stationMode && (e.key === "Enter" || e.key === " ")) {
                      e.preventDefault();
                      onStation(c.id);
                    }
                  }}
                >
                  {stationMode && (
                    <circle
                      cx={c.x}
                      cy={c.y}
                      r={23}
                      fill="#f2ce6488"
                      stroke="#8e5c21"
                      strokeWidth={3}
                    />
                  )}
                  {lit && (
                    <circle
                      cx={c.x}
                      cy={c.y}
                      r="23"
                      fill="#f1b52f"
                      opacity=".7"
                    />
                  )}
                  {c.kind !== "country" && (
                    <circle
                      cx={c.x}
                      cy={c.y}
                      r={lit ? 9 : 5}
                      fill={lit ? "#b53427" : "#f6f0dc"}
                      stroke="#4d503c"
                      strokeWidth="2"
                    />
                  )}
                  {stationOwner >= 0 && (
                    <g
                      className="map-station"
                      transform={`translate(${c.x},${c.y})`}
                    >
                      {assets ? (
                        <image
                          href={`${assets}/${catalog.map.art}/station-${wagonColors[stationOwner]}.webp`}
                          x={-20}
                          y={-34}
                          width={40}
                          height={52}
                        />
                      ) : (
                        <path
                          d="M-17 9V-8L0-22L17-8V9ZM-5 9V-3H5V9"
                          fill={playerColors[stationOwner]}
                          stroke="#fff6d4"
                          strokeWidth={3}
                        />
                      )}
                      <title>玩家 {stationOwner + 1} 的车站</title>
                    </g>
                  )}
                  {showCities && c.kind !== "border" && (
                    <rect
                      x={lx}
                      y={ly}
                      width={lw}
                      height={lh}
                      rx="5"
                      fill="#f4f0e3"
                    />
                  )}
                  {showCities && c.kind !== "border" && (
                    <text
                      x={lx + lw / 2}
                      y={ly + lh / 2}
                      textAnchor="middle"
                      style={{
                        fontSize: Math.min(
                          22,
                          (lw - 4) / cityName(c.name).length,
                        ),
                      }}
                      dominantBaseline="central"
                    >
                      {cityName(c.name)}
                    </text>
                  )}
                </g>
              );
            })}
          </svg>
        </div>
      </div>
      <div className="map-caption">
        <span>滚轮缩放 · 按住拖动 · 点击路线查看费用</span>
        <span>灰色路线可使用任意一种颜色</span>
      </div>
    </div>
  );
}
function RailBoard({
  room,
  act,
  busy,
}: {
  room: Room;
  act: Act;
  busy: boolean;
}) {
  const [catalog, setCatalog] = useState<Catalog>();
  const [loadError, setLoadError] = useState("");
  const [selected, setSelected] = useState<Route>();
  const [color, setColor] = useState(0);
  const [wild, setWild] = useState(0);
  const [keep, setKeep] = useState<number[]>([]);
  const [ticketChoiceOpen, setTicketChoiceOpen] = useState(true);
  const [confirmTickets, setConfirmTickets] = useState(false);
  const [highlight, setHighlight] = useState<Ticket>();
  const [stationMode, setStationMode] = useState(false);
  const [stationCity, setStationCity] = useState<number>();
  const [payment, setPayment] = useState<RailPayment>({
    color: 0,
    tokens: Array(9).fill(0),
  });
  const [tunnelOpen, setTunnelOpen] = useState(true);
  const [tunnelWild, setTunnelWild] = useState(-1);
  const g = room.game!,
    r = g.rail!,
    p = r.players[room.you] ?? {
      hand: Array(9).fill(0),
      handCount: 0,
      tickets: [],
      ticketCount: 0,
      trains: 0,
      stations: [],
      stationRoutes: [],
    },
    mine =
      !room.spectating &&
      (r.setup ? !r.setupReady?.[room.you] : g.turn === room.you) &&
      !g.finished,
    turn = mine && !r.setup && g.phase === "turn",
    drawing = mine && !r.setup && (g.phase === "turn" || g.phase === "draw");
  useEffect(() => {
    let alive = true;
    setCatalog(undefined);
    setLoadError("");
    setHighlight(undefined);
    api(`/catalog?map=${encodeURIComponent(r.map || "usa")}`)
      .then((value) => {
        if (alive) setCatalog(value);
      })
      .catch((e) => {
        if (alive) setLoadError(e.message);
      });
    return () => {
      alive = false;
    };
  }, [r.map]);
  useEffect(() => {
    setSelected(undefined);
    setConfirmTickets(false);
    setStationMode(false);
    setStationCity(undefined);
    setTunnelOpen(true);
    setTunnelWild(-1);
  }, [g.turn, g.phase, g.round]);
  useEffect(() => {
    setKeep([]);
    setTicketChoiceOpen(true);
  }, [r.pending?.map((t) => t.id).join(",")]);
  if (loadError)
    return <p role="alert">地图加载失败：{loadError}。请刷新重试。</p>;
  if (!catalog) return <div className="loading">正在展开地图…</div>;
  const city = (id: number) => cityName(catalog.cities[id].name);
  const selectRoute = (route: Route) => {
    setSelected(route);
    setStationMode(false);
    const suggested = r.payments?.[route.id]?.[0];
    setPayment(
      suggested
        ? { color: suggested.color, tokens: [...suggested.tokens] }
        : { color: Math.max(0, route.color), tokens: Array(9).fill(0) },
    );
    const c =
      route.color >= 0
        ? route.color
        : Math.max(
            0,
            p.hand!.findIndex((n) => n >= route.length),
          );
    setColor(c > 7 ? 0 : c);
    setWild(
      Math.min(
        route.length,
        Math.max(0, route.length - p.hand![c > 7 ? 0 : c]),
      ),
    );
  };
  const parallelBlocked =
    selected &&
    catalog.routes.some(
      (x) =>
        x.id !== selected.id &&
        ((x.a === selected.a && x.b === selected.b) ||
          (x.a === selected.b && x.b === selected.a)) &&
        r.owners[x.id] !== undefined &&
        (room.seats.length < catalog.map.doubleMin ||
          r.owners[x.id] === room.you),
    );
  return (
    <div
      className={`rail-board ${catalog.map.height > catalog.map.width ? "portrait-board" : ""}`}
    >
      <h2 className="sr-only">铁路环游 · {catalog.map.name}</h2>
      <div className="rail-map-legend">
        <span className="tag">{catalog.map.name}地图</span>
        <span>
          {
            {
              usa: "经典路线",
              europe: "隧道 · 渡轮 · 车站",
              india: "渡轮 · 环游奖励",
              switzerland: "隧道 · 国家任务",
              nordiccountries: "隧道 · 渡轮 · 任务奖励",
              legendaryasia: "山地 · 渡轮 · 网络奖励",
            }[catalog.map.id]
          }
        </span>
        {!!catalog.map.stations && !room.spectating && (
          <button
            className="outline"
            disabled={
              !turn || busy || (p.stations?.length || 0) >= catalog.map.stations
            }
            onClick={() => {
              setSelected(undefined);
              setStationMode(!stationMode);
            }}
          >
            {stationMode ? "取消建站" : "建造车站"} · 剩余{" "}
            {catalog.map.stations - (p.stations?.length || 0)}
          </button>
        )}
      </div>
      {stationMode && (
        <p className="rail-special-note">
          点击地图上的城市建站。第 {(p.stations?.length || 0) + 1} 座车站需{" "}
          {(p.stations?.length || 0) + 1} 张同色牌。
        </p>
      )}
      {r.tunnel && (
        <section
          className={`ticket-choice ${tunnelOpen ? "" : "is-collapsed"}`}
          aria-label="隧道追加费用"
        >
          <header>
            <h3>
              {city(catalog.routes.find((x) => x.id === r.tunnel!.route)!.a)} →{" "}
              {city(catalog.routes.find((x) => x.id === r.tunnel!.route)!.b)} ·
              隧道检查
            </h3>
            <button
              className="subtle"
              onClick={() => setTunnelOpen(!tunnelOpen)}
            >
              {tunnelOpen ? "收起看地图" : "展开检查牌"}
            </button>
          </header>
          {tunnelOpen && (
            <div>
              <div className="tunnel-revealed">
                {r.tunnel.revealed.map((c, i) => (
                  <TrainCard key={i} color={c} />
                ))}
              </div>
              <p>
                原支付：{paymentText(r.tunnel.base)}。需追加 {r.tunnel.extra} 张
                {r.tunnel.wildOnly
                  ? "万能"
                  : `${trainNames[r.tunnel.color]}或万能`}
                牌。
              </p>
              {mine ? (
                <>
                  <p className="muted small">
                    放弃会退回原支付牌，并结束本回合。
                  </p>
                  {!r.tunnel.wildOnly && (
                    <label>
                      追加费用中的万能牌
                      <select
                        aria-label="隧道追加万能牌"
                        value={
                          tunnelWild < 0
                            ? Math.max(
                                0,
                                r.tunnel.extra - p.hand![r.tunnel.color],
                              )
                            : tunnelWild
                        }
                        onChange={(e) => setTunnelWild(Number(e.target.value))}
                      >
                        {Array.from({ length: r.tunnel.extra + 1 }, (_, i) => (
                          <option
                            key={i}
                            value={i}
                            disabled={
                              i > p.hand![8] ||
                              r.tunnel!.extra - i > p.hand![r.tunnel!.color]
                            }
                          >
                            {i} 张万能 + {r.tunnel!.extra - i} 张
                            {trainNames[r.tunnel!.color]}牌
                          </option>
                        ))}
                      </select>
                    </label>
                  )}
                  <div className="modal-actions">
                    <button
                      className="outline"
                      disabled={busy}
                      onClick={() => void act({ type: "tunnel_cancel" })}
                    >
                      放弃并退牌
                    </button>
                    <button
                      className="primary"
                      disabled={
                        busy ||
                        (r.tunnel.wildOnly
                          ? p.hand![8]
                          : p.hand![r.tunnel.color] + p.hand![8]) <
                          r.tunnel.extra
                      }
                      onClick={() =>
                        void act({
                          type: "tunnel_pay",
                          wild: r.tunnel!.wildOnly
                            ? r.tunnel!.extra
                            : tunnelWild >= 0
                              ? tunnelWild
                              : Math.max(
                                  0,
                                  r.tunnel!.extra - p.hand![r.tunnel!.color],
                                ),
                        })
                      }
                    >
                      追加支付并铺路
                    </button>
                  </div>
                </>
              ) : (
                <p>等待 {room.seats[g.turn].name} 决定追加支付或放弃。</p>
              )}
            </div>
          )}
        </section>
      )}
      {mine && g.phase === "tickets" && (
        <section
          className={`ticket-choice ${ticketChoiceOpen ? "" : "is-collapsed"}`}
          aria-label="选择目的地任务"
        >
          <header>
            <h3>
              {ticketChoiceOpen
                ? r.setup
                  ? "选择你的第一段旅程"
                  : "新的目的地，新的机会"
                : `目的地 · 已选 ${keep.length} 张`}
            </h3>
            <button
              className="subtle"
              aria-expanded={ticketChoiceOpen}
              aria-controls="ticket-choice-controls"
              onClick={() => setTicketChoiceOpen(!ticketChoiceOpen)}
            >
              {ticketChoiceOpen ? "收起看地图" : "继续选择"}
            </button>
          </header>
          {ticketChoiceOpen && (
            <div id="ticket-choice-controls">
              <p>
                从这些任务中至少保留 <strong>{r.setup ? 2 : 1}</strong>{" "}
                张。未完成的任务会在结算时扣分。
                {r.setup && "所有人同时选择；120 秒后由电脑选牌并开启托管。"}
              </p>
              {catalog.map.height > catalog.map.width && (
                <small className="portrait-ticket-hint">
                  左右滑动车票查看全部任务，点击选择保留。
                </small>
              )}
              <div className="pending-tickets">
                {r.pending?.map((t) => (
                  <TicketCard
                    key={t.id}
                    ticket={t}
                    catalog={catalog}
                    selected={keep.includes(t.id)}
                    onClick={() => {
                      setHighlight(t);
                      setKeep(
                        keep.includes(t.id)
                          ? keep.filter((x) => x !== t.id)
                          : [...keep, t.id],
                      );
                    }}
                  />
                ))}
              </div>
              <button
                className="primary wide"
                disabled={busy || keep.length < (r.setup ? 2 : 1)}
                onClick={() => void act({ type: "keep", keep })}
              >
                保留 {keep.length} 张目的地任务
                <Check size={17} />
              </button>
            </div>
          )}
        </section>
      )}

      {confirmTickets && (
        <Modal
          title="领取新的目的地任务"
          onClose={() => setConfirmTickets(false)}
        >
          <p>
            将抽取最多 3 张任务，至少保留 1
            张，并消耗本回合。未完成的任务会在结算时扣分。
          </p>
          <div className="modal-actions">
            <button
              className="outline"
              onClick={() => setConfirmTickets(false)}
            >
              取消
            </button>
            <button
              className="primary"
              disabled={!turn || busy || r.ticketsRemaining === 0}
              onClick={() => {
                setConfirmTickets(false);
                void act({ type: "tickets" });
              }}
            >
              确认领取
            </button>
          </div>
        </Modal>
      )}
      <HiddenDrawAnimation room={room} />
      <div
        className={`rail-table ${catalog.map.height > catalog.map.width ? "portrait-map" : ""}`}
      >
        <RailMap
          key={catalog.map.id}
          catalog={catalog}
          players={r.players}
          stationMode={stationMode}
          onStation={(id) => {
            setStationCity(id);
            const cost = (p.stations?.length || 0) + 1;
            const hand = p.hand!.slice(0, 8);
            const enough = hand.findIndex((n) => n >= cost);
            const c = enough >= 0 ? enough : hand.indexOf(Math.max(...hand));
            setColor(c);
            setWild(Math.max(0, cost - hand[c]));
          }}
          owners={r.owners}
          selected={selected}
          onSelect={selectRoute}
          highlight={highlight}
        />
        <div className="train-market">
          <div className="section-line">
            <h3>列车牌市场</h3>
            <small>
              {catalog.map.wildSingle
                ? g.phase === "draw" && mine
                  ? "还可摸 1 张；可拿万能牌"
                  : "每回合摸两张；公开万能牌计一张"
                : g.phase === "draw" && mine
                  ? "还可以摸 1 张（不能拿公开万能牌）"
                  : "每回合摸两张；公开万能牌占用整个行动"}
            </small>
          </div>
          <div className="train-market-cards">
            <button
              className="train-deck"
              aria-label="摸一张暗牌"
              disabled={!drawing || busy}
              onClick={() => void act({ type: "draw", slot: -1 })}
            >
              <TrainFront size={28} />
              <strong>
                <span className="train-deck-label-full">摸一张暗牌</span>
                <span className="train-deck-label-short" aria-hidden="true">
                  暗牌
                </span>
              </strong>
              <small>可摸 {r.remaining} 张</small>
            </button>
            {r.face.map((c, i) => (
              <AnimatedSlot
                key={i}
                identity={`${c}:${r.faceVersion?.[i] ?? 0}`}
                marker={i}
              >
                {c < 0 ? (
                  <div className="train-card empty">牌堆已空</div>
                ) : (
                  <TrainCard
                    color={c}
                    disabled={
                      !drawing ||
                      busy ||
                      (g.phase === "draw" && c === 8 && !catalog.map.wildSingle)
                    }
                    onClick={() => void act({ type: "draw", slot: i })}
                    label={`拿取公开${trainNames[c]}列车牌，第 ${i + 1} 张`}
                  />
                )}
              </AnimatedSlot>
            ))}
          </div>
          <button
            className="ticket-deck"
            disabled={!turn || busy || r.ticketsRemaining === 0}
            onClick={() => setConfirmTickets(true)}
          >
            <Flag size={25} />
            <strong>领取目的地</strong>
            <small>抽 3 留至少 1 · 余 {r.ticketsRemaining}</small>
          </button>
        </div>
      </div>
      {!room.spectating && (
        <div className="personal-area rail-personal">
          <div className="section-line">
            <h3>
              你的列车手牌 <span>{p.handCount}</span>
            </h3>
            <span className="tag">
              剩余车厢 {p.trains} / {catalog.map.trains}
            </span>
          </div>
          <div className="train-hand">
            {p.hand!.map((n, c) => (
              <TrainCard key={c} color={c} count={n} disabled={n === 0} />
            ))}
          </div>
          {!!catalog.map.stations && (
            <p className="muted small">
              车站按总得分最高的借路方案结算，各任务共用同一方案。当前借路：
              {p.stationRoutes
                ?.filter(Boolean)
                .map((id) => {
                  const x = catalog.routes.find((r) => r.id === id)!;
                  return `${city(x.a)} → ${city(x.b)}`;
                })
                .join("、") || "暂无"}
              （仅你可见）
            </p>
          )}
          <div className="section-line ticket-heading">
            <h3>
              你的目的地{" "}
              <span>
                已完成 {p.tickets?.filter((t) => t.complete).length || 0} /{" "}
                {p.tickets?.length || 0}
              </span>
            </h3>
            <small>点击车票，在地图上标记起终点</small>
          </div>
          <div className="tickets">
            {p.tickets?.map((t) => (
              <TicketCard
                key={t.id}
                ticket={t}
                catalog={catalog}
                selected={highlight?.id === t.id}
                onClick={() =>
                  setHighlight(highlight?.id === t.id ? undefined : t)
                }
                showResult={g.finished}
                showProgress
              />
            ))}
          </div>
        </div>
      )}
      {g.finished && (
        <div className="personal-area">
          <h3>所有玩家的目的地结算</h3>
          {room.seats.map((seat, i) => (
            <details key={seat.id}>
              <summary>
                <PlayerName user={seat} /> · 完成 {r.players[i].completed} 张 ·
                净得分 {r.players[i].ticketScore}
              </summary>
              <div className="tickets">
                {r.players[i].tickets?.map((t) => (
                  <TicketCard
                    key={t.id}
                    ticket={t}
                    catalog={catalog}
                    showResult
                    onClick={() => setHighlight(t)}
                  />
                ))}
              </div>
            </details>
          ))}
        </div>
      )}
      {selected && (
        <Modal
          title={`${city(selected.a)} → ${city(selected.b)}`}
          onClose={() => setSelected(undefined)}
        >
          <div className="route-preview">
            <TrainFront size={28} />
            <strong>{selected.length} 节车厢</strong>
            <span>铺设得 {routePoints(selected)} 分</span>
          </div>
          {r.owners[selected.id] !== undefined ? (
            <p>
              这条路线已由{" "}
              <strong>{room.seats[r.owners[selected.id]].name}</strong> 占领。
            </p>
          ) : parallelBlocked ? (
            <p>
              这条双线目前不可用。少于 {catalog.map.doubleMin}{" "}
              人只能使用其中一条，同一玩家也不能同时占领双线。
            </p>
          ) : room.spectating ? (
            <p>
              这条路线尚未铺设，需要 {selected.length} 张
              {selected.color >= 0
                ? trainNames[selected.color]
                : "同一种颜色的"}
              列车牌，可使用万能牌替代。
            </p>
          ) : (
            <>
              {(selected.tunnel ||
                selected.ferry ||
                selected.mountain ||
                selected.substitute ||
                catalog.map.wildRule !== "any") && (
                <p className="rail-special-note">
                  {selected.tunnel &&
                    "隧道：翻开三张检查牌后，可能需要追加支付；可放弃并退回原支付牌。 "}
                  {!!selected.ferry &&
                    `渡轮：至少 ${selected.ferry} 张万能牌。 `}
                  {!!selected.mountain &&
                    `山地：另消耗 ${selected.mountain} 节车厢，奖励已计入上方得分。 `}
                  {selected.substitute === 3 &&
                    "每张必需万能牌可用任意三张牌替代。"}
                  {selected.substitute === 4 &&
                    "每张同色牌可用任意四张牌替代。"}
                  {catalog.map.wildRule !== "any" && "本地图万能牌只能用于隧道"}
                  {catalog.map.wildRule === "special" && "和渡轮"}
                </p>
              )}
              <label>
                路线主颜色
                <select
                  aria-label="路线主颜色"
                  value={payment.color}
                  disabled={selected.color >= 0}
                  onChange={(e) =>
                    setPayment({ ...payment, color: Number(e.target.value) })
                  }
                >
                  {trainNames.slice(0, 8).map((n, i) => (
                    <option key={i} value={i}>
                      {n}
                    </option>
                  ))}
                </select>
              </label>
              <RailPaymentEditor
                hand={p.hand!}
                value={payment}
                onChange={setPayment}
                suggestions={r.payments?.[selected.id]}
              />
              {!r.payments?.[selected.id]?.length && (
                <p className="muted small">
                  目前手牌或车厢不足以铺设这条路线。
                </p>
              )}
              <button
                className="primary wide"
                disabled={
                  !turn ||
                  busy ||
                  p.trains < selected.length + (selected.mountain || 0) ||
                  !railPaymentValid(
                    selected,
                    catalog.map,
                    p.hand!,
                    payment.color,
                    payment.tokens,
                  )
                }
                onClick={() =>
                  void act({
                    type: "claim",
                    route: selected.id,
                    color: payment.color,
                    tokens: payment.tokens,
                  })
                }
              >
                {selected.tunnel ? "尝试铺设隧道" : "铺设这条铁路"}
                <TrainFront size={18} />
              </button>
            </>
          )}
        </Modal>
      )}
      {stationCity !== undefined && (
        <Modal
          title={`在${city(stationCity)}建造车站`}
          onClose={() => setStationCity(undefined)}
        >
          <p>
            第 {(p.stations?.length || 0) + 1}{" "}
            座车站需相同数量的同色牌，可以万能牌补足。每个城市只能有一座车站。未建车站结算时每座得
            4 分。
          </p>
          <label>
            支付颜色
            <select
              value={color}
              onChange={(e) => {
                setColor(Number(e.target.value));
                setWild(
                  Math.max(
                    0,
                    (p.stations?.length || 0) +
                      1 -
                      p.hand![Number(e.target.value)],
                  ),
                );
              }}
            >
              {trainNames.slice(0, 8).map((n, i) => (
                <option key={i} value={i}>
                  {n} · {p.hand![i]} 张
                </option>
              ))}
            </select>
          </label>
          <label>
            使用万能牌
            <select
              value={wild}
              onChange={(e) => setWild(Number(e.target.value))}
            >
              {Array.from({ length: (p.stations?.length || 0) + 2 }, (_, i) => (
                <option key={i} value={i}>
                  {i} 张
                </option>
              ))}
            </select>
          </label>
          <button
            className="primary wide"
            disabled={
              !turn ||
              busy ||
              r.players.some((x) => x.stations?.includes(stationCity)) ||
              p.hand![color] < (p.stations?.length || 0) + 1 - wild ||
              p.hand![8] < wild
            }
            onClick={() =>
              void act({ type: "station", vertex: stationCity, color, wild })
            }
          >
            确认建造车站
          </button>
        </Modal>
      )}
      {turn && (
        <button
          className="pass-button"
          disabled={busy}
          onClick={() => void act({ type: "pass" })}
        >
          无合法行动时跳过回合
        </button>
      )}
    </div>
  );
}
function Rules({
  kind,
  railMap,
  sgOptions,
  room,
}: {
  kind: string;
  railMap?: RailMapSpec;
  sgOptions?: SGOptions;
  room: Room;
}) {
  return (
    <div className="rules">
      {kind === "dota" ? (
        <DotaRules />
      ) : kind === "sanguosha" ? (
        <SanguoshaRules options={sgOptions} />
      ) : kind === "carcassonne" ? (
        <>
          <p>
            卡卡颂基础版，2–5 人，72 张地块，每人 7
            名随从，包含农民。不含河流、修道院长等扩展。
          </p>
          <ol>
            <li>
              抽到地块后旋转并放在相邻空位，所有接边的道路、城市和田地必须匹配。完全无法放置的地块移出本局并重抽。
            </li>
            <li>
              可以在刚放的地块上派遣一名随从，也可以不派遣。相连的整个区域必须没有任何人的随从。
            </li>
            <li>
              完整道路每块 1 分；完整城市每块及每枚盾徽各 2
              分；修道院自身和八个邻格各 1 分。完成后随从回到手中。
            </li>
            <li>
              终局未完成道路每块 1 分，未完成城市每块及每枚盾徽各 1
              分，修道院按已占据邻格计分。
            </li>
            <li>
              农民留在田地直到终局，每座相邻完整城市 3
              分。同一田地的同一城市只计一次；道路和城墙会分隔田地。
            </li>
            <li>
              相连区域随从最多的玩家得分，并列各得全部分数。总分最高者获胜，同分共同获胜。
            </li>
          </ol>
          <p>
            每回合 120
            秒，放地块与派随从共用倒计时。超时自动开启电脑托管，可随时取消并继续手动操作。
          </p>
        </>
      ) : kind === "catan" ? (
        <CatanRules room={room} />
      ) : kind === "splendor" ? (
        <SplendorRules room={room} />
      ) : (
        <>{railMap && <RailMapRules map={railMap} />}</>
      )}
      {kind !== "sanguosha" && kind !== "catan" && (
        <p>
          每回合120秒，弃牌、贵族选择和第二次摸牌共用本回合计时。超时自动开启电脑托管，玩家保留席位，可随时取消托管。房主可直接结束牌桌。
        </p>
      )}
      <p className="muted small">
        操作由服务器验证。每次行动自动保存，刷新或重新登录后回到原来的座位。
      </p>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
