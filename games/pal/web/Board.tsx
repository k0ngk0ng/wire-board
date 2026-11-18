import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { api } from "../../../web/src/ui/board-host";
import { SaveGameControl } from "../../../web/src/ui/saved-games";
import { GameController } from "../../../web/src/controllers/game-controller";
import type {
  ControllerProfile,
  ControllerSettingsData,
} from "../../../web/src/controllers/input";
import { SceneRenderer, type SceneLoadProgress } from "./scene-renderer";
import { PalStage } from "./Stage";
import { PalControls } from "./Controls";
import { PalInput, palInputAllowed } from "./input";
import { LocalClient } from "./local-client";
import { FieldCorrection } from "./field-correction";
import { PalAudioAssets } from "./audio-assets";
import { PalAudioDirector, type PalAudioMemory } from "./audio";
import { endingPublication } from "./ending-assets";
import { visualPublication } from "./visual-assets";
import type { PalView } from "./view";
import "./board.css";

export function Board(props: BoardProps<PalView>) {
  const [localData, setLocalData] = useState<PalView>();
  const root = useRef<HTMLDivElement>(null),
    canvas = useRef<HTMLCanvasElement>(null),
    latest = useRef(props),
    authority = useRef(props);
  authority.current = props;
  const displayed = props.room.seats[props.room.you]?.autoPlay
    ? props.data
    : (localData ?? props.data);
  latest.current = {
    ...props,
    data: { ...displayed, controlAt: props.data.controlAt },
  };
  const renderer = useRef<SceneRenderer | undefined>(undefined),
    audioDirector = useRef<PalAudioDirector | undefined>(undefined),
    localClient = useRef<LocalClient | undefined>(undefined),
    correction = useRef(new FieldCorrection()),
    input = useRef<PalInput | undefined>(undefined),
    readyRef = useRef(false),
    paintNumber = useRef(0),
    paintQueued = useRef(false);
  const [profile, setProfile] = useState<ControllerProfile>(),
    [ready, setReady] = useState(false),
    [error, setError] = useState(""),
    [retry, setRetry] = useState(0),
    [audioVisibility, setAudioVisibility] = useState(0),
    [handover, setHandover] = useState(false);
  const [progress, setProgress] = useState<SceneLoadProgress>({
    percent: 0,
    label: "读取场景清单",
    loaded: 0,
    total: 0,
  });
  const user = props.room.seats[props.room.you]?.id ?? "spectator";
  const loadingGeneration = useRef(props.room.loading?.token ?? "");
  if (props.room.loading?.token)
    loadingGeneration.current = props.room.loading.token;
  const audioIdentity = `${props.room.id}:${props.room.matchId ?? ""}:${props.room.you}:${user}:${loadingGeneration.current}`;
  const audioMemory = useRef<PalAudioMemory & { identity: string }>({
    identity: audioIdentity,
    floor: 0,
  });
  if (audioMemory.current.identity !== audioIdentity)
    audioMemory.current = { identity: audioIdentity, floor: 0 };
  const transportPaused = props.inputWindow === 0;
  const battleAssetKey = latest.current.data.battleAssets?.join() ?? "";
  const running = () =>
    readyRef.current &&
    !error &&
    !handover &&
    !transportPaused &&
    document.hasFocus() &&
    !document.hidden &&
    !latest.current.room.loading &&
    latest.current.room.status === "playing" &&
    !latest.current.room.game?.paused &&
    !latest.current.room.seats[latest.current.room.you]?.autoPlay &&
    ![...document.querySelectorAll('[aria-modal="true"],dialog[open]')].some(
      (e) => e.getClientRects().length && !e.closest("[inert]"),
    );
  const enabled = () =>
    running() &&
    (!localClient.current || localClient.current.usable()) &&
    latest.current.data.local?.generation ===
      authority.current.data.local?.generation &&
    (latest.current.data.phase !== "battle" ||
      !latest.current.data.battleAssets?.length ||
      !!renderer.current?.battleReady(latest.current.data.battleAssets)) &&
    palInputAllowed(latest.current);
  const runningRef = useRef(running);
  runningRef.current = running;
  const enabledRef = useRef(enabled);
  enabledRef.current = enabled;
  const paint = async () => {
    if (paintQueued.current || !renderer.current || !canvas.current) return;
    paintQueued.current = true;
    try {
      let seen = -1;
      while (seen !== paintNumber.current) {
        seen = paintNumber.current;
        const { data: current } = latest.current,
          number = seen;
        await renderer.current.paint(
          canvas.current,
          current,
          () => number === paintNumber.current,
          correction.current.current,
        );
      }
    } finally {
      paintQueued.current = false;
    }
  };
  const redraw = () => {
    paintNumber.current++;
    if (readyRef.current)
      void paint().catch((e) => {
        setError(e instanceof Error ? e.message : "画面暂时无法同步");
      });
  };
  useEffect(() => {
    const abort = new AbortController();
    let alive = true,
      frame = 0,
      owned: PalInput | undefined,
      controller: GameController | undefined,
      audioAssets: PalAudioAssets | undefined,
      ownedAudio: PalAudioDirector | undefined,
      local: LocalClient | undefined,
      changedProfile: ControllerProfile | undefined;
    readyRef.current = false;
    setLocalData(undefined);
    setReady(false);
    setError("");
    setProfile(undefined);
    setHandover(false);
    const changed = (event: Event) => {
      const value = (event as CustomEvent).detail;
      if (value?.userId !== user || value.model !== "game:pal" || !alive)
        return;
      if (changedProfile && changedProfile.revision > value.profile.revision)
        return;
      changedProfile = value.profile;
      setProfile(value.profile);
      owned?.clear();
      controller?.configure(value.profile);
    };
    window.addEventListener("controller-settings-changed", changed);
    const parts = new Map<string, SceneLoadProgress>();
    const report = (part: string, value: SceneLoadProgress) => {
      if (!alive) return;
      parts.set(part, value);
      if (!parts.has("visual") || (audioAssets && !parts.has("audio"))) return;
      const loaded = [...parts.values()].reduce(
          (sum, item) => sum + item.loaded,
          0,
        ),
        total = [...parts.values()].reduce((sum, item) => sum + item.total, 0),
        combined = {
          percent: total ? (loaded / total) * 100 : 0,
          loaded,
          total,
          label: value.label,
        };
      setProgress(combined);
      latest.current.onLoadProgress?.(combined.percent, combined.label, {
        loaded,
        total,
      });
    };
    void (async () => {
      latest.current.onLoadProgress?.(0, "读取场景清单");
      const settings: ControllerSettingsData = await api(
        "/controllers?game=pal",
      );
      if (!alive) return;
      const model = settings.models.find((m) => m.id === "game:pal"),
        fetched = settings.profiles["game:pal"];
      const saved =
        changedProfile &&
        (!fetched || changedProfile.revision >= fetched.revision)
          ? changedProfile
          : fetched;
      if (!model || !saved) throw Error("无法读取仙剑控制器设置");
      setProfile(saved);
      owned = new PalInput(
        () => latest.current,
        () => enabledRef.current(),
        (message) => {
          if (alive) setError(message);
        },
        (action) => local?.submit(action),
        undefined,
        () => runningRef.current(),
      );
      input.current = owned;
      controller = new GameController(
        root.current!.closest<HTMLElement>("[data-fullscreen-board]") ??
          root.current!,
        saved,
        model,
        {
          enabled: () => enabledRef.current(),
          changed: () => owned?.sample(performance.now()),
          menu: () =>
            root.current?.querySelector<HTMLElement>("[data-pal-menu-panel]") ??
            null,
          menuActions: {
            up: "navUp",
            down: "navDown",
            left: "navLeft",
            right: "navRight",
          },
          keyboardTarget: (target) =>
            !(
              target instanceof Element &&
              target.closest("[data-pal-panel-action]")
            ),
        },
      );
      owned.controller = controller;
      owned.update();
      if (authority.current.data.local && authority.current.room.you === 0) {
        local = new LocalClient(
          authority.current.room.id,
          (frame, reason) => {
            if (!alive) return;
            if (reason === "reset") correction.current.clear();
            correction.current.update(
              frame,
              performance.now(),
              reason === "confirm",
            );
            latest.current = { ...latest.current, data: frame };
            setLocalData(frame);
            redraw();
          },
          (action) => owned?.commitLocal(action),
          (message) => {
            if (alive) setError(message);
          },
          async (frames) => {
            if (frames.length && renderer.current)
              await renderer.current.prepare(frames[0], frames.slice(1));
          },
        );
        localClient.current = local;
        local.accept(
          authority.current.data,
          authority.current.clock,
          authority.current.room.seats[authority.current.room.you]?.network
            ?.rttMs ?? 0,
        );
        await local.start();
        if (!alive) return;
      }
      const base = (latest.current.assets || "").replace(/\/$/, "") + "/pal/v1";
      const scene = new SceneRenderer(base, {
        signal: abort.signal,
        progress: (value) => report("visual", value),
        publication: visualPublication(),
        endingPublication: endingPublication(),
        prepareEndingAudio: (url) =>
          latest.current.audio.prepare([url], { signal: abort.signal }),
      });
      renderer.current = scene;
      if (latest.current.data.audio) {
        audioAssets = new PalAudioAssets(
          latest.current.data.audio.assetID,
          latest.current.audio,
          {
            signal: abort.signal,
            progress: (value) => report("audio", value),
          },
        );
        ownedAudio = new PalAudioDirector(
          audioAssets,
          latest.current.audio,
          audioIdentity,
          () => {
            const current = latest.current,
              paused =
                !!current.clock.paused ||
                !!current.room.game?.paused ||
                !!current.room.loading;
            return {
              audio: current.data.audio,
              now:
                current.clock.serverNow +
                (paused
                  ? 0
                  : Math.max(0, performance.now() - current.clock.receivedAt)),
              ready: readyRef.current,
              paused,
              quiet:
                document.hidden ||
                current.inputWindow === 0 ||
                !(current.audioEnabled ?? (current.sound || paused)),
            };
          },
          audioMemory.current,
        );
        audioDirector.current = ownedAudio;
      }
      await Promise.all([
        scene.prepare(latest.current.data, local?.previews),
        audioAssets?.prepare(latest.current.data.audio),
      ]);
      if (!alive) return;
      await scene.paint(canvas.current!, latest.current.data);
      await audioAssets?.prepare(latest.current.data.audio);
      if (!alive) return;
      readyRef.current = true;
      setReady(true);
      latest.current.onReady?.();
      const tick = (at: number) => {
        if (!alive) return;
        local?.setActive(runningRef.current());
        if (correction.current.sample(at)) redraw();
        if (local && root.current)
          root.current.dataset.palLocalState = JSON.stringify({
            ...local.status(),
            enabled: enabledRef.current(),
            phase: latest.current.data.phase,
            controlAt: latest.current.data.controlAt,
          });
        owned?.sample(at);
        ownedAudio?.update();
        frame = requestAnimationFrame(tick);
      };
      frame = requestAnimationFrame(tick);
    })().catch((e) => {
      if (!alive) return;
      const text = e instanceof Error ? e.message : "无法加载仙剑画面";
      setError(text);
      latest.current.onLoadError?.(text);
    });
    return () => {
      alive = false;
      readyRef.current = false;
      abort.abort();
      cancelAnimationFrame(frame);
      owned?.dispose();
      local?.dispose();
      correction.current.clear();
      if (localClient.current === local) localClient.current = undefined;
      ownedAudio?.dispose();
      audioAssets?.dispose();
      if (audioDirector.current === ownedAudio)
        audioDirector.current = undefined;
      controller?.dispose();
      window.removeEventListener("controller-settings-changed", changed);
      if (input.current === owned) input.current = undefined;
      renderer.current = undefined;
    };
  }, [audioIdentity, props.assets, retry]);
  useLayoutEffect(() => {
    input.current?.interrupt();
    localClient.current?.recover();
    audioDirector.current?.resync();
    redraw();
  }, [props.clock.epoch, transportPaused]);
  useEffect(() => {
    const scene = renderer.current;
    if (!ready || !battleAssetKey || !scene) return;
    let active = true;
    void scene.prepareBattle(latest.current.data.battleAssets).catch((e) => {
      if (active) setError(e instanceof Error ? e.message : "战斗素材未就绪");
    });
    return () => {
      active = false;
    };
  }, [battleAssetKey, ready]);
  useEffect(() => {
    localClient.current?.accept(
      props.data,
      props.clock,
      props.room.seats[props.room.you]?.network?.rttMs ?? 0,
    );
    input.current?.update();
    audioDirector.current?.update();
    if (!localClient.current) redraw();
  }, [
    props.data,
    props.room.version,
    props.room.seats[props.room.you]?.network?.rttMs,
  ]);
  useEffect(() => {
    const changed = () => {
      if (document.hidden && latest.current.data.audio)
        audioMemory.current.floor = Math.max(
          audioMemory.current.floor,
          latest.current.data.audio.serial,
        );
      setAudioVisibility((value) => value + 1);
    };
    document.addEventListener("visibilitychange", changed);
    return () => document.removeEventListener("visibilitychange", changed);
  }, []);
  useEffect(() => {
    const ending = props.data.ending;
    let alive = true;
    if (
      !ready ||
      !props.sound ||
      transportPaused ||
      !ending?.clip ||
      props.clock.paused ||
      props.room.game?.paused ||
      props.room.loading ||
      document.hidden
    ) {
      props.audio.stopTrack("pal-ending");
      return;
    }
    let playing = false,
      pending = false,
      attempts = 0;
    const id = `${props.room.matchId}:${props.data.epoch}:${ending.serial}:${audioVisibility}:${performance.now()}`;
    const start = async () => {
      if (pending || playing || !alive) return;
      pending = true;
      await renderer.current?.prepare(latest.current.data);
      const current = latest.current.data.ending;
      if (
        !alive ||
        document.hidden ||
        !current ||
        current.clip !== ending.clip ||
        current.serial !== ending.serial
      ) {
        pending = false;
        return;
      }
      const source = await renderer.current?.endingAudioSource(ending.clip);
      if (
        !alive ||
        !source ||
        document.hidden ||
        latest.current.data.ending?.serial !== ending.serial
      ) {
        pending = false;
        return;
      }
      playing = await props.audio.track("pal-ending", [source], {
        loop: false,
        playbackID: `${id}:${++attempts}`,
        startOffset: (current.elapsedMS ?? 0) / 1000,
      });
      pending = false;
    };
    const gesture = () => {
      void start().catch(() => {
        pending = false;
      });
    };
    document.addEventListener("pointerdown", gesture, true);
    document.addEventListener("keydown", gesture, true);
    gesture();
    return () => {
      alive = false;
      document.removeEventListener("pointerdown", gesture, true);
      document.removeEventListener("keydown", gesture, true);
      props.audio.stopTrack("pal-ending");
    };
  }, [
    ready,
    props.audio,
    props.sound,
    transportPaused,
    props.clock.epoch,
    props.clock.paused,
    props.room.game?.paused,
    props.room.loading,
    props.room.matchId,
    props.data.epoch,
    props.data.ending?.clip,
    props.data.ending?.serial,
    audioVisibility,
  ]);
  useEffect(() => {
    const panel = root.current?.querySelector<HTMLElement>(
      "[data-pal-menu-panel]",
    );
    if (!panel) return;
    const cancel = (event: Event) => {
      event.preventDefault();
      event.stopPropagation();
      if (latest.current.data.phase === "cooperation")
        input.current?.decide("coop_cancel");
      else if (latest.current.data.phase === "system")
        input.current?.decide("system_close");
      else setHandover(false);
    };
    panel.addEventListener("controller-cancel", cancel);
    return () => panel.removeEventListener("controller-cancel", cancel);
  }, [props.data.phase, handover]);
  const { data, room } = latest.current,
    seat = room.you,
    can = ready && !error && palInputAllowed(latest.current),
    read = data.phase === "dialogue";
  let panel;
  if (data.cooperation) {
    const c = data.cooperation,
      required = c.required.includes(seat);
    panel = (
      <section
        className="pal-shared-panel"
        data-pal-menu-panel
        role="group"
        aria-label="合击确认"
      >
        <strong>{c.name}</strong>
        <div className="pal-coop-members">
          {data.party
            .filter((p) => c.affectedRoles.includes(p.role))
            .map((p) => (
              <span key={p.role}>
                <b>{p.name}</b>
                <span>
                  {c.contributors.includes(p.role)
                    ? `体力 −${c.hpCost}`
                    : "本轮行动联动"}
                </span>
              </span>
            ))}
        </div>
        <div className="pal-panel-actions">
          <button
            disabled={!can || !required || c.approved[seat]}
            onClick={() => input.current?.decide("coop_accept")}
          >
            {c.approved[seat] ? "已同意" : "同意合击"}
          </button>
          <button
            disabled={!can || !required}
            onClick={() => input.current?.decide("coop_cancel")}
          >
            取消合击
          </button>
        </div>
      </section>
    );
  } else if (data.phase === "system") {
    panel = (
      <section
        className="pal-shared-panel"
        data-pal-menu-panel
        role="group"
        aria-label="系统"
      >
        <SaveGameControl
          room={room}
          userID={user}
          busy={props.busy}
          command={(type) => void props.command(type)}
        />
        <button
          disabled={!can}
          onClick={() => input.current?.decide("system_close")}
        >
          返回游戏
        </button>
      </section>
    );
  } else if (handover) {
    panel = (
      <section
        className="pal-shared-panel"
        data-pal-menu-panel
        role="group"
        aria-label="交接领队"
      >
        {room.seats.map((p, i) =>
          i === data.leader ? null : (
            <button
              key={p.id}
              onClick={() => {
                setHandover(false);
                input.current?.handover(i);
              }}
            >
              {p.name}
            </button>
          ),
        )}
        <button onClick={() => setHandover(false)}>取消</button>
      </section>
    );
  } else if (profile && input.current) {
    panel = (
      <>
        {room.seats.length > 1 && (
          <div className="pal-play-status">
            {read ? (
              <span>
                {data.dialogue!.read.filter(Boolean).length}/{room.seats.length}{" "}
                人已读
              </span>
            ) : (
              <span>
                {data.phase === "battle"
                  ? data.party[data.battle?.current ?? 0]?.name
                  : room.seats[data.leader]?.name}
              </span>
            )}
            {data.phase === "field" && room.seats.length > 1 && (
              <button
                data-pal-panel-action
                disabled={!can}
                onClick={() => setHandover(true)}
              >
                交接领队
              </button>
            )}
          </div>
        )}
        <PalControls
          key={props.clock.epoch}
          canvas={canvas}
          input={input.current}
          data={data}
          enabled={() => enabledRef.current()}
        />
      </>
    );
  }
  return (
    <div
      className="pal-board"
      ref={root}
      tabIndex={-1}
      data-pal-board
      data-pal-phase={data.phase}
      data-pal-dialog-position={data.dialogue?.position}
      data-pal-dialog-id={data.dialogue?.id}
      data-pal-menu-kind={data.phase === "menu" ? data.menu.kind : undefined}
      data-pal-menu-selected={data.menu.selected}
      data-pal-battle-menu={data.battle?.menu}
      data-pal-battle-state={data.battle?.state}
      data-pal-battle-selected={data.battle?.selected}
      data-pal-ready={ready}
      data-loaded-bytes={progress.loaded}
      data-total-bytes={progress.total}
      onPointerDown={(event) => {
        if (event.target === canvas.current)
          root.current?.focus({ preventScroll: true });
      }}
    >
      <PalStage
        canvasRef={canvas}
        controls={panel}
        label={read ? "仙剑奇侠传，点击画面继续对白" : "仙剑奇侠传"}
      >
        {error && (
          <div className="pal-board-error" role="alert">
            <p>{error}</p>
            <button onClick={() => setRetry((n) => n + 1)}>重新加载画面</button>
          </div>
        )}
      </PalStage>
      {data.dialogue && <p className="pal-sr-only">{data.dialogue.text}</p>}
    </div>
  );
}
