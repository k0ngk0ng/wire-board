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
  RotateCcw,
  Flag,
  Minus,
  ZoomIn,
  ZoomOut,
  Crown,
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
} from "./types";
import "./style.css";
import { cityName } from "./cities";
import { GameAudio } from "./audio";
const AssetsContext = createContext("");
const gemColors = [
  "#218357",
  "#f7f2dd",
  "#2974bb",
  "#343c43",
  "#c94440",
  "#dcac38",
];
const gemNames = ["祖母绿", "钻石", "蓝宝石", "缟玛瑙", "红宝石", "黄金"];
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
  kind === "splendor" ? "璀璨宝石" : "铁路环游";
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
      {kind === "splendor" ? (
        <svg viewBox="0 0 520 260" preserveAspectRatio="xMidYMid slice">
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
        <svg viewBox="0 0 520 260" preserveAspectRatio="xMidYMid slice">
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
}: {
  title: string;
  dismissible?: boolean;
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
        className="modal"
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
  const [state, setState] = useState<State>();
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [online, setOnline] = useState(false);
  const [create, setCreate] = useState("");
  const [join, setJoin] = useState<Room>();
  const [rules, setRules] = useState(false);
  const [sound, setSound] = useState(
    () => localStorage.getItem("wb_sound") !== "off",
  );
  const [notice, setNotice] = useState("");
  const seq = useRef(0);
  const previousTurn = useRef("");
  const previousRoom = useRef<Room | undefined>(undefined);
  const audio = useRef<GameAudio>(new GameAudio());
  const refresh = async () => {
    const n = ++seq.current;
    try {
      const data = await api("/state");
      if (n === seq.current) {
        if (previousRoom.current?.status === "playing" && !data.room) {
          setNotice("你已因回合超时被移出本局，可以创建或加入其他房间。");
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
    const connect = () => {
      socket = new WebSocket(
        `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/ws`,
      );
      socket.onopen = () => setOnline(true);
      socket.onmessage = () => void refresh();
      socket.onclose = () => {
        setOnline(false);
        if (!stopped) timer = setTimeout(connect, 2000);
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
    const key = `${r.id}:${r.game!.round}:${r.game!.turn}:${r.status}`;
    if (key !== previousTurn.current && sound) {
      if ((r.game!.finished || r.status === "closed") && previousTurn.current) {
        void audio.current.play("finish");
      } else if (r.status === "playing" && r.game!.turn === r.you) {
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
        submit={(body, register) => {
          if (sound) enableAudio();
          void run(() => api(register ? "/register" : "/login", body));
        }}
      />
    );
  const room = state.room;
  return (
    <AssetsContext.Provider value={state.assetsBaseURL || ""}>
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
            <span className="nav-active">{room ? "游戏牌桌" : "桌游大厅"}</span>
            <span className="nav-caption">让相聚，多一局。</span>
          </nav>
          <div className="header-right">
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
              title={sound ? "音效已开启，点击静音" : "音效已关闭，点击开启"}
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
            <span className="avatar">{state.user.name[0]}</span>
            <span className="username">{state.user.name}</span>
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
        {!room ? (
          <Lobby
            state={state}
            onCreate={setCreate}
            onJoin={(r) => {
              if (r.locked) setJoin(r);
              else void run(() => command(r, "join"));
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
                {room.status !== "playing" && (
                  <button
                    className="subtle"
                    disabled={busy}
                    onClick={() => roomCommand("leave")}
                  >
                    <ArrowLeft size={16} />
                    离开房间
                  </button>
                )}
                {room.status === "playing" && room.host === state.user.id && (
                  <button
                    className="subtle danger"
                    disabled={busy}
                    onClick={() => {
                      if (
                        confirm(
                          "结束当前牌桌？本局不计胜负，所有玩家可以离开或重新开局。",
                        )
                      )
                        roomCommand("close");
                    }}
                  >
                    结束牌桌
                  </button>
                )}
              </div>
            </div>
            {room.status === "waiting" ? (
              <Waiting
                room={room}
                busy={busy}
                host={room.host === state.user.id}
                command={roomCommand}
              />
            ) : room.status === "closed" ? (
              <div className="closed-panel">
                <Flag size={40} />
                <h2>这张牌桌已结束</h2>
                <p>休息一下，或者准备下一局。</p>
                {room.host === state.user.id && (
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
                <Players room={room} />
                {room.game?.finished && (
                  <Results
                    room={room}
                    host={room.host === state.user.id}
                    busy={busy}
                    command={roomCommand}
                  />
                )}
                <div className="game-layout">
                  <div className="game-main">
                    {room.kind === "splendor" ? (
                      <SplendorBoard room={room} act={act} busy={busy} />
                    ) : (
                      <RailBoard room={room} act={act} busy={busy} />
                    )}
                  </div>
                  <aside className="game-sidebar">
                    <Turn
                      room={room}
                      serverNow={state.serverNow}
                      receivedAt={state.receivedAt}
                      busy={busy}
                      command={roomCommand}
                    />
                    <div className="journal">
                      <h3>
                        牌桌动态 <span>LIVE</span>
                      </h3>
                      <ol>
                        {room.game?.log
                          .slice(-12)
                          .reverse()
                          .map((line, i) => (
                            <li key={`${room.version}-${i}`}>
                              {line.replace(
                                /玩家 (\d+)/g,
                                (_, n) =>
                                  room.seats[Number(n) - 1]?.name ||
                                  `玩家 ${n}`,
                              )}
                            </li>
                          ))}
                        {!room.game?.log.length && (
                          <li>牌已洗好，祝你好运。</li>
                        )}
                      </ol>
                    </div>
                  </aside>
                </div>
              </>
            )}
          </main>
        )}
        {!room && (
          <footer>
            围桌 WIRE BOARD <span>好游戏，和好朋友一起。</span>
            <span>私人牌桌 · 自动保存</span>
          </footer>
        )}
        {create && (
          <Create
            kind={create}
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
            title={`加入「${join.name}」`}
            onClose={() => setJoin(undefined)}
          >
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                void run(async () => {
                  await command(join, "join", { password: f.get("password") });
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
                加入牌桌
                <ArrowRight size={18} />
              </button>
            </form>
          </Modal>
        )}
        {rules && (
          <Modal
            title={`${gameName(room!.kind)} · 玩法速查`}
            onClose={() => setRules(false)}
          >
            <Rules kind={room!.kind} />
          </Modal>
        )}
      </div>
    </AssetsContext.Provider>
  );
}
function Auth({
  busy,
  error,
  submit,
}: {
  busy: boolean;
  error: string;
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
              璀璨宝石
            </span>
            <span>
              <TrainFront size={17} />
              铁路环游
            </span>
            <span>两款经典，无限好时光</span>
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
  busy,
}: {
  state: State;
  onCreate: (v: string) => void;
  onJoin: (r: Room) => void;
  busy: boolean;
}) {
  const [filter, setFilter] = useState("all");
  const rooms = state.rooms
    .filter((r) => filter === "all" || r.kind === filter)
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
          <div>
            <Gemstone color={4} size={30} />
            <Gemstone color={2} size={30} />
            <Gemstone color={0} size={30} />
          </div>
          <small>MAKE TIME TO PLAY</small>
        </div>
      </section>
      <section className="game-selection">
        {["splendor", "rail"].map((kind) => (
          <article className="game-feature" key={kind}>
            <Cover kind={kind} />
            <div className="feature-info">
              <div className="feature-kicker">
                {kind === "splendor"
                  ? "THE ART OF COLLECTING"
                  : "EVERY ROUTE TELLS A STORY"}
                <span>基础版</span>
              </div>
              <h2>
                {gameName(kind)}
                <small>
                  {kind === "splendor" ? "Splendor" : "Ticket to Ride"}
                </small>
              </h2>
              <p>
                {kind === "splendor"
                  ? "从宝石商人到财富大师，每一次选择都闪闪发光。"
                  : "从海岸到海岸，让你的铁路连接每一个目的地。"}
              </p>
              <div className="feature-bottom">
                <span>
                  <Users size={15} />
                  {kind === "splendor" ? "2–4" : "2–5"} 人 <Clock size={15} />
                  {kind === "splendor" ? "30" : "45–60"} 分钟
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
          <button className="outline" onClick={() => onCreate("splendor")}>
            <Plus size={17} />
            创建房间
          </button>
        </div>
        <div className="filter-tabs">
          {[
            ["all", "全部桌游"],
            ["splendor", "璀璨宝石"],
            ["rail", "铁路环游"],
          ].map(([k, label]) => (
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
                      {gameName(r.kind)} · {r.id.toUpperCase()}
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
                      {p.name[0]}
                    </span>
                  ))}
                  <small>
                    {r.seats.length} / {r.capacity}
                  </small>
                </div>
                <span className={`status ${r.status}`}>
                  {statusName[r.status]}
                </span>
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
              onClick={() => onCreate(filter === "all" ? "splendor" : filter)}
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
  kind,
  busy,
  onClose,
  onSubmit,
}: {
  kind: string;
  busy: boolean;
  onClose: () => void;
  onSubmit: (body: unknown) => void;
}) {
  const [k, setK] = useState(kind);
  const [capacity, setCapacity] = useState(4);
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
            capacity,
          });
        }}
      >
        <div className="choose-games">
          {["splendor", "rail"].map((x) => (
            <button
              type="button"
              className={k === x ? "selected" : ""}
              key={x}
              onClick={() => {
                setK(x);
                setCapacity(Math.min(capacity, x === "rail" ? 5 : 4));
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
              onChange={(e) => setCapacity(Number(e.target.value))}
            >
              {Array.from(
                { length: k === "rail" ? 4 : 3 },
                (_, i) => i + 2,
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
          至少 2 人即可开始，不必坐满。所有玩家准备后，由房主开局。
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
  command: (t: string) => void;
}) {
  const ready = room.seats.every((p) => p.ready) && room.seats.length >= 2;
  return (
    <div className="waiting-layout">
      <div className="waiting-cover">
        <Cover kind={room.kind} />
        <h2>{gameName(room.kind)}</h2>
        <p>
          {room.kind === "splendor"
            ? "每颗宝石，都是通往胜利的可能。"
            : "下一站，会是你的目的地吗？"}
        </p>
        <span className="tag">
          {room.kind === "splendor"
            ? "原版基础规则 · 2–4 人"
            : "美国基础地图 · 2–5 人"}
        </span>
      </div>
      <div className="waiting-seats">
        <span className="eyebrow">TAKE YOUR SEAT</span>
        <h2>朋友到齐，就开局。</h2>
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
                  {p ? p.name : "虚位以待"}
                  {p?.id === room.host && <Crown size={14} />}
                </h3>
                <span>
                  {p ? (p.ready ? "✓ 已准备" : "等待准备") : "邀请一位朋友"}
                </span>
              </div>
            );
          })}
        </div>
        <div className="waiting-actions">
          <button
            disabled={busy}
            className={room.seats[room.you]?.ready ? "outline" : "primary"}
            onClick={() => command("ready")}
          >
            {room.seats[room.you]?.ready ? "取消准备" : "我准备好了"}
            <Check size={18} />
          </button>
          {host && (
            <button
              className="primary gold"
              disabled={busy || !ready}
              onClick={() => command("start")}
            >
              开始游戏
              <ArrowRight size={18} />
            </button>
          )}
        </div>
        <p className="muted small">
          {host
            ? "所有玩家准备后，你就可以开始游戏。"
            : "准备好后，等待房主开始游戏。"}
        </p>
      </div>
    </div>
  );
}
function Players({ room }: { room: Room }) {
  const g = room.game!;
  return (
    <div className="players-strip">
      {room.seats.map((p, i) => {
        const stats = g.splendor?.players[i] || g.rail?.players[i];
        return (
          <div
            className={`player-panel ${i === g.turn && !g.finished ? "current" : ""} ${i === room.you ? "self" : ""} ${g.splendor?.players[i].eliminated || g.rail?.players[i].eliminated ? "eliminated" : ""}`}
            key={p.id}
          >
            <span className="avatar" style={{ background: playerColors[i] }}>
              {p.name[0]}
            </span>
            <div className="player-info">
              <strong>
                {p.name}
                {p.left && (
                  <small>
                    {g.splendor?.players[i].eliminated ||
                    g.rail?.players[i].eliminated
                      ? "超时离场"
                      : "已离桌"}
                  </small>
                )}
                {i === room.you && <small>你</small>}
              </strong>
              {g.splendor ? (
                <div className="mini-resources">
                  {g.splendor.players[i].bonus.map((n, c) => (
                    <span key={c} title={`${gemNames[c]}永久折扣`}>
                      <Gemstone color={c} size={14} />
                      {n}
                    </span>
                  ))}
                  <span title="预留卡数量">
                    ▤{" "}
                    {g.splendor.players[i].reserved?.length ??
                      g.splendor.players[i].reservedCount}
                  </span>
                </div>
              ) : (
                <small>
                  车厢 {g.rail!.players[i].trains} · 手牌{" "}
                  {g.rail!.players[i].handCount} · 任务{" "}
                  {g.rail!.players[i].ticketCount}
                </small>
              )}
              {g.splendor && (
                <div className="mini-resources tokens-mini">
                  {g.splendor.players[i].tokens.map((n, c) => (
                    <span key={c} title={`${gemNames[c]}筹码`}>
                      <i style={{ background: gemColors[c] }} />
                      {n}
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
    </div>
  );
}
function Turn({
  room,
  serverNow,
  receivedAt,
  busy,
  command,
}: {
  room: Room;
  serverNow: number;
  receivedAt: number;
  busy: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
}) {
  const g = room.game!;
  const setup = !!g.rail?.setup;
  const mine = setup ? !g.rail?.setupReady?.[room.you] : g.turn === room.you;
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
    turn: "选择一个行动",
    discard: "归还多出的宝石",
    noble: "选择一位贵族",
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
          : setup
            ? mine
              ? "一起选择目的地"
              : "等待其他人选好"
            : mine
              ? "轮到你了"
              : `${room.seats[g.turn]?.name} 的回合`}
      </h3>
      <p>
        {g.finished
          ? "感谢同桌，好局下次再来。"
          : setup
            ? "所有人同时选牌，超时自动保留前两张。"
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
                ? setup
                  ? "正在自动选牌"
                  : "已超时"
                : setup
                  ? "共同选牌限时"
                  : "每回合 120 秒"}
            </span>
          </div>
          {expired && !setup && (
            <p>
              {mine
                ? "你已超时，其他玩家可以将你移出。尚未被移出前仍可行动。"
                : "该玩家已超时。可以继续等候，或将其移出后继续对局。"}
            </p>
          )}
          {expired && !mine && !setup && (
            <button
              className="timeout-kick"
              disabled={busy}
              onClick={() => {
                const target = room.seats[g.turn];
                if (
                  confirm(
                    `将超时玩家「${target.name}」移出本局？${g.splendor ? "其筹码归还供应区，预留卡洗回牌堆。" : "其列车牌归还弃牌堆，已铺铁路保留。"}剩余玩家继续，若仅剩一人则获胜。`,
                  )
                ) {
                  command("kick_timeout", { target: target.id });
                }
              }}
            >
              移出超时玩家
            </button>
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
          {g.splendor
            ? g.splendor.players.filter((p) => !p.eliminated).length === 1
              ? "其他玩家已超时离场，最后留在牌桌的玩家获胜。"
              : "达到 15 分后完成本轮；同分时，发展卡更少者获胜。超时离场的玩家不参与排名。"
            : "总分 = 路线分 + 目的地净得分 + 最长路线奖励。"}
        </p>
      </div>
      {g.rail && (
        <div className="score-table">
          {room.seats.map((p, i) => {
            const x = g.rail!.players[i];
            return (
              <div key={p.id}>
                <strong>{p.name}</strong>
                <span>路线 {x.routeScore}</span>
                <span>
                  任务 {x.ticketScore > 0 ? "+" : ""}
                  {x.ticketScore}
                </span>
                <span>
                  最长 {x.longest} 节 / +{x.bonus}
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
}: {
  card: Card;
  onClick?: () => void;
  selected?: boolean;
}) {
  return (
    <button
      className={`dev-card gem-${card.color} ${selected ? "selected" : ""}`}
      onClick={onClick}
      aria-label={`${gemNames[card.color]}发展卡，${card.points}分，费用 ${card.cost
        .map((n, i) => (n ? `${gemNames[i]}${n}` : ""))
        .filter(Boolean)
        .join("，")}`}
      style={
        {
          "--card-color": gemColors[card.color],
          "--art-x": `${((card.tier - 1) * 2 + (card.id % 2)) * 20}%`,
          "--art-y": `${[3, 4, 0, 1, 2][card.color] * 20}%`,
        } as React.CSSProperties
      }
    >
      <div className="card-top">
        <strong>{card.points || ""}</strong>
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
  return (
    <button
      className={`noble ${eligible ? "eligible" : ""}`}
      style={{
        backgroundPosition: `${((noble.id - 1) % 5) * 25}% ${Math.floor((noble.id - 1) / 5) * 50}%`,
      }}
      onClick={onClick}
      aria-label={`贵族 ${noble.id}，3 分，需要 ${noble.cost
        .map((n, i) => (n ? `${gemNames[i]}折扣${n}` : ""))
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
  const g = room.game!,
    s = g.splendor!,
    p = s.players[room.you],
    mine = g.turn === room.you && !g.finished;
  const [tokens, setTokens] = useState<number[]>(Array(6).fill(0));
  const [selected, setSelected] = useState<Card>();
  const [collection, setCollection] = useState(false);
  const [payment, setPayment] = useState<number[]>(Array(6).fill(0));
  const selectCard = (card: Card) => {
    const spend = card.cost.map((n, i) =>
      Math.min(Math.max(0, n - p.bonus[i]), p.tokens[i]),
    );
    spend.push(
      card.cost.reduce(
        (total, n, i) => total + Math.max(0, n - p.bonus[i]) - spend[i],
        0,
      ),
    );
    setPayment(spend);
    setSelected(card);
  };
  useEffect(() => {
    setTokens(Array(6).fill(0));
    setSelected(undefined);
  }, [room.version]);
  const taking = mine && g.phase === "turn",
    discard = mine && g.phase === "discard";
  const pay = selected
    ? selected.cost.map((n, i) => Math.max(0, n - p.bonus[i]))
    : [];
  const gold = payment[5];
  const affordable = gold <= p.tokens[5];
  const reserved = selected && p.reserved?.some((c) => c.id === selected.id);
  return (
    <div className="splendor-board">
      <h2 className="sr-only">璀璨宝石游戏桌面</h2>
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
              eligible={n.cost.every((v, i) => v <= p.bonus[i])}
              onClick={() => {
                if (mine && g.phase === "noble")
                  void act({ type: "noble", noble: n.id });
              }}
            />
          ))}
        </div>
      </div>
      <div className="market">
        {[2, 1, 0].map((tier) => (
          <div className="market-row" key={tier}>
            <button
              className={`deck tier-${tier}`}
              style={{ backgroundPosition: `${tier * 20}% 100%` }}
              disabled={
                !taking || busy || !s.remaining[tier] || p.reserved!.length >= 3
              }
              onClick={() => void act({ type: "reserve", tier: tier + 1 })}
              aria-label={`从 ${tier + 1} 级牌堆预留一张牌`}
            >
              <span className="deck-ornament">✧</span>
              <strong>{["Ⅰ", "Ⅱ", "Ⅲ"][tier]}</strong>
              <small>{s.remaining[tier]} 张 · 盲预留</small>
            </button>
            {s.market[tier].map((c) => (
              <DevCard
                key={c.id}
                card={c}
                selected={selected?.id === c.id}
                onClick={() => selectCard(c)}
              />
            ))}
          </div>
        ))}
      </div>
      <div className="gem-bank">
        <div className="section-line">
          <h3>公共宝石</h3>
          <small>三种各一枚，或同色两枚（该堆至少四枚）</small>
        </div>
        <div className="token-row">
          {s.bank.map((n, i) => (
            <button
              key={i}
              className={`token-button ${tokens[i] > 0 ? "chosen" : ""}`}
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
          ))}
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
      <div className="personal-area">
        <div className="section-line">
          <h3>你的商会</h3>
          <button className="subtle" onClick={() => setCollection(!collection)}>
            发展卡 {p.cards.length} · 贵族 {p.nobles.length}
            <ChevronRight size={15} />
          </button>
        </div>
        <div className="personal-gems">
          {p.tokens.map((n, i) => (
            <div key={i}>
              <Gemstone color={i} />
              <strong>{n}</strong>
              <small>{i < 5 ? `永久 −${p.bonus[i]}` : "万能支付"}</small>
            </div>
          ))}
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
      {mine && g.phase === "noble" && (
        <div className="action-prompt">
          多位贵族青睐你的商会，请点击上方符合条件的一位。
        </div>
      )}
      {discard && (
        <Modal
          dismissible={false}
          title={`请归还 ${p.tokens.reduce((a, b) => a + b, 0) - 10} 枚宝石`}
          onClose={() => {}}
        >
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
        </Modal>
      )}
      {selected && (
        <Modal
          title={`${gemNames[selected.color]}发展卡 · ${selected.points} 分`}
          onClose={() => setSelected(undefined)}
        >
          <div className="purchase-preview">
            <DevCard card={selected} />
            <div>
              <p>永久提供 1 枚{gemNames[selected.color]}折扣</p>
              <small>扣除永久折扣后的费用</small>
              <Cost cost={pay} />
              <p className="muted small">
                {gold ? `需要使用 ${gold} 枚黄金补足` : "无需使用黄金"}
              </p>
            </div>
          </div>
          {pay.some((n) => n > 0) && (
            <div className="payment-grid">
              {pay.map(
                (need, i) =>
                  need > 0 && (
                    <label key={i}>
                      {gemNames[i]} · 需要 {need}
                      <select
                        aria-label={`支付${gemNames[i]}数量`}
                        value={payment[i]}
                        onChange={(e) => {
                          const next = [...payment];
                          next[i] = Number(e.target.value);
                          next[5] = pay.reduce(
                            (total, n, c) => total + n - next[c],
                            0,
                          );
                          setPayment(next);
                        }}
                      >
                        {Array.from(
                          { length: Math.min(need, p.tokens[i]) + 1 },
                          (_, n) => (
                            <option key={n} value={n}>
                              {n} 枚{gemNames[i]} + {need - n} 枚黄金
                            </option>
                          ),
                        )}
                      </select>
                    </label>
                  ),
              )}
              <p className="muted small">
                本次支付黄金 {gold} 枚，你持有 {p.tokens[5]}{" "}
                枚。可主动使用黄金保留其他宝石。
              </p>
            </div>
          )}
          <div className="form-grid">
            <button
              className="primary"
              disabled={!taking || busy || !affordable}
              onClick={() =>
                void act({ type: "buy", card: selected.id, tokens: payment })
              }
            >
              {affordable ? "购买卡牌" : "宝石不足"}
            </button>
            {!reserved && (
              <button
                className="outline"
                disabled={!taking || busy || p.reserved!.length >= 3}
                onClick={() => void act({ type: "reserve", card: selected.id })}
              >
                预留{s.bank[5] > 0 ? " · 黄金 +1" : ""}
              </button>
            )}
          </div>
        </Modal>
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
  return (
    <button
      className={`train-card train-${color}`}
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
      <span>{trainNames[color]}</span>
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
}: {
  ticket: Ticket;
  catalog: Catalog;
  selected?: boolean;
  onClick?: () => void;
  showResult?: boolean;
}) {
  const assets = useContext(AssetsContext);
  const a = catalog.cities[ticket.a],
    b = catalog.cities[ticket.b];
  return (
    <button
      style={
        assets
          ? {
              backgroundImage: `url("${assets}/rail/tickets/${ticket.id}.webp")`,
            }
          : undefined
      }
      className={`ticket-card ${selected ? "selected" : ""} ${showResult ? (ticket.complete ? "completed" : "incomplete") : ""}`}
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
        <strong>{cityName(b.name)}</strong>
      </div>
      <small>
        {a.name} — {b.name}
      </small>
      <div className="ticket-bottom">
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
          {ticket.points}
          <small>分</small>
        </b>
      </div>
    </button>
  );
}
function RailMap({
  catalog,
  owners,
  selected,
  onSelect,
  highlight,
}: {
  catalog: Catalog;
  owners: Record<string, number>;
  selected?: Route;
  onSelect: (r: Route) => void;
  highlight?: Ticket;
}) {
  const [zoom, setZoom] = useState(1);
  const assets = useContext(AssetsContext);
  return (
    <div className="map-frame">
      <div className="map-controls">
        <span>UNITED STATES · 1900</span>
        <div>
          <button
            className="icon-button"
            aria-label="缩小地图"
            disabled={zoom <= 1}
            onClick={() => setZoom(Math.max(1, zoom - 0.25))}
          >
            <ZoomOut size={17} />
          </button>
          <span>{Math.round(zoom * 100)}%</span>
          <button
            className="icon-button"
            aria-label="放大地图"
            disabled={zoom >= 2}
            onClick={() => setZoom(Math.min(2, zoom + 0.25))}
          >
            <ZoomIn size={17} />
          </button>
        </div>
      </div>
      <div
        className="map-scroll"
        style={zoom > 1 ? { maxHeight: "75vh" } : undefined}
      >
        <svg
          className="rail-map"
          viewBox="0 0 1744 1125"
          style={
            {
              "--map-scale": zoom,
              width: `${zoom * 100}%`,
              height: zoom > 1 ? "auto" : undefined,
            } as React.CSSProperties
          }
          role="group"
          aria-label="美国铁路地图，点击路线选择占领"
        >
          <rect width="1744" height="1125" fill="#ece8d8" />
          {assets && (
            <image
              href={`${assets}/rail/map.webp`}
              width="1744"
              height="1125"
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
                    <rect
                      x="-26"
                      y="-8"
                      width="52"
                      height="16"
                      rx="3"
                      fill={
                        claimed
                          ? playerColors[owner]
                          : assets
                            ? "transparent"
                            : r.color < 0
                              ? "#bbb4a1"
                              : trainColors[r.color]
                      }
                      stroke={
                        active ? "#f5cf47" : claimed ? "#fff9e3" : "transparent"
                      }
                      strokeWidth="3"
                    />
                    {claimed && (
                      <path
                        d="M-20 0H20M-17 7v4M17 7v4"
                        stroke="white"
                        strokeWidth="2"
                      />
                    )}
                  </g>
                ))}
              </g>
            );
          })}
          {catalog.cities.map((c) => {
            const lit =
              highlight && (c.id === highlight.a || c.id === highlight.b);
            const [lx, ly, lw, lh] = c.label || [c.x + 12, c.y - 30, 100, 30];
            return (
              <g key={c.id} className="map-city">
                {lit && (
                  <circle
                    cx={c.x}
                    cy={c.y}
                    r="23"
                    fill="#f1b52f"
                    opacity=".7"
                  />
                )}
                <circle
                  cx={c.x}
                  cy={c.y}
                  r={lit ? 9 : 5}
                  fill={lit ? "#b53427" : "#f6f0dc"}
                  stroke="#4d503c"
                  strokeWidth="2"
                />
                <rect
                  x={lx}
                  y={ly}
                  width={lw}
                  height={lh}
                  rx="5"
                  fill="#f4f0e3"
                />
                <text
                  x={lx + lw / 2}
                  y={ly + lh / 2}
                  textAnchor="middle"
                  dominantBaseline="central"
                >
                  {cityName(c.name)}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
      <div className="map-caption">
        <span>点击路线查看费用 · 放大后可拖动滚动条浏览</span>
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
  const [highlight, setHighlight] = useState<Ticket>();
  const g = room.game!,
    r = g.rail!,
    p = r.players[room.you],
    mine =
      (r.setup ? !r.setupReady?.[room.you] : g.turn === room.you) &&
      !g.finished,
    turn = mine && !r.setup && g.phase === "turn",
    drawing = mine && !r.setup && (g.phase === "turn" || g.phase === "draw");
  useEffect(() => {
    api("/catalog")
      .then(setCatalog)
      .catch((e) => setLoadError(e.message));
  }, []);
  useEffect(() => {
    setSelected(undefined);
  }, [g.turn, g.phase, g.round]);
  useEffect(() => {
    setKeep([]);
  }, [r.pending?.map((t) => t.id).join(",")]);
  if (loadError)
    return <p role="alert">地图加载失败：{loadError}。请刷新重试。</p>;
  if (!catalog) return <div className="loading">正在展开地图…</div>;
  const city = (id: number) => cityName(catalog.cities[id].name);
  const selectRoute = (route: Route) => {
    setSelected(route);
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
        (room.seats.length < 4 || r.owners[x.id] === room.you),
    );
  return (
    <div className="rail-board">
      <h2 className="sr-only">铁路环游游戏桌面</h2>
      {mine && g.phase === "tickets" && (
        <section className="ticket-choice" aria-label="选择目的地任务">
          <h3>{r.setup ? "选择你的第一段旅程" : "新的目的地，新的机会"}</h3>
          <p>
            从这些任务中至少保留 <strong>{r.setup ? 2 : 1}</strong>{" "}
            张。未完成的任务会在结算时扣分。
            {r.setup && "所有人同时选择；120 秒后自动保留列表前两张。"}
          </p>
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
        </section>
      )}

      <div className="rail-table">
        <RailMap
          catalog={catalog}
          owners={r.owners}
          selected={selected}
          onSelect={selectRoute}
          highlight={highlight}
        />
        <div className="train-market">
          <div className="section-line">
            <h3>列车牌市场</h3>
            <small>
              {g.phase === "draw" && mine
                ? "还可以摸 1 张（不能拿公开万能牌）"
                : "每回合摸两张；公开万能牌占用整个行动"}
            </small>
          </div>
          <div className="train-market-cards">
            <button
              className="train-deck"
              disabled={!drawing || busy}
              onClick={() => void act({ type: "draw", slot: -1 })}
            >
              <TrainFront size={28} />
              <strong>摸一张暗牌</strong>
              <small>可摸 {r.remaining} 张</small>
            </button>
            {r.face.map((c, i) =>
              c < 0 ? (
                <div className="train-card empty" key={i}>
                  牌堆已空
                </div>
              ) : (
                <TrainCard
                  key={i}
                  color={c}
                  disabled={!drawing || busy || (g.phase === "draw" && c === 8)}
                  onClick={() => void act({ type: "draw", slot: i })}
                  label={`拿取公开${trainNames[c]}列车牌，第 ${i + 1} 张`}
                />
              ),
            )}
            <button
              className="ticket-deck"
              disabled={!turn || busy || r.ticketsRemaining === 0}
              onClick={() => void act({ type: "tickets" })}
            >
              <Flag size={25} />
              <strong>领取目的地</strong>
              <small>抽 3 留至少 1 · 余 {r.ticketsRemaining}</small>
            </button>
          </div>
        </div>
      </div>
      <div className="personal-area rail-personal">
        <div className="section-line">
          <h3>
            你的列车手牌 <span>{p.handCount}</span>
          </h3>
          <span className="tag">剩余车厢 {p.trains} / 45</span>
        </div>
        <div className="train-hand">
          {p.hand!.map((n, c) => (
            <TrainCard key={c} color={c} count={n} disabled={n === 0} />
          ))}
        </div>
        <div className="section-line ticket-heading">
          <h3>
            你的目的地 <span>{p.tickets?.length}</span>
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
            />
          ))}
        </div>
      </div>
      {g.finished && (
        <div className="personal-area">
          <h3>所有玩家的目的地结算</h3>
          {room.seats.map((seat, i) => (
            <details key={seat.id}>
              <summary>
                {seat.name} · 完成 {r.players[i].completed} 张 · 净得分{" "}
                {r.players[i].ticketScore}
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
            <span>铺设得 {[0, 1, 2, 4, 7, 10, 15][selected.length]} 分</span>
          </div>
          {r.owners[selected.id] !== undefined ? (
            <p>
              这条路线已由{" "}
              <strong>{room.seats[r.owners[selected.id]].name}</strong> 占领。
            </p>
          ) : parallelBlocked ? (
            <p>
              这条双线目前不可用。2–3
              人局只能使用其中一条，同一玩家也不能同时占领双线。
            </p>
          ) : (
            <>
              <label>
                支付颜色
                <select
                  value={color}
                  disabled={selected.color >= 0}
                  onChange={(e) => {
                    setColor(Number(e.target.value));
                    setWild(
                      Math.max(
                        0,
                        selected.length - p.hand![Number(e.target.value)],
                      ),
                    );
                  }}
                >
                  {trainNames.slice(0, 8).map((n, i) => (
                    <option value={i} key={n}>
                      {n} · 持有 {p.hand![i]} 张
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
                  {Array.from({ length: selected.length + 1 }, (_, i) => (
                    <option key={i} value={i}>
                      {i} 张万能 + {selected.length - i} 张{trainNames[color]}牌
                    </option>
                  ))}
                </select>
              </label>
              <p className="muted small">
                你的万能牌：{p.hand![8]} 张。占领后自动扣除所选牌和{" "}
                {selected.length} 节车厢。
              </p>
              <button
                className="primary wide"
                disabled={
                  !turn ||
                  busy ||
                  p.trains < selected.length ||
                  p.hand![color] < selected.length - wild ||
                  p.hand![8] < wild
                }
                onClick={() =>
                  void act({ type: "claim", route: selected.id, color, wild })
                }
              >
                铺设这条铁路
                <TrainFront size={18} />
              </button>
            </>
          )}
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
function Rules({ kind }: { kind: string }) {
  return (
    <div className="rules">
      {kind === "splendor" ? (
        <>
          <p>
            成为宝石商人，购买发展卡积累永久折扣和声望。2–4 人，完整基础版。
          </p>
          <ol>
            <li>
              <b>每回合选择一个行动：</b>
              拿取三种不同宝石各一枚；拿取同色两枚（供应至少四枚）；购买一张公开或预留的发展卡；预留一张公开或牌堆顶牌。
            </li>
            <li>
              <b>购买：</b>
              先扣除已购卡牌的永久折扣，再支付宝石，黄金可替代任意颜色，可自行调整支付组合。
            </li>
            <li>
              <b>预留：</b>
              最多三张，若供应中有黄金，获得一枚。预留卡对他人保密。
            </li>
            <li>
              <b>回合收尾：</b>
              持有超过十枚宝石时必须归还。满足贵族条件时获得一位贵族及三分，多位符合条件时自行选择。
            </li>
            <li>
              <b>结束：</b>
              任意玩家达到十五分后完成当前轮，让所有玩家行动次数相同。最高分获胜；同分时发展卡更少者获胜，仍相同则共同获胜。
            </li>
          </ol>
          <p>
            白色钻石、蓝色蓝宝石、绿色祖母绿、红色红宝石、黑色缟玛瑙，以及黄色黄金。
          </p>
        </>
      ) : (
        <>
          <p>美国基础版，2–5 人。收集列车牌，连接目的地，争夺最长连续铁路。</p>
          <ol>
            <li>
              <b>开局：</b>
              每人四张列车牌、四十五节车厢；所有人同时从三张目的地中至少保留两张，120
              秒后未提交者自动保留前两张。
            </li>
            <li>
              <b>每回合选择一个行动：</b>
              摸两张列车牌、占领一条路线、或抽取最多三张目的地任务（至少保留一张）。
            </li>
            <li>
              <b>摸牌：</b>
              可在公开牌和暗牌之间组合。拿公开万能牌时只能拿这一张；第二次不能拿公开万能牌。暗抽万能牌计为普通一次摸牌。
            </li>
            <li>
              <b>占领路线：</b>
              支付与长度相同、颜色一致的列车牌，可混用万能牌。灰线可选择任意一种颜色。2–3
              人局双线只用一条；同一人不能占领双线。
            </li>
            <li>
              <b>终局：</b>
              某玩家回合结束时只剩两节或更少车厢，每人（包括触发者）再行动一次。
            </li>
            <li>
              <b>计分：</b>路线长度 1–6 分别得 1、2、4、7、10、15
              分。完成目的地加分，未完成扣分。最长连续路线奖励十分，可重复经过城市，不可重复使用路段；并列均获奖励。
            </li>
            <li>
              <b>同分：</b>
              完成目的地更多者获胜；仍相同，获得最长路线奖励者获胜，再相同则共同获胜。
            </li>
          </ol>
          <p>
            公开牌出现至少三张万能牌时会自动换牌。牌量不足以形成正常市场时保留有限牌面；所有人均无合法行动时提前结算，避免死局。
          </p>
        </>
      )}
      <p>
        每回合 120
        秒，弃牌、贵族选择和第二次摸牌共用本回合计时。超时后同局其他玩家可移出当前玩家，剩余玩家继续，最后一人获胜。房主可直接结束牌桌。
      </p>
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
