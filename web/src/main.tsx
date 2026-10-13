import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Bot,
  Check,
  Clock,
  Copy,
  Eye,
  Flag,
  Gem,
  Hand,
  Lock,
  LogOut,
  Plus,
  RotateCcw,
  ScrollText,
  ShieldCheck,
  TrainFront,
  Users,
  Volume2,
  VolumeX,
  Wifi,
  WifiOff,
  X,
} from "lucide-react";
import React, { useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  caravanPlacementStep,
  caravanResponder,
} from "../../games/catan/web/catan-caravans-state";
import { fishResponder } from "../../games/catan/web/catan-fishing-state";
import { catanSeatColor } from "../../games/catan/web/catan-player-colors";
import { twoResponder } from "../../games/catan/web/catan-two-state";
import { RailMapsContext, playerColors } from "../../games/rail/web/board";
import { railMapNames } from "../../games/rail/web/rail-expansions";
import "../../games/splendor/web/splendor-layout.css";
import { AdminDashboard } from "./admin";
import { GameAudio } from "./audio";
import "./autoplay.css";
import { Chat } from "./chat";
import {
  GameBoard,
  ModuleCover,
  ModuleCreate,
  ModulePlayers,
  ModuleRules,
  ModuleTurn,
  ModuleWaiting,
} from "./game-api/host";
import { gameModule, gameName, isIndependentGame } from "./game-api/registry";
import {
  LegacyCover,
  LegacyCreate,
  LegacyPlayers,
  LegacyRules,
  LegacyTurn,
  LegacyWaiting,
  Results,
} from "./legacy/room-ui";
import { LogLine } from "./log-line";
import {
  Leaderboard,
  PlayerName,
  ProfileContext,
  ProfilePage,
} from "./profiles";
import "./room-option-cards.css";
import "./style.css";
import { useTurnTitle } from "./turn-title";
import type { Act, Room, State } from "./types";
import { AssetsContext, Modal, api } from "./ui/board-host";

