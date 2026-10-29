import { CompactRoomPanel } from "./ui/compact-room-panel";
import { NetworkSignal } from "./ui/network-signal";
import type { NetworkInfo } from "./network-quality";
import { isTransientSyncError, quietSyncError } from "./game-api/sync-error";
import { blocksAutoplayAction } from "./game-api/autoplay-action";
import {
  GameFullscreen,
  type GameFullscreenHandle,
} from "./ui/room-fullscreen";
import { GameFeedback } from "./ui/game-overlay-root";
import { SoundControls } from "./ui/sound-controls";
import { readAudioPreferences } from "./audio-preferences";
import { Results } from "./ui/results";
import { RatingChanges } from "./ui/rating-changes";
import {
  closeTableDescription,
  cooperativeOutcomeLabel,
} from "./ui/cooperative-rating";
import { GameLoading } from "./game-api/loading";
import { Runtime } from "./game-api/runtime";
import { VersionUpdateNotice } from "./VersionUpdateNotice";
import { installControlContextMenuGuard } from "./control-context-menu";
import { revealGameOnEntry } from "./game-entry-scroll";
import { CategoryFilter, matchesCategory } from "./ui/game-category";
import { refreshQueue } from "./refresh-queue";
import {
  mergeStreamState,
  mergeStreamGame,
  mergeNetworkState,
  StreamInputs,
  type GamePacket,
} from "./realtime-stream";
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Bot,
  Check,
  Copy,
  Eye,
  Flag,
  Gem,
  Gamepad2,
  Hand,
  Lock,
  Maximize,
  LogOut,
  Plus,
  RotateCcw,
  ScrollText,
  ShieldCheck,
  Sparkles,
  TrainFront,
  Users,
  Wifi,
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
import { RailMapsContext, playerColors } from "../../games/rail/web/context";
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
import {
  gameModule,
  gameName,
  isIndependentGame,
  useGameRegistry,
} from "./game-api/registry";
import {
  LegacyCover,
  LegacyCreate,
  LegacyPlayers,
  LegacyRules,
  LegacyTurn,
  LegacyWaiting,
} from "./legacy/lazy-room-ui";
import { LogLine } from "./log-line";
import {
  Leaderboard,
  PlayerName,
  ProfileContext,
  ProfilePage,
} from "./profiles";
import "./room-option-cards.css";
import "./style.css";
import { useRoomPresentation } from "./game-api/presentation";
import { ResultOverlay } from "./ui/result-overlay";
import { RoomHeading } from "./ui/room-heading";
import { ClosedRoomDescription } from "./ui/closed-room";
import { CreateRoom } from "./ui/create-room";
import { ControllerSettings } from "./controllers/Settings";
import { useTurnTitle } from "./turn-title";
import type { Act, Room, State } from "./types";
import { AssetsContext, Modal, api } from "./ui/board-host";
import { WishDialog } from "./wishes";

