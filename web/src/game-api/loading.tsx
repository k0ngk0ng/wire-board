import {
  Component,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import type { Room } from "../types";
import type { BoardProps } from "./module";
import { createPortal } from "react-dom";
import { api } from "../ui/board-host";
import { gameModule, gameName } from "./registry";
import { ModuleCover } from "./host";
import { prepareGame, type LoadProgress } from "./load-assets";
import "./loading.css";
import { revealGameOnEntry } from "../game-entry-scroll";
import { entryAutoScrollAllowed } from "./viewport-fit-core";
import { loadingClientID } from "./loading-session";
import { releaseResidentResources, sealResidentResources, watchResidentResources } from "./resident-assets";
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
  "onReady" | "onLoadError" | "onLoadProgress" | "loadSizes"
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
  const [loadSizes, setLoadSizes] = useState<Readonly<Record<string, number>>>(
    {},
  );
  const [target, setTarget] = useState<HTMLElement | null>(null);
  const latest = useRef(room);
  latest.current = room;
  const host = useRef<HTMLDivElement>(null),
    complete = useRef<() => void>(() => {}),
    alive = useRef(false),
    failed = useRef(false);
  const token = room.loading?.token;
  const retained = useRef(false);
  const generation = useRef(token);
  if (token && generation.current !== token) {
    retained.current = !!room.supportsCheckpoint && !!host.current?.querySelector("[data-room-fullscreen]");
    generation.current = token;
  }
  const identity = `${room.id}:${room.matchId ?? room.setupVersion}:${room.you}:${generation.current ?? ""}`;
  const surfaceIdentity = useRef(identity);
  if (!retained.current) surfaceIdentity.current = identity;
  const onReady = useCallback(() => complete.current(), []);
  const onLoadError = useCallback((message: string) => {
    if (alive.current) {
      failed.current = true;
      setError(message);
    }
  }, []);
  const onLoadProgress = useCallback(
    (
      percent: number,
      label: string,
      bytes?: { loaded: number; total: number },
    ) => {
      if (alive.current)
        setProgress((previous) => ({
          ...previous,
          percent:
            bytes || gameModule(latest.current.kind)?.assetLoading === "board"
              ? 35 + Math.min(100, Math.max(0, percent)) * 0.64
              : 90 + Math.min(100, Math.max(0, percent)) * 0.09,
          label,

        }));
    },
    [],
  );
  useEffect(() => {
    const abort = new AbortController();
    const lease = crypto.randomUUID();
    const unwatch = watchResidentResources(lease, bytes => {
      if (!abort.signal.aborted) setProgress(previous => ({ ...previous, ...bytes }));
    });
    alive.current = true;
    failed.current = false;
    if (!retained.current) setMounted(false);
    setReady(false);
    setError("");
    setProgress({ percent: 0, label: "准备加载" });
    const initial = latest.current,
      gate = initial.loading,
      client = loadingClientID();
    let done = false,
      build = "",
      assetsReady = false,
      boardReady = false;
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
      boardReady = true;
      if (!assetsReady) return;
      if (done || failed.current || abort.signal.aborted) return;
      done = true;
      void (async () => {
        const imgs = [...(host.current?.querySelectorAll("img") ?? [])];
        await Promise.all(
          imgs.map(async (img) => {
            if (img.currentSrc || img.src) {
              try {
                img.loading = "eager";
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
        await sealResidentResources(lease, abort.signal);
        if (failed.current) return;
        if (gate && !initial.spectating && initial.you >= 0)
          await send("load_ready");
        if (abort.signal.aborted) return;
        setReady(true);
        setProgress((previous) => ({
          ...previous,
          percent: 100,
          label: "本机加载完成",
        }));
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
        (incoming) => setProgress((previous) => ({ ...previous, percent: incoming.percent, label: incoming.label })),
        lease,
      );
      build = result.build;
      setLoadSizes(result.media);
      abort.signal.throwIfAborted();
      assetsReady = true;
      setMounted(true);
      if (boardReady) complete.current();
    })().catch((e) => {
      if (!abort.signal.aborted) setError(e.message);
    });
    return () => {
      alive.current = false;
      abort.abort();
      unwatch();
      void releaseResidentResources(lease);
      complete.current = () => {};
    };
  }, [identity, attempt, assets]);
  const asyncBoard = gameModule(room.kind)?.asyncBoard;
  useEffect(() => {
    if (mounted && !asyncBoard) onReady();
  }, [mounted, asyncBoard, onReady, identity]);
  const blocked = !ready || !!room.loading || !!error;
  useLayoutEffect(() => {
    const surface = host.current;
    const update = () => setTarget(surface?.querySelector<HTMLElement>("[data-room-fullscreen]") ?? null);
    update();
    if (!surface) return;
    const observer = new MutationObserver(update);
    observer.observe(surface, {
      attributes: true,
      attributeFilter: ["data-room-fullscreen"],
      childList: true,
      subtree: true,
    });
    return () => observer.disconnect();
  }, [mounted, identity, attempt]);
  useEffect(() => {
    if (
      !blocked &&
      host.current &&
      entryAutoScrollAllowed(gameModule(room.kind))
    ) {
      const surface =
        host.current.querySelector<HTMLElement>(".game-main") ?? host.current;
      return revealGameOnEntry(surface);
    }
  }, [blocked]);
  const cover = blocked && (
        <section
          className={`game-loading${target ? " game-loading-fullscreen" : ""}`}
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
            {progress.total !== undefined && (
              <small
                data-loaded-bytes={progress.loaded}
                data-total-bytes={progress.total}
              >
                {(
                  progress.loaded! /
                  (progress.total >= 1048576 ? 1048576 : 1024)
                ).toFixed(1)}{" "}
                /{" "}
                {(
                  progress.total / (progress.total >= 1048576 ? 1048576 : 1024)
                ).toFixed(1)}{" "}
                {progress.total >= 1048576 ? "MB" : "KB"}
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
      );
  return (
    <>
      {target ? createPortal(cover, target) : cover}
      {mounted && (
        <div
          ref={host}
          className={`game-ready-surface${blocked && !retained.current && !target ? " game-loading-staging" : ""}`}
          inert={blocked && !retained.current && !target || undefined}
          aria-hidden={blocked && !retained.current && !target || undefined}
          key={`${surfaceIdentity.current}:${attempt}`}
        >
          <LoadBoundary failed={onLoadError}>
            {children(
              { onReady, onLoadError, onLoadProgress, loadSizes },
              blocked,
            )}
          </LoadBoundary>
        </div>
      )}
    </>
  );
}
