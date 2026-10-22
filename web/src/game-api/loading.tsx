import {
  Component,
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type { Room } from "../types";
import type { BoardProps } from "./module";
import { api } from "../ui/board-host";
import { gameModule, gameName } from "./registry";
import { ModuleCover } from "./host";
import { prepareGame, type LoadProgress } from "./load-assets";
import "./loading.css";
import { revealGameOnEntry } from "../game-entry-scroll";
class LoadBoundary extends Component<
  { children: ReactNode; failed: (message: string) => void },
  { error: boolean }
> {
  state = { error: false };
  static getDerivedStateFromError() {
    return { error: true };
  }
  componentDidCatch(error: Error) {
    this.props.failed(error.message || "游戏画面初始化失败");
  }
  render() {
    return this.state.error ? null : this.props.children;
  }
}
type LoadEvents = Pick<
  BoardProps,
  "onReady" | "onLoadError" | "onLoadProgress"
>;
export function GameLoading({
  room,
  assets,
  children,
}: {
  room: Room;
  assets: string;
  children: (events: LoadEvents, blocked: boolean) => ReactNode;
}) {
  const [attempt, retry] = useState(0),
    [mounted, setMounted] = useState(false),
    [ready, setReady] = useState(false),
    [error, setError] = useState("");
  const [progress, setProgress] = useState<LoadProgress>({
    percent: 0,
    label: "准备加载",
  });
  const latest = useRef(room);
  latest.current = room;
  const host = useRef<HTMLDivElement>(null),
    complete = useRef<() => void>(() => {}),
    alive = useRef(false),
    failed = useRef(false);
  const token = room.loading?.token;
  // A completed gate disappearing must not recreate this local load session.
  const generation = useRef(token);
  if (token && generation.current !== token) generation.current = token;
  const identity = `${room.id}:${room.matchId ?? room.setupVersion}:${room.you}:${generation.current ?? ""}`;
  const onReady = useCallback(() => complete.current(), []);
  const onLoadError = useCallback((message: string) => {
    if (alive.current) {
      failed.current = true;
      setError(message);
    }
  }, []);
  const onLoadProgress = useCallback((percent: number, label: string) => {
    if (alive.current)
      setProgress({
        percent: 90 + Math.min(100, Math.max(0, percent)) * 0.09,
        label,
      });
  }, []);
  useEffect(() => {
    const abort = new AbortController();
    alive.current = true;
    failed.current = false;
    setMounted(false);
    setReady(false);
    setError("");
    const initial = latest.current,
      gate = initial.loading,
      client = crypto.randomUUID();
    let done = false,
      build = "";
    async function send(type: string) {
      const r = latest.current;
      return api("/rooms/" + r.id, {
        type,
        version: r.version,
        nonce: crypto.randomUUID(),
        loadToken: gate?.token,
        loadBuild: build || gate?.build,
        loadClient: client,
      });
    }
    complete.current = () => {
      if (done || failed.current || abort.signal.aborted) return;
      done = true;
      void (async () => {
        // Decode rendered HTML images too (legacy card art is discovered from DOM).
        const imgs = [...(host.current?.querySelectorAll("img") ?? [])];
        await Promise.all(
          imgs.map(async (img) => {
            if (img.currentSrc || img.src) {
              try {
                await img.decode();
              } catch {
                if (!img.complete || !img.naturalWidth)
                  throw Error("图片未能加载，请重试");
              }
            }
          }),
        );
        await new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        );
        abort.signal.throwIfAborted();
        if (failed.current) return;
        if (gate && !initial.spectating && initial.you >= 0)
          await send("load_ready");
        if (abort.signal.aborted) return;
        setReady(true);
        setProgress({ percent: 100, label: "本机加载完成" });
      })().catch((e) => {
        if (!abort.signal.aborted) setError(e.message);
      });
    };
    void (async () => {
      if (gate && !initial.spectating && initial.you >= 0)
        await send("load_begin");
      abort.signal.throwIfAborted();
      const result = await prepareGame(
        initial,
        assets,
        abort.signal,
        setProgress,
      );
      build = result.build;
      abort.signal.throwIfAborted();
      setMounted(true);
    })().catch((e) => {
      if (!abort.signal.aborted) setError(e.message);
    });
    return () => {
      alive.current = false;
      abort.abort();
      complete.current = () => {};
    };
  }, [identity, attempt, assets]);
  const asyncBoard = gameModule(room.kind)?.asyncBoard;
  useEffect(() => {
    if (mounted && !asyncBoard) onReady();
  }, [mounted, asyncBoard, onReady]);
  const blocked = !ready || !!room.loading || !!error;
  useEffect(() => {
    if (!blocked && host.current) {
      const surface =
        host.current.querySelector<HTMLElement>(".game-main") ?? host.current;
      return revealGameOnEntry(surface);
    }
  }, [blocked]);
  return (
    <>
      {blocked && (
        <section
          className="game-loading"
          data-game-loading={room.kind}
          aria-label={`${gameName(room.kind)}加载`}
        >
          <div className="game-loading-cover">
            <ModuleCover kind={room.kind} />
          </div>
          <div className="game-loading-content">
            <h2>{gameName(room.kind)}</h2>
            <p role="status">
              {error
                ? "加载未完成"
                : ready
                  ? "等待其他玩家加载完成"
                  : progress.label}
            </p>
            <progress
              aria-label="游戏加载进度"
              max={100}
              value={Math.floor(progress.percent)}
            />
            <strong>{Math.floor(progress.percent)}%</strong>
            {progress.total !== undefined && !ready && (
              <small>
                {(progress.loaded! / 1048576).toFixed(1)} /{" "}
                {(progress.total / 1048576).toFixed(1)} MB
              </small>
            )}
            {error && (
              <div role="alert">
                <p>{error}</p>
                <button className="primary" onClick={() => retry((n) => n + 1)}>
                  重试加载
                </button>{" "}
                <button className="outline" onClick={() => location.reload()}>
                  刷新页面
                </button>
              </div>
            )}
            {room.loading && (
              <>
                <p>所有玩家加载完成后开始，等待期间不计时。</p>
                <ul>
                  {room.seats
                    .filter((s) => !s.left)
                    .map((s) => {
                      const i = room.seats.indexOf(s);
                      return (
                        <li key={s.id}>
                          <span>{s.name}</span>
                          <b>
                            {s.bot
                              ? "电脑已就绪"
                              : room.loading?.ready[i]
                                ? "已加载"
                                : "加载中"}
                          </b>
                        </li>
                      );
                    })}
                </ul>
              </>
            )}
          </div>
        </section>
      )}
      {mounted && (
        <div
          ref={host}
          className={blocked ? "game-loading-staging" : undefined}
          inert={blocked || undefined}
          aria-hidden={blocked || undefined}
          key={`${identity}:${attempt}`}
        >
          <LoadBoundary failed={onLoadError}>
            {children({ onReady, onLoadError, onLoadProgress }, blocked)}
          </LoadBoundary>
        </div>
      )}
    </>
  );
}