const versionLabel = (version?: string) => {
  const value = (version || "dev").trim();
  return value.startsWith("v") ? value : `v${value}`;
};

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
  useGameRegistry();
  useEffect(() => installControlContextMenuGuard(document), []);
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
  const [streamInputs] = useState(() => new StreamInputs());
  const serverInstance = useRef<string | undefined>(undefined);
  const snapshotGeneration = useRef(0);
  const acceptState = useRef<(data: State) => void>(() => {});
  const [presentationEpoch, setPresentationEpoch] = useState(0);
  const resyncRequired = useRef(true);
  const presented = useRoomPresentation(
    state?.room,
    state?.serverNow ?? 0,
    state?.receivedAt ?? 0,
    presentationEpoch,
  );
  useTurnTitle(presented.room);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const networkFailure = useRef(false);
  const networkMessage = "网络连接中断，正在重新连接。";
  const reportError = (error: unknown) => {
    if (
      state?.room &&
      gameModule(state.room.kind)?.realtime &&
      isTransientSyncError(error)
    ) {
      networkFailure.current = true;
      return;
    }
    if (error instanceof TypeError) {
      networkFailure.current = true;
      setError(networkMessage);
    } else
      setError(error instanceof Error ? error.message : "操作失败，请重试。");
  };
  const [busy, setBusy] = useState(false);
  const [online, setOnline] = useState(false);
  const [connectionNetwork, setConnectionNetwork] = useState<NetworkInfo>();
  const [create, setCreate] = useState(false);
  useEffect(() => {
    if (create && state && !state.availableGames.length) setCreate(false);
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
  const [wishOpen, setWishOpen] = useState(false);
  const [controllerSettingsOpen, setControllerSettingsOpen] = useState(false);
  const gameFullscreen = useRef<GameFullscreenHandle>(null);
  const [gameFullscreenReady, setGameFullscreenReady] = useState(false);
  const [chatFullscreenTarget, setChatFullscreenTarget] =
    useState<HTMLElement | null>(null);
  useEffect(() => setControllerSettingsOpen(false), [state?.user.id]);
  const [sound, setSound] = useState(() => readAudioPreferences().enabled);
  const [volume, setVolume] = useState(() => readAudioPreferences().volume);
  const lastVolume = useRef(volume || 0.5);
  const [notice, setNotice] = useState("");
  const refreshWork = useRef<() => Promise<void>>(async () => {});
  const refresh = useMemo(() => refreshQueue(() => refreshWork.current()), []);
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
    audio.setVolume(volume);
    audio.mount();
    const visibility = () => {
      resyncRequired.current = true;
      setPresentationEpoch((v) => v + 1);
      if (document.hidden) audio.stop();
      else void refresh();
    };
    const storage = (event: StorageEvent) => {
      if (
        event.key !== "wb_sound" &&
        event.key !== "wb_volume" &&
        event.key !== null
      )
        return;
      const { enabled, volume: nextVolume } = readAudioPreferences();
      previewRequest.current++;
      audio.setEnabled(enabled);
      audio.setVolume(nextVolume);
      setSound(enabled);
      setVolume(nextVolume);
      if (nextVolume > 0) lastVolume.current = nextVolume;
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
      const surface = document.querySelector<HTMLElement>(".game-main");
      if (surface) return revealGameOnEntry(surface);
    }
  }, [state?.room?.id, state?.room?.status]);
  acceptState.current = (data: State) => {
    if (serverInstance.current !== data.serverInstance) {
      snapshotGeneration.current++;
      serverInstance.current = data.serverInstance;
      resyncRequired.current = true;
    }
    if (networkFailure.current) {
      networkFailure.current = false;
      setError((message) => (message === networkMessage ? "" : message));
    }
    if (
      previousRoom.current?.status === "playing" &&
      !previousRoom.current.spectating &&
      !data.room
    ) {
      setNotice("你已离开这张牌桌，可以创建或加入其他房间。");
    }
    previousRoom.current = data.room;
    if (resyncRequired.current || document.hidden) {
      setPresentationEpoch((v) => v + 1);
      resyncRequired.current = false;
    }
    setState((previous) => mergeStreamState(previous, data, performance.now()));
    setLoaded(true);
  };
  refreshWork.current = async () => {
    try {
      const generation = snapshotGeneration.current;
      const data = await api("/state");
      if (generation === snapshotGeneration.current) acceptState.current(data);
    } catch (e) {
      if ((e as { status: number }).status === 401) {
        previousRoom.current = undefined;
        setState(undefined);
      } else reportError(e);
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
    let opened = false;
    const connect = () => {
      const connection = new WebSocket(
        `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/ws?stream=1`,
      );
      socket = connection;
      socket.onopen = () => {
        if (stopped || socket !== connection) return;
        retryDelay = 250;
        setOnline(true);
        setConnectionNetwork(undefined);
        if (opened) resyncRequired.current = true;
        opened = true;
      };
      socket.onmessage = (event) => {
        if (stopped || socket !== connection) return;
        try {
          const packet = JSON.parse(event.data);
          if (packet.type === "state") {
            if (!streamInputs.connected) streamInputs.attach(socket);
            acceptState.current(packet.state);
          } else if (packet.type === "network") {
            if (
              packet.serverInstance === serverInstance.current &&
              packet.userId === state.user.id
            )
              setConnectionNetwork(packet.self);
            setState((previous) => mergeNetworkState(previous, packet));
          } else if (packet.type === "game") {
            setState((previous) =>
              mergeStreamGame(
                previous,
                packet as GamePacket,
                performance.now(),
              ),
            );
          } else if (packet.type === "ack") {
            streamInputs.ack(packet);
          } else {
            void refresh();
          }
        } catch {
          socket.close();
        }
      };
      socket.onclose = () => {
        if (stopped || socket !== connection) return;
        streamInputs.close();
        setOnline(false);
        setConnectionNetwork(undefined);
        resyncRequired.current = true;
        setPresentationEpoch((v) => v + 1);
        audio.stop();
        if (!stopped) {
          timer = setTimeout(connect, retryDelay + Math.random() * 150);
          retryDelay = Math.min(retryDelay * 2, 2000);
        }
      };
      socket.onerror = () => connection.close();
    };
    connect();
    const poll = setInterval(() => {
      if (!streamInputs.connected) void refresh();
    }, 15000);
    const focus = () => void refresh();
    window.addEventListener("focus", focus);
    return () => {
      stopped = true;
      clearTimeout(timer);
      clearInterval(poll);
      streamInputs.close();
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
    const fleetActor =
      twoResponder(r) ??
      caravanResponder(r) ??
      fishResponder(r) ??
      r.game!.catan?.cardEvent?.players[0] ??
      r.game!.catan?.citiesKnights?.pending?.players[0] ??
      r.game!.catan?.seafarers?.pirateIslands?.raid?.rewards[0];
    const key = `${r.id}:${r.game!.round}:${r.game!.turn}:${r.status}:${sg?.pending?.id || 0}:${sg?.pending?.canRespond || false}:${fleetActor ?? -1}:${r.game!.catan?.two?.sequence ?? 0}:${r.game!.catan?.two?.pending ? "two_build" : r.game!.catan?.two?.trade ? "two_trade" : ""}:${r.game!.catan?.caravans?.pending?.kind ?? ""}:${r.game!.catan?.caravans?.sequence ?? 0}:${caravanPlacementStep(r.game!.catan) ?? 0}:${r.game!.catan?.citiesKnights?.pending?.kind ?? ""}:${r.game!.catan?.cardEvent?.kind ?? ""}:${r.game!.catan?.cardEvent ? r.game!.catan.rollId : 0}`;
    if (previousTurn.current && key !== previousTurn.current && sound) {
      if ((r.game!.finished || r.status === "closed") && previousTurn.current) {
        void audio.play("finish");
      } else if (
        r.status === "playing" &&
        !r.seats[r.you]?.autoPlay &&
        (sg?.pending
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
      reportError(e);
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
    if (!state?.room || presented.blocked) return;
    if (
      blocksAutoplayAction(
        state.room.seats[state.room.you]?.autoPlay,
        gameModule(state.room.kind),
        action,
      )
    ) {
      setNotice("当前由电脑托管，取消托管后即可继续操作。");
      return;
    }
    if (gameModule(state.room.kind)?.realtime) {
      if (
        busy ||
        state.room.status !== "playing" ||
        state.room.spectating ||
        state.room.you < 0 ||
        state.room.seats[state.room.you]?.left
      )
        return;
      try {
        if (streamInputs.connected) {
          await streamInputs.send({
            serverInstance: state.serverInstance,
            roomId: state.room.id,
            version: state.room.version,
            nonce: crypto.randomUUID(),
            action,
          });
        } else {
          await command(state.room, "action", {
            action,
            serverInstance: state.serverInstance,
          });
          void refresh();
        }
      } catch (e) {
        const failure = quietSyncError(e);
        if (!isTransientSyncError(failure)) reportError(failure);
        void refresh();
        throw failure;
      }
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
    if (enabled && volume === 0) changeVolume(lastVolume.current);
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
  const changeVolume = (value: number) => {
    if (!Number.isFinite(value)) return;
    const next = Math.max(0, Math.min(1, value));
    audio.setVolume(next);
    setVolume(next);
    if (next > 0) {
      lastVolume.current = next;
      audio.setEnabled(true);
      setSound(true);
      void audio.unlock();
    }
    try {
      localStorage.setItem("wb_volume", String(next));
      if (next > 0) localStorage.setItem("wb_sound", "on");
    } catch {
      /* The current tab still follows the slider when storage is blocked. */
    }
  };
  const previewAudio = async () => {
    if (!audio.enabled) return "";
    const request = ++previewRequest.current;
    const played = await audio.play("preview");
    if (request !== previewRequest.current || !audio.enabled) return "";
    return played ? "已播放" : "无法播放，请检查站点声音权限";
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
  const room = presented.room;
  const compactPoker =
    !adminOpen &&
    !!room?.game &&
    (room.status === "playing" || room.status === "finished") &&
    gameModule(room.kind)?.tableLayout === "poker";
  const canAdmin =
    state.user.role === "admin" || state.user.role === "superadmin";
  const autoPlay = !!room?.seats[room.you]?.autoPlay;
  const canAutoPlay =
    room?.status === "playing" &&
    state.room?.status === "playing" &&
    !room.spectating &&
    room.you >= 0 &&
    !room.seats[room.you]?.left &&
    !room.game?.sanguosha?.players[room.you]?.dead &&
    !room.game?.eliminated?.includes(room.you) &&
    (!isIndependentGame(room.kind) || !!gameModule(room.kind)?.supportsBots);
  const boardBusy = busy || autoPlay || presented.blocked;
  return (
    <ProfileContext.Provider value={setProfileID}>
      <AssetsContext.Provider value={state.assetsBaseURL || ""}>
        <RailMapsContext.Provider value={state.railMaps || []}>
          <div
            data-table-layout={compactPoker ? "poker" : undefined}
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
            <CompactRoomPanel
              enabled={compactPoker}
              label="平台"
              name="platform"
            >
              <header className="topbar">
                <Logo />
                <nav>
                  <span className="nav-active">
                    {adminOpen ? "管理后台" : room ? "游戏牌桌" : "游戏大厅"}
                  </span>
                  <span className="nav-caption">
                    让相聚，多一局。
                    <em className="app-version">
                      {versionLabel(state.version)}
                    </em>
                  </span>
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
                    className="subtle wish-nav-button"
                    aria-label="打开游戏许愿池"
                    onClick={() => setWishOpen(true)}
                  >
                    <Sparkles size={16} />
                    许愿池
                  </button>
                  <button
                    className="subtle"
                    onClick={() => setShowLeaderboard(true)}
                  >
                    积分榜
                  </button>
                  <NetworkSignal info={connectionNetwork} connected={online} />
                  <SoundControls
                    enabled={sound && volume > 0}
                    volume={volume}
                    onVolume={changeVolume}
                    onMute={toggleSound}
                    onPreview={previewAudio}
                  />
                  <button
                    className="icon-button"
                    aria-label="控制器设置"
                    title="控制器设置"
                    onClick={() => setControllerSettingsOpen(true)}
                  >
                    <Gamepad2 size={19} />
                  </button>
                  <button
                    className="profile-nav"
                    aria-label="我的战绩与好友"
                    title={`查看 ${state.user.name} 的玩家页面`}
                    onClick={() => setProfileID(state.user.id)}
                  >
                    <span className="avatar" aria-hidden="true">
                      {state.user.name[0]}
                    </span>
                    <span className="username">{state.user.name}</span>
                  </button>
                </div>
              </header>
            </CompactRoomPanel>
            {controllerSettingsOpen && (
              <ControllerSettings
                key={state.user.id}
                userId={state.user.id}
                gameId={room?.kind}
                availableGames={state.availableGames}
                onClose={() => setControllerSettingsOpen(false)}
              />
            )}
            {error && (
              <GameFeedback>
                <div className="toast error" role="alert">
                  {error}
                  <button aria-label="关闭错误" onClick={() => setError("")}>
                    <X size={16} />
                  </button>
                </div>
              </GameFeedback>
            )}
            {notice && (
              <GameFeedback>
                <div className="toast" role="status">
                  {notice}
                  <button aria-label="关闭提示" onClick={() => setNotice("")}>
                    <X size={16} />
                  </button>
                </div>
              </GameFeedback>
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
                onCreate={() => setCreate(true)}
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
                data-table-layout={compactPoker ? "poker" : undefined}
                className={`room-page ${room.game ? "playing" : ""} game-${room.kind}`}
              >
                <RoomHeading
                  roomId={room.id}
                  finished={!!state.room?.game?.finished}
                >
                  <div>
                    <span className="eyebrow">
                      {gameName(room.kind)} <span> / </span> 牌桌{" "}
                      {room.id.toUpperCase()}
                    </span>
                    <h1>{room.name}</h1>
                  </div>
                  <CompactRoomPanel
                    enabled={compactPoker}
                    label="房间"
                    title={`房间 · ${room.name}`}
                    name="tools"
                  >
                    <div className="room-tools">
                      {room.game &&
                        (room.status === "playing" ||
                          room.status === "finished") && (
                          <button
                            className="subtle"
                            data-fullscreen-enter
                            data-fullscreen-toggle
                            aria-label="全屏"
                            disabled={!gameFullscreenReady}
                            onClick={() => gameFullscreen.current?.enter()}
                          >
                            <Maximize size={16} />
                            全屏
                          </button>
                        )}
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
                    </div>
                  </CompactRoomPanel>
                </RoomHeading>
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
                      {room.result?.ratingPolicy === "coop_campaign_v1"
                        ? cooperativeOutcomeLabel(room.result)
                        : room.closeReason === "unpublished"
                          ? "这款游戏已下架"
                          : room.closeReason === "admin"
                            ? "管理员已结束牌桌"
                            : room.closeReason === "inactive"
                              ? "牌桌已因闲置关闭"
                              : "这张牌桌已结束"}
                    </h2>
                    <ClosedRoomDescription room={room} />
                    {room.result?.rated && (
                      <RatingChanges result={room.result} />
                    )}
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
                    <GameLoading
                      key={`${room.id}:${room.matchId ?? room.setupVersion}:${room.you}`}
                      room={room}
                      assets={state.assetsBaseURL || ""}
                    >
                      {(loadEvents, loading) => (
                        <>
                          {!room.game?.sanguosha && (
                            <CompactRoomPanel
                              enabled={compactPoker}
                              label="玩家"
                              name="players"
                            >
                              <Players room={room} />
                            </CompactRoomPanel>
                          )}
                          <div className="game-layout">
                            <div className="game-main" tabIndex={-1}>
                              <GameFullscreen
                                ref={gameFullscreen}
                                onReadyChange={setGameFullscreenReady}
                                onChatTargetChange={setChatFullscreenTarget}
                                scope={`${room.id}:${room.matchId ?? room.setupVersion}`}
                                enabled={!loading && !!room.game}
                                status={
                                  room.turnDeadline ? (
                                    <Turn
                                      room={room}
                                      serverNow={state.serverNow}
                                      receivedAt={state.receivedAt}
                                    />
                                  ) : undefined
                                }
                                result={
                                  room.game?.finished && (
                                    <ResultOverlay
                                      key={`${room.id}:${room.result?.id || "finished"}`}
                                    >
                                      <Results
                                        room={room}
                                        host={
                                          !room.spectating &&
                                          room.host === state.user.id &&
                                          state.availableGames.includes(
                                            room.kind,
                                          )
                                        }
                                        busy={busy}
                                        command={roomCommand}
                                      />
                                    </ResultOverlay>
                                  )
                                }
                              >
                                <GameBoard
                                  key={`${room.id}:${state.serverInstance ?? ""}`}
                                  room={room}
                                  loadEvents={loadEvents}
                                  act={act}
                                  busy={boardBusy || loading}
                                  assets={state.assetsBaseURL || ""}
                                  sound={sound && volume > 0 && !loading}
                                  audio={gameAudio}
                                  inputWindow={streamInputs.connected ? 8 : 1}
                                  clock={presented.clock}
                                  command={(type, extra) =>
                                    run(() => command(room, type, extra))
                                  }
                                />
                              </GameFullscreen>
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
                    </GameLoading>
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
                availableGames={state.availableGames}
                busy={busy}
                onClose={() => setCreate(false)}
                onSubmit={(body) =>
                  void run(async () => {
                    await api("/rooms", body);
                    setCreate(false);
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
                  <p>{closeTableDescription(room)}</p>
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
            {wishOpen && (
              <WishDialog
                onClose={() => setWishOpen(false)}
                onChanged={refresh}
              />
            )}
            {profileID && (
              <GameFeedback>
                <Modal title="玩家页面" onClose={() => setProfileID("")}>
                  {profileID === state.user.id && (
                    <div className="profile-account-actions">
                      <button
                        className="outline"
                        aria-label="退出登录"
                        disabled={busy}
                        onClick={() =>
                          void run(async () => {
                            await api("/logout", {});
                            setProfileID("");
                          })
                        }
                      >
                        <LogOut size={17} />
                        退出登录
                      </button>
                    </div>
                  )}
                  <ProfilePage
                    key={profileID}
                    id={profileID}
                    self={state.user.id}
                  />
                </Modal>
              </GameFeedback>
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
                fullscreenTarget={chatFullscreenTarget}
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
            <span>联机游戏 · 好友相聚 · 无限好时光</span>
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
              ? "使用朋友分享的邀请码，加入这个私人游戏大厅。"
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
                placeholder="你的游戏昵称"
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
              {busy ? "请稍候…" : register ? "创建账号，入座" : "进入游戏大厅"}
              <ArrowRight size={18} />
            </button>
          </form>
          <div className="auth-note">
            <Lock size={14} />
            只属于你和朋友的游戏空间
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
  const [roomQuery, setRoomQuery] = useState("");
  const [roomStatus, setRoomStatus] = useState("all");
  const [category, setCategory] = useState("all");
  const categoryGames = state.availableGames.filter((kind) =>
    matchesCategory(kind, category),
  );
  const query = roomQuery.trim().toLocaleLowerCase();
  const filtered =
    filter !== "all" || category !== "all" || roomStatus !== "all" || !!query;
  const resetFilters = () => {
    setFilter("all");
    setCategory("all");
    setRoomStatus("all");
    setRoomQuery("");
  };
  const createKind =
    filter !== "all" ? filter : (categoryGames[0] ?? state.availableGames[0]);
  useEffect(() => {
    if (filter !== "all" && !state.availableGames.includes(filter))
      setFilter("all");
  }, [state.availableGames, filter]);
  const rooms = state.rooms
    .filter(
      (r) =>
        (r.status === "waiting" || r.status === "playing") &&
        (filter === "all" || r.kind === filter) &&
        matchesCategory(r.kind, category) &&
        (roomStatus === "all" || r.status === roomStatus) &&
        `${r.name} ${r.id} ${gameName(r.kind)}`
          .toLocaleLowerCase()
          .includes(query),
    )
    .sort((a, b) => b.updated - a.updated);
  const invited = new URLSearchParams(location.search).get("room");
  return (
    <main className="lobby">
      <section className="room-list">
        <div className="section-heading">
          <div>
            <h1>游戏大厅</h1>
            <p>找个房间，一起开局。</p>
          </div>
          <button
            className="primary lobby-create"
            disabled={busy || !createKind}
            onClick={() => onCreate(createKind)}
          >
            <Plus size={17} />
            创建房间
          </button>
        </div>
        <div className="room-filters">
          <label className="game-search room-search">
            搜索房间
            <input
              type="search"
              aria-label="搜索房间"
              placeholder="房间名、房间号或游戏"
              value={roomQuery}
              onChange={(event) => setRoomQuery(event.target.value)}
            />
          </label>
          <CategoryFilter
            value={category}
            onChange={(value) => {
              setCategory(value);
              if (!matchesCategory(filter, value)) setFilter("all");
            }}
          />
          <label>
            游戏
            <select
              aria-label="按游戏筛选牌桌"
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
            >
              <option value="all">全部游戏</option>
              {categoryGames.map((kind) => (
                <option key={kind} value={kind}>
                  {gameName(kind)}
                </option>
              ))}
            </select>
          </label>
          <label>
            状态
            <select
              aria-label="房间状态"
              value={roomStatus}
              onChange={(event) => setRoomStatus(event.target.value)}
            >
              <option value="all">全部状态</option>
              <option value="waiting">等待入座</option>
              <option value="playing">游戏中</option>
            </select>
          </label>
        </div>
        <div className="room-results">
          <span role="status">{rooms.length} 个房间</span>
          {filtered && (
            <button className="room-reset" onClick={resetFilters}>
              清除筛选
            </button>
          )}
          <span className="live-dot">实时更新</span>
        </div>
        {rooms.length ? (
          <div className="room-table">
            <div className="table-label">
              <span>房间 / 游戏</span>
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
                  <span className="room-cover" aria-hidden="true">
                    <Cover kind={r.kind} mini />
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
              {filtered ? "没有符合条件的房间" : "还没有房间，来开第一局吧"}
            </h3>
            <p>
              {!state.availableGames.length
                ? "游戏正在准备中，请稍后再来看看。"
                : filtered
                  ? "试试其他筛选条件，或创建一个新房间。"
                  : "点击「创建房间」选择游戏，邀请朋友一起玩。"}
            </p>
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
        <small className="lobby-version">{versionLabel(state.version)}</small>
      </div>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
    <VersionUpdateNotice />
  </React.StrictMode>,
);

function Cover(props: Parameters<typeof LegacyCover>[0]) {
  return isIndependentGame(props.kind) ? (
    <ModuleCover {...props} />
  ) : (
    <LegacyCover {...props} />
  );
}

function Create(props: {
  availableGames: string[];
  busy: boolean;
  onClose: () => void;
  onSubmit: (body: unknown) => void;
}) {
  return (
    <CreateRoom
      availableGames={props.availableGames}
      busy={props.busy}
      onClose={props.onClose}
      onSubmit={props.onSubmit}
      renderCover={(kind) => <Cover kind={kind} mini />}
      renderForm={(form) =>
        isIndependentGame(form.kind) ? (
          <Runtime kind={form.kind}>
            <ModuleCreate {...form} />
          </Runtime>
        ) : (
          <LegacyCreate {...form} />
        )
      }
    />
  );
}

function Waiting(props: Parameters<typeof LegacyWaiting>[0]) {
  return isIndependentGame(props.room.kind) ? (
    <Runtime kind={props.room.kind}>
      <ModuleWaiting {...props} />
    </Runtime>
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