const statusName: Record<string, string> = {
  waiting: "等待入座",
  playing: "游戏中",
  finished: "已结算",
  closed: "已结束",
};

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
  const [audio] = useState(() => new GameAudio());
  const previewRequest = useRef(0);
  const gameAudio = useMemo(
    () => audio.channel(`room:${state?.room?.id || "lobby"}`),
    [audio, state?.room?.id],
  );
  useEffect(() => {
    audio.setEnabled(sound);
    audio.mount();
    const visibility = () => {
      if (document.hidden) audio.stop();
    };
    const storage = (event: StorageEvent) => {
      if (event.key !== "wb_sound" && event.key !== null) return;
      const enabled = event.key === null || event.newValue !== "off";
      previewRequest.current++;
      audio.setEnabled(enabled);
      setSound(enabled);
    };
    document.addEventListener("visibilitychange", visibility);
    window.addEventListener("storage", storage);
    return () => {
      document.removeEventListener("visibilitychange", visibility);
      window.removeEventListener("storage", storage);
      audio.dispose();
    };
  }, [audio]);
  useEffect(() => {
    audio.setEnabled(sound);
  }, [audio, sound]);
  useEffect(() => {
    audio.setAssetsBase(state?.assetsBaseURL || "");
  }, [audio, state?.assetsBaseURL]);
  useEffect(() => {
    previewRequest.current++;
    audio.stop();
  }, [audio, state?.room?.id, state?.user.id]);
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
    if (gameModule(r.kind)?.ownsSounds) {
      previousTurn.current = "";
      return;
    }
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
    if (previousTurn.current && key !== previousTurn.current && sound) {
      if ((r.game!.finished || r.status === "closed") && previousTurn.current) {
        void audio.play("finish");
      } else if (
        r.status === "playing" &&
        !r.seats[r.you]?.autoPlay &&
        (dota
          ? dota.actors?.includes(r.you)
          : sg?.pending
            ? sg.pending.canRespond
            : (fleetActor ?? r.game!.turn) === r.you)
      ) {
        void audio.play("turn");
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
      if (sound && !gameModule(state.room!.kind)?.ownsSounds)
        void audio.play(
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
    void audio.unlock();
  };
  const toggleSound = () => {
    const enabled = !audio.enabled;
    previewRequest.current++;
    audio.setEnabled(enabled);
    setSound(enabled);
    try {
      localStorage.setItem("wb_sound", enabled ? "on" : "off");
    } catch {
      /* Still works for this tab when storage is unavailable. */
    }
    if (enabled) void audio.unlock();
  };
  const previewAudio = async () => {
    if (!audio.enabled) return;
    const request = ++previewRequest.current;
    const played = await audio.play("preview");
    if (request !== previewRequest.current || !audio.enabled) return;
    setNotice(
      played
        ? "已播放试听音效，音量由设备媒体音量控制。"
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
    !room.game?.sanguosha?.players[room.you]?.dead &&
    !room.game?.eliminated?.includes(room.you) &&
    (!isIndependentGame(room.kind) || !!gameModule(room.kind)?.supportsBots);
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
                <div className="sound-controls" role="group" aria-label="声音">
                  <button
                    className="sound-preview"
                    disabled={!sound}
                    title={
                      sound ? "音量由设备媒体音量控制" : "取消静音后可试听"
                    }
                    onClick={() => void previewAudio()}
                  >
                    试听音效
                  </button>
                  <button
                    className="sound-toggle"
                    aria-label={sound ? "静音" : "取消静音"}
                    aria-pressed={!sound}
                    title={
                      sound ? "静音所有游戏和提示音" : "恢复所有游戏和提示音"
                    }
                    onClick={toggleSound}
                  >
                    {sound ? <Volume2 size={18} /> : <VolumeX size={18} />}
                    <span>{sound ? "静音" : "取消静音"}</span>
                  </button>
                </div>
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
                    {!room.spectating &&
                      (room.status !== "playing" ||
                        room.game?.eliminated?.includes(room.you)) && (
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
                        <GameBoard
                          room={room}
                          act={act}
                          busy={boardBusy}
                          assets={state.assetsBaseURL || ""}
                          sound={sound}
                          audio={gameAudio}
                          clock={{
                            serverNow: state.serverNow,
                            receivedAt: state.receivedAt,
                          }}
                          command={(type, extra) =>
                            run(() => command(room, type, extra))
                          }
                        />
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
        {state.availableGames.map((kind) =>
          isIndependentGame(kind) ? (
            <article className="game-feature" key={kind}>
              <ModuleCover kind={kind} />
              <div className="feature-info">
                <h2>{gameName(kind)}</h2>
                <p>{gameModule(kind)?.description}</p>
                <div className="feature-bottom">
                  <span>
                    {gameModule(kind)?.minPlayers}–
                    {gameModule(kind)?.maxPlayers} 人
                  </span>
                  <button className="primary" onClick={() => onCreate(kind)}>
                    创建牌桌
                  </button>
                </div>
              </div>
            </article>
          ) : (
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
          ),
        )}
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
            ...state.availableGames.map((kind) => [kind, gameName(kind)]),
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
                      {gameName(r.kind)}
                      {r.kind === "rail" &&
                        ` · ${railMapNames[r.railMap || "usa"]}`}{" "}
                      {r.kind === "sanguosha" &&
                        ` · ${r.sanguoshaOptions?.mode === "3v3" ? "3v3竞技" : r.sanguoshaOptions?.mode === "hegemony" ? "国战双将" : r.sanguoshaOptions?.deck === "military" ? "标准＋军争" : "身份局"}`}{" "}
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

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);

function Cover(props: Parameters<typeof LegacyCover>[0]) {
  return isIndependentGame(props.kind) ? (
    <ModuleCover {...props} />
  ) : (
    <LegacyCover {...props} />
  );
}

function Create(props: Parameters<typeof LegacyCreate>[0]) {
  const [kind, setKind] = useState(props.kind);
  return isIndependentGame(kind) ? (
    <ModuleCreate
      {...props}
      key={kind}
      kind={kind}
      onGameChange={setKind}
      renderCover={(id) => <Cover kind={id} mini />}
    />
  ) : (
    <LegacyCreate {...props} kind={kind} onGameChange={setKind} />
  );
}

function Waiting(props: Parameters<typeof LegacyWaiting>[0]) {
  return isIndependentGame(props.room.kind) ? (
    <ModuleWaiting {...props} />
  ) : (
    <LegacyWaiting {...props} />
  );
}

function Players(props: Parameters<typeof LegacyPlayers>[0]) {
  return isIndependentGame(props.room.kind) ? (
    <ModulePlayers {...props} />
  ) : (
    <LegacyPlayers {...props} />
  );
}

function Turn(props: Parameters<typeof LegacyTurn>[0]) {
  return isIndependentGame(props.room.kind) ? (
    <ModuleTurn {...props} />
  ) : (
    <LegacyTurn {...props} />
  );
}

function Rules(props: Parameters<typeof LegacyRules>[0]) {
  return isIndependentGame(props.kind) ? (
    <ModuleRules {...props} />
  ) : (
    <LegacyRules {...props} />
  );
}
