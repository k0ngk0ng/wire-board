import type { BoxDisplay } from "./BoxPanel";
import { HatchPanel } from "./ShopPanel";
import { personalView } from "./personal-view";
import { hasRouteResult, ResultPanel } from "./ResultPanel";
import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../../../web/src/ui/board-host";
import type { BoardProps } from "../../../web/src/game-api/module";
import { challengeLocksPanel, emptyChallenge } from "./ChallengePanel";
import { controllerCommand } from "./keyboard-controller";
import { keyboardCommand } from "./keyboard-command";
import { createPointerAim, pointerTarget } from "./pointer-aim";
import { deploymentKey, deploymentRangeKey, siteAllowsDeployment, siteAt, type DeploymentPreview, type DeploymentRange } from "./placement";
import type { DamageMode } from "./DamageMonitor";
import { I18nProvider } from "./I18n";
import { audioCues } from "./audio-cues";
import { AssetStore, type Progress } from "./assets";
import type { StageHandle } from "./Stage";
import { ClassicScene } from "./ClassicScene";
import { PokemonDetails } from "./PokemonDetails";
import { GamePanel, type Panel } from "./Panels";
import { interpolateBattle, pokemonAt } from "./battle-renderer";
import { loadNativeRuntime, readBattlePhysics, type NativeRuntime, type NativeHandle, type NativePredictionRuntime } from "./native-runtime";
import { createPredictionController, type PredictionFrame } from "./prediction";
import { DraftPanel } from "./DraftPanel";
import { Editor } from "./Editor";
import type { Action, Assets, Catalog, Point, Pokemon, Route, RuleState, StreamView, View } from "./types";

type Controller = ReturnType<typeof createPredictionController<NativeHandle, View, Action>>;
type Probe = { predictions?: unknown[]; costs?: { phase: string; milliseconds: number }[]; frames?: unknown[] };
const emptyAssets = (): Assets => ({ images: new Map(), urls: new Map(), version: 0, ready: false });

function completeView(value: StreamView): value is View {
  return [value.team, value.box, value.tiles, value.towers, value.enemies, value.projectiles, value.events].every(Array.isArray);
}

function routeAssetID(routes: Catalog["routes"], value: { route: number; background?: unknown }) {
  return routes[String(value.route)] ? value.route : Object.values(routes).find(route => route.background === value.background)?.id ?? 0;
}

async function loadSnapshot(roomID: string): Promise<View> {
  const result = await api(`/rooms/${encodeURIComponent(roomID)}/sync`) as { roomId: string; data: StreamView };
  if (result.roomId !== roomID || !completeView(result.data) || !result.data.snapshot) throw Error("同步状态缺少完整战场");
  return result.data;
}

export function Board(props: BoardProps<StreamView>) {
  const current = useRef(props);
  const root = useRef<HTMLElement>(null);
  const commandHandler = useRef<(command: NonNullable<ReturnType<typeof keyboardCommand>>) => void>(() => {});
  const canonicalRoom = useRef(props.room.id);
  const canonical = useRef<View | undefined>(completeView(props.data) ? props.data : undefined);
  const shown = useRef<View | undefined>(completeView(props.data) ? props.data : undefined);
  const stage = useRef<StageHandle>(null);
  const store = useRef<AssetStore | undefined>(undefined);
  const runtime = useRef<NativeRuntime<RuleState, View, Action> | undefined>(undefined);
  const controller = useRef<Controller | undefined>(undefined);
  const syncing = useRef(false);
  const fast = useRef<NativePredictionRuntime<View, Action> | undefined>(undefined);
  const ready = useRef(false);
  const lastEvent = useRef(0);
  const pendingSounds = useRef<View["events"]>([]);
  const inputAction = useRef<Action | undefined>(undefined);
  const inputSilent = useRef(false);
  const pointerAim = useRef<ReturnType<typeof createPointerAim> | undefined>(undefined);
  const deploymentCache = useRef<{ runtime: NativePredictionRuntime<View, Action>; key: string; preview: DeploymentPreview } | undefined>(undefined);
  const rangeCache = useRef<{ runtime: NativePredictionRuntime<View, Action>; key: string; tiles: Map<number, DeploymentRange> } | undefined>(undefined);
  const music = useRef<{ path: string; volume: number; started: boolean } | undefined>(undefined);
  const serial = useRef(0);
  const clientPrefix = useRef("");
  const [view, setView] = useState<View | undefined>(() => completeView(props.data) ? props.data : undefined);
  const [assets, setAssets] = useState<Assets>(emptyAssets);
  const [assetStore, setAssetStore] = useState<AssetStore>();
  const [selected, setSelected] = useState<number | undefined>(props.data.team?.[0]?.id);
  const [panel, setPanel] = useState<Panel>("team");
  const [hatches, setHatches] = useState<Pokemon[]>([]);
  const hatch = hatches[0];
  const [boxDisplay, setBoxDisplay] = useState<BoxDisplay>({ sort: 0, size: 0 });
  const [challengePlan, setChallengePlan] = useState(emptyChallenge);
  const [error, setError] = useState("");
  const [allRanges, setAllRanges] = useState(false);
  const [damageOpen, setDamageOpen] = useState(false);
  const [damageMode, setDamageMode] = useState<DamageMode>("trueDamage");
  const [placing, setPlacing] = useState(false);
  const [aiming, setAiming] = useState<number>();
  const [range, setRange] = useState<number>();
  const [details, setDetails] = useState<number>();
  const [detailToken, setDetailToken] = useState<number>();
  const [inspection, setInspection] = useState<Pokemon>();
  const interaction = useRef({ selected, placing, panel, details, aiming });
  interaction.current = { selected, placing, panel, details, aiming };
  const assetsRef = useRef(assets);
  assetsRef.current = assets;
  current.current = props;
  if (canonicalRoom.current !== props.room.id) {
    canonicalRoom.current = props.room.id;
    canonical.current = completeView(props.data) ? props.data : undefined;
    shown.current = canonical.current;
  } else if (completeView(props.data)) canonical.current = props.data;
  const controllable = !props.room.spectating && props.room.you >= 0 && !props.room.game?.finished && !props.room.seats[props.room.you]?.autoPlay && !props.room.loading && !props.data.saving;
  const controlRef = useRef(controllable);
  controlRef.current = controllable;

  const report = useRef({ assets: { loaded: 0, total: 0 }, runtime: { loaded: 0, total: 0 } });
  const progress = useCallback((kind: "assets" | "runtime", value: { loaded: number; total?: number }) => {
    report.current[kind] = { loaded: value.loaded, total: value.total ?? 0 };
    const loaded = report.current.assets.loaded + report.current.runtime.loaded;
    const total = report.current.assets.total + report.current.runtime.total;
    current.current.onLoadProgress?.(total ? Math.min(99, Math.floor(100 * loaded / total)) : 0, "读取战场与伙伴素材", { loaded, total });
  }, []);

  const fail = useCallback((cause: unknown) => {
    const message = cause instanceof Error ? cause.message : String(cause);
    setError(message);
    if (!ready.current) current.current.onLoadError?.(message);
  }, []);

  const receiveSound = useCallback((value: View, reset: boolean) => {
    const events = value.events ?? [];
    const highest = events.reduce((max, event) => Math.max(max, Number(event.id ?? 0)), 0);
    if (reset) { lastEvent.current = highest; pendingSounds.current = []; return; }
    const fresh = events.filter(event => Number(event.id ?? 0) > lastEvent.current);
    lastEvent.current = Math.max(lastEvent.current, highest);
    const seat = current.current.room.spectating ? 0 : current.current.room.you + 1;
    pendingSounds.current.push(...fresh.filter(event => event.type !== "shopEgg" && event.type !== "shopItem" || event.source === seat));
    const ownHatches = seat > 0 ? fresh.filter(event => event.source === seat && (event.type === "shopEgg" || event.type === "shopItem" && event.key === "gimmighoul")).flatMap(event => {
      const pokemon = value.team.find(entry => entry.id === event.target) ?? value.box.find(entry => entry.id === event.target);
      return pokemon ? [pokemon] : [];
    }) : [];
    if (ownHatches.length) setHatches(previous => [...previous, ...ownHatches]);
  }, []);

  const flushSound = useCallback((value: View) => {
    const visualTime = value.physicsClock ?? value.clock;
    const due = pendingSounds.current.filter(event => Number(event.at ?? 0) <= visualTime);
    pendingSounds.current = pendingSounds.current.filter(event => Number(event.at ?? 0) > visualTime);
    if (!current.current.sound || !store.current) return;
    const loader = store.current;
    const roomID = current.current.room.id;
    const audio = value.progression?.settings.audio;
    const cues = audioCues(value, assetsRef.current.catalog, due);
    for (const cue of cues.slice(-4)) {
      const volume = (audio?.master ?? 12) * (audio?.[cue.category] ?? 8) / 400;
      if (volume <= 0 || !loader.files.has(loader.path(cue.path))) continue;
      void loader.media(cue.path).then(url => {
        if (store.current === loader && current.current.room.id === roomID && current.current.sound && shown.current?.epoch === value.epoch && Math.abs((shown.current?.physicsClock ?? 0) - visualTime) < 350) void current.current.audio.play([url], { volume });
      }).catch(() => {});
    }
  }, []);

  const requestSync = useCallback(() => {
    if (syncing.current) return;
    const roomID = current.current.room.id;
    const opaque = fast.current;
    const prediction = controller.current;
    const native = runtime.current;
    const loader = store.current;
    if (!opaque || !prediction || !native || !loader) return;
    syncing.current = true;
    void loadSnapshot(roomID).then(async value => {
      if (current.current.room.id !== roomID || fast.current !== opaque || controller.current !== prediction) return;
      await native.ensureRoute(routeAssetID((native.catalog as unknown as Catalog).routes, value), progress => loader.external(progress));
      if (current.current.room.id !== roomID || fast.current !== opaque || controller.current !== prediction || prediction.sample().epoch > value.epoch) return;
      canonical.current = value;
      const imported = opaque.import(value.snapshot);
      try { prediction.receive({ state: imported, epoch: value.epoch, sequence: value.sequence, logicalMS: value.clock, acceptedInputs: value.acceptedInputs }); }
      finally { opaque.release(imported); }
    }).catch(cause => { if (current.current.room.id === roomID && fast.current === opaque) fail(cause); }).finally(() => { if (fast.current === opaque) syncing.current = false; });
  }, [fail]);

  const installPrediction = useCallback((native: NativeRuntime<RuleState, View, Action>) => {
    runtime.current = native;
    const opaque = native.createPredictionRuntime();
    fast.current = opaque;
    const prediction = createPredictionController<NativeHandle, View, Action>({
      runtime: { clone: opaque.clone, apply: opaque.apply, advance: opaque.advance, project: state => personalView(opaque.frame(state), current.current.room.you), release: opaque.release },
      actor: () => current.current.room.you,
      readClock: value => Number(value.clock ?? 0),
      readPaused: value => Boolean(value.paused),
      readPhysics: readBattlePhysics,
      now: () => performance.now(),
      requestFrame: callback => requestAnimationFrame(callback),
      cancelFrame: id => cancelAnimationFrame(id),
      send: (action, clientInputID) => {
        void current.current.act({ ...action, clientInputID }).then(() => prediction.acknowledge(clientInputID, true)).catch(cause => { prediction.acknowledge(clientInputID, false); fail(cause); });
      },
      onFrame: (next, metadata) => {
        const probe = (window as typeof window & { __pokepathPresentationProbe?: Probe }).__pokepathPresentationProbe;
        probe?.predictions?.push({ ...metadata, previousView: undefined, nativeAt: performance.now(), paused: Boolean(next.paused), waveActive: next.waveActive, physicsClock: next.physicsClock, visibleEnemies: next.enemies.filter(enemy => enemy.entered && !enemy.dead && !enemy.escaped).length });
        shown.current = next;
        stage.current?.paint(next, metadata);
        const visual = metadata.phase !== "reset" ? interpolateBattle(next, metadata.previousView, metadata.interpolation) : next;
        flushSound(visual);
        if ((metadata.phase !== "frame" && !(metadata.phase === "input" && inputSilent.current)) || visual.gold !== viewRef.current?.gold || visual.health !== viewRef.current?.health || visual.wave !== viewRef.current?.wave) {
          viewRef.current = visual;
          setView(visual);
        }
      },
      onAuthority: (next, metadata) => receiveSound(next, metadata.reset),
      onInput: next => {
        if (inputSilent.current || !current.current.sound || !store.current) return;
        if (inputAction.current?.type === "egg" || inputAction.current?.type === "buyItem") return;
        const audio = next.progression?.settings.audio;
        const loader = store.current;
        const roomID = current.current.room.id;
        const kind = inputAction.current?.type;
        const effect = kind === "upgrade" || kind === "continueRoute" ? "obtain" : kind === "retry" || kind === "custom_retry" ? "button2" : "click1";
        const path = `src/assets/sounds/ui/${effect}.wav`;
        const volume = (audio?.master ?? 12) * (audio?.ui ?? 8) / 400;
        if (volume > 0) void loader.media(path).then(url => { if (store.current === loader && current.current.room.id === roomID && current.current.sound && shown.current?.epoch === next.epoch) void current.current.audio.play([url], { volume }); }).catch(() => {});
      },
      requestSync,
      onReject: (_, message) => setError(message),
      onCost: sample => {
        const probe = (window as typeof window & { __pokepathPresentationProbe?: Probe }).__pokepathPresentationProbe;
        probe?.costs?.push(sample);
      },
    });
    controller.current = prediction;
    const value = canonical.current;
    if (!value) { requestSync(); return; }
    if (value.snapshot) {
      const imported = opaque.import(value.snapshot);
      try { prediction.reset({ state: imported, epoch: value.epoch, sequence: value.sequence, logicalMS: value.clock, acceptedInputs: value.acceptedInputs }); }
      finally { opaque.release(imported); }
    }
    else requestSync();
    prediction.setControllable(controlRef.current);
    prediction.setPaused(Boolean(current.current.room.loading || value.saving));
  }, [fail, receiveSound, flushSound, requestSync]);
  const viewRef = useRef(view);

  useEffect(() => {
    clientPrefix.current = crypto.randomUUID().slice(0, 12);
    syncing.current = false;
    const base = import.meta.env?.DEV ? new URL("pokepath/v1/", new URL(import.meta.env.BASE_URL, window.location.href)) : new URL("/_media/pokepath/v1/", window.location.origin);
    const loader = new AssetStore(base, (value: Progress) => progress("assets", value));
    store.current = loader;
    setAssets(emptyAssets());
    setAssetStore(undefined);
    setView(canonical.current);
    let alive = true;
    const load = async () => {
      let initial = canonical.current;
      if (!initial || initial.epoch !== current.current.data.epoch) {
        initial = await loadSnapshot(current.current.room.id);
        if (!alive) return;
        canonical.current = initial;
        shown.current = initial;
        viewRef.current = initial;
        setView(initial);
        setSelected(initial.team[0]?.id);
      }
      await loader.initialize();
      const provisional = await loader.json<Catalog>("catalog.json");
      const routeID = routeAssetID(provisional.routes, initial);
      loader.require(["catalog.json", `routes/${routeID}.json`]);
      const [catalog, route, uiText] = await Promise.all([Promise.resolve(provisional), loader.json<Route>(`routes/${routeID}.json`), loader.json("ui-text.json")]);
      await loader.prepare(initial, catalog, route);
      const native = await loadNativeRuntime<RuleState, View, Action>({ baseURL: new URL("native/", base), catalogBaseURL: base, route: routeID, resolveResource: path => loader.url(path), readResource: (path, expected) => loader.readExternal(path, expected), signal: loader.controller.signal, onProgress: value => loader.external(value) });
      let installed = routeID;
      while (alive && canonical.current) {
        const ahead = routeAssetID(catalog.routes, canonical.current);
        if (ahead === installed) break;
        await native.ensureRoute(ahead, value => loader.external(value));
        installed = ahead;
      }
      if (!alive) return;
      setAssetStore(loader);
      setAssets({ catalog, route, uiText, images: loader.images, urls: loader.urls, version: 1, ready: true });
      installPrediction(native);
    };
    void load().catch(cause => { if (alive) fail(cause); });
    return () => {
      alive = false;
      ready.current = false;
      controller.current?.destroy();
      controller.current = undefined;
      fast.current?.dispose();
      fast.current = undefined;
      runtime.current = undefined;
      loader.dispose();
      current.current.audio.stop();
    };
  }, [props.room.id, fail, installPrediction, progress]);

  useEffect(() => {
    const value = props.data;
    const prediction = controller.current;
    if (!prediction) { if (completeView(value)) { shown.current = value; viewRef.current = value; setView(value); } return; }
    prediction.setControllable(controllable);
    prediction.setPaused(Boolean(props.room.loading || value.saving));
    const opaque = fast.current;
    const native = runtime.current;
    const loader = store.current;
    if (!opaque || !native || !loader) return;
    let alive = true;
    const receive = () => {
      if (!alive || controller.current !== prediction || prediction.sample().epoch > value.epoch) return;
      let imported: NativeHandle | undefined;
      try {
        imported = value.snapshot ? opaque.import(value.snapshot) : undefined;
        prediction.receive({ epoch: value.epoch, sequence: value.sequence, fromSequence: value.fromSequence, logicalMS: value.clock, state: imported, operations: value.operations, acceptedInputs: value.acceptedInputs });
      } catch (cause) { fail(cause); }
      finally { if (imported) opaque.release(imported); }
    };
    if (prediction.state()?.route === value.route) receive();
    else void native.ensureRoute(routeAssetID((native.catalog as unknown as Catalog).routes, value), update => loader.external(update)).then(receive).catch(cause => { if (alive && controller.current === prediction) fail(cause); });
    return () => { alive = false; };
  }, [props.data, props.room.loading, controllable, fail]);

  useEffect(() => {
    const loader = store.current;
    const catalog = assets.catalog;
    if (!view || !assets.ready || !loader || loader !== assetStore || !catalog) return;
    let alive = true;
    const prepare = async () => {
      const routeID = routeAssetID(catalog.routes, view);
      const route = assets.route?.id === routeID ? assets.route : await loader.json<Route>(`routes/${routeID}.json`);
      await loader.prepare(view, catalog, route);
      await runtime.current?.ensureRoute(routeID, value => loader.external(value));
      if (alive) setAssets(previous => ({ ...previous, route, version: previous.version + 1 }));
    };
    void prepare().catch(fail);
    return () => { alive = false; };
  }, [view?.route, view?.wave, view?.progression?.secrets?.klefki, view?.progression?.secretItems?.gengarite, view?.progression?.secretItems?.beedrillite, view?.team.map(pokemon => `${pokemon.species}:${pokemon.sprite?.image}`).join(","), [...new Set(view?.enemies.map(enemy => `${enemy.key}:${enemy.sprite?.image ?? ""}`) ?? [])].sort().join(","), assets.catalog, assets.ready, assetStore, fail]);

  useEffect(() => {
    if (!view || !assets.ready || !store.current) return;
    const metadata = assets.catalog?.routes[String(view.route)];
    const key = metadata?.musicKey;
    if (!key) return;
    const path = `src/assets/sounds/music/${key}.mp3`;
    if (!store.current.files.has(path)) return;
    const values = view.progression?.settings.audio;
    const volume = (values?.master ?? 12) * (values?.music ?? 12) / 400;
    const previous = music.current;
    const request = { path, volume, started: previous?.path === path && previous.started || false };
    music.current = request;
    const loader = store.current;
    const roomID = props.room.id;
    if (props.sound && (!previous || previous.path !== path || previous.volume !== volume || !previous.started)) void loader.media(path).then(url => {
      if (music.current !== request || store.current !== loader || current.current.room.id !== roomID || !current.current.sound) return;
      request.started = true;
      return current.current.audio.music([url], { loop: true, playbackID: `${roomID}:${path}`, volume });
    }).catch(() => {});
  }, [assets.ready, assets.catalog, view?.route, view?.progression?.settings.audio.master, view?.progression?.settings.audio.music, props.sound, props.audio, props.room.id]);


  useEffect(() => {
    const loader = store.current;
    const font = view?.progression?.settings.font;
    if (!assets.ready || !loader || loader !== assetStore || !font || font === "system") return;
    void loader.font(font).catch(fail);
  }, [assets.ready, assetStore, view?.progression?.settings.font, fail]);

  const act = useCallback((action: Action, options: { coalesceKey?: string; silent?: boolean } = {}) => {
    if (!controlRef.current) return;
    if (!options.silent) setError("");
    const id = `${clientPrefix.current}:${++serial.current}`;
    const dispatch = () => {
      if (controller.current) {
        inputAction.current = action;
        inputSilent.current = Boolean(options.silent);
        const result = controller.current.input(action, id, { coalesceKey: options.coalesceKey ?? (action.type === "settings" ? String(action.name) : undefined) });
        inputAction.current = undefined;
        inputSilent.current = false;
        if (!result.accepted) setError(result.error ?? "当前不能执行此操作");
      }
      else void current.current.act({ ...action, epoch: current.current.data.epoch, clientInputID: id }).catch(fail);
    };
    try {
      if (action.type === "route" && action.route !== undefined && runtime.current && store.current && assetsRef.current.catalog) {
        const target = action.route;
        const loader = store.current;
        const native = runtime.current;
        void Promise.all([native.ensureRoute(target, value => loader.external(value)), loader.json<Route>(`routes/${target}.json`)]).then(async ([, route]) => {
          const snapshot = shown.current;
          if (!snapshot) throw Error("完整战场尚未就绪");
          await loader.prepare(snapshot, assetsRef.current.catalog!, route);
          setAssets(previous => ({ ...previous, route, version: previous.version + 1 }));
          dispatch();
        }).catch(fail);
      } else dispatch();
    } catch (cause) { inputSilent.current = false; inputAction.current = undefined; fail(cause); }
  }, [fail]);

  useEffect(() => {
    const roomID = props.room.id;
    const input = createPointerAim({ now: () => performance.now(), setTimer: (callback, delay) => window.setTimeout(callback, delay), clearTimer: id => window.clearTimeout(id), send: location => {
      const state = interaction.current;
      const target = pointerTarget(shown.current, assetsRef.current.catalog, state.selected, controlRef.current && !state.placing && state.panel === "team" && state.details === undefined && state.aiming === undefined);
      if (current.current.room.id !== roomID || target?.epoch !== location.epoch || target.pokemonID !== location.pokemonID) return;
      act({ type: "aim", ...location }, { coalesceKey: `pointer:${location.pokemonID}`, silent: true });
    } });
    pointerAim.current = input;
    return () => { input.dispose(); if (pointerAim.current === input) pointerAim.current = undefined; };
  }, [props.room.id, act]);

  useEffect(() => { pointerAim.current?.cancel(); }, [selected, placing, panel, details, aiming, controllable, view?.epoch, view?.paused]);

  const pointerPoint = useCallback((location: Point) => {
    const state = interaction.current;
    const target = pointerTarget(shown.current, assetsRef.current.catalog, state.selected, controlRef.current && !state.placing && state.panel === "team" && state.details === undefined && state.aiming === undefined);
    if (target) pointerAim.current?.move({ ...target, ...location });
    else pointerAim.current?.cancel();
  }, []);

  const placementPreview = useCallback((value: View, id: number) => {
    if (!controlRef.current) return undefined;
    const handle = controller.current?.state();
    const runtime = fast.current;
    if (!handle || !runtime || handle.epoch !== value.epoch || handle.route !== value.route) return undefined;
    const key = `${current.current.room.id}:${deploymentKey(value, id)}`;
    const previous = deploymentCache.current;
    if (previous?.runtime === runtime && previous.key === key) return previous.preview;
    const preview = runtime.deployment(handle, id);
    deploymentCache.current = { runtime, key, preview };
    return preview;
  }, []);

  const placementRange = useCallback((value: View, id: number, tileID: number) => {
    if (!controlRef.current) return undefined;
    const handle = controller.current?.state();
    const runtime = fast.current;
    if (!handle || !runtime || handle.epoch !== value.epoch || handle.route !== value.route) return undefined;
    const key = `${current.current.room.id}:${handle.projectionRevision}:${deploymentRangeKey(value, id)}`;
    let cache = rangeCache.current;
    if (cache?.runtime !== runtime || cache.key !== key) { cache = { runtime, key, tiles: new Map() }; rangeCache.current = cache; }
    const previous = cache.tiles.get(tileID);
    if (previous) return previous;
    try {
      const footprint = runtime.deploymentRange(handle, id, tileID);
      cache.tiles.set(tileID, footprint);
      return footprint;
    } catch { return undefined; }
  }, []);

  const point = useCallback((location: Point, fromController = false, touch = false) => {
    const value = shown.current;
    if (!value) return;
    if (fromController && !placing && aiming === undefined) { commandHandler.current({ type: "wave" }); return; }
    if (aiming !== undefined && controlRef.current) { act({ type: "aim", pokemonID: aiming, x: location.x, y: location.y }); setAiming(undefined); return; }
    const placed = pokemonAt(value, location);
    if (placed && !placing && value.team.some(pokemon => pokemon.id === placed.id)) { setSelected(placed.id); setDetails(placed.id); return; }
    const target = pointerTarget(value, assetsRef.current.catalog, selected, controlRef.current && !placing && panel === "team" && details === undefined);
    if (target) { pointerAim.current?.edge({ ...target, ...location }); return; }
    if (!placing && controlRef.current) {
      const hotspot = value.progression?.hotspots.find(hotspot => location.x >= hotspot.x && location.x <= hotspot.x + hotspot.width && location.y >= hotspot.y && location.y <= hotspot.y + hotspot.height);
      if (hotspot) { act({ type: hotspot.kind === "bell" ? "bell" : "secret", name: hotspot.key, x: location.x, y: location.y }); return; }
    }
    if (!placing || selected === undefined || !controlRef.current) {
      if (controlRef.current && value.team.some(pokemon => pokemon.tileID < 0)) setError("先选择下方队伍中的宝可梦，再点地图上的可建造位置");
      return;
    }
    const preview = placementPreview(value, selected);
    if (!preview) return;
    const tile = siteAt(value.tiles, location, preview.tileSize, touch);
    if (!tile || !siteAllowsDeployment(preview.sites[tile.id])) { setError("这里不能部署，请点亮起的可建造位置"); return; }
    if (value.team.find(pokemon => pokemon.id === selected)?.tileID !== tile.id) act({ type: "deploy", pokemonID: selected, tileID: tile.id });
    setPlacing(false);
  }, [selected, placing, aiming, panel, details, act, placementPreview]);

  const onReady = useCallback(() => {
    if (ready.current || !assetsRef.current.ready || !runtime.current) return;
    ready.current = true;
    const loaded = report.current.assets.loaded + report.current.runtime.loaded;
    const total = report.current.assets.total + report.current.runtime.total;
    current.current.onLoadProgress?.(100, "战场已就绪", { loaded, total });
    current.current.onReady?.();
  }, []);

  const liveDraft = view?.progression?.draft;
  const draftToken = liveDraft?.active ? liveDraft.token : undefined;
  const resultPending = hasRouteResult(view?.phase);
  useEffect(() => {
    if (!resultPending) return;
    setPanel("team");
    setPlacing(false);
    setAiming(undefined);
    setDetails(undefined);
    setDetailToken(undefined);
    setHatches([]);
  }, [resultPending]);
  useEffect(() => { if (draftToken !== undefined) setHatches([]); }, [draftToken]);
  useEffect(() => { setHatches([]); }, [props.room.id]);
  const detailEntry = view && !resultPending && draftToken === detailToken ? [...view.team, ...view.box].find(pokemon => pokemon.id === details) : undefined;
  useEffect(() => {
    if (draftToken === detailToken) return;
    setDetails(undefined);
    setDetailToken(undefined);
  }, [draftToken, detailToken]);
  useEffect(() => {
    if (details === undefined || !detailEntry) { setInspection(undefined); return; }
    const opaque = fast.current;
    const handle = controller.current?.state();
    if (!opaque || !handle) return;
    try { setInspection(opaque.inspect<Pokemon>(handle, details)); }
    catch (cause) { fail(cause); }
  }, [details, view?.epoch, detailEntry?.species, detailEntry?.level, detailEntry?.target, detailEntry?.item, detailEntry?.sprite?.base, fail]);

  useEffect(() => {
    const element = root.current;
    if (!element) return;
    const action = (event: Event) => { const command = controllerCommand(String((event as CustomEvent).detail?.action ?? "")); if (command) { event.preventDefault(); commandHandler.current(command); } };
    const cancel = (event: Event) => { event.preventDefault(); commandHandler.current({ type: "close" }); };
    element.addEventListener("controller-action", action);
    element.addEventListener("controller-cancel", cancel);
    return () => { element.removeEventListener("controller-action", action); element.removeEventListener("controller-cancel", cancel); };
  }, [Boolean(view)]);

  if (!view) return <section className="pokepath-board"><div className="pokepath-sync-status" role="status">正在同步战场</div>{error && <div role="alert">{error}</div>}</section>;

  const detailsPokemon = detailEntry && inspection?.id === detailEntry.id ? { ...detailEntry, combatStats: detailEntry.combatStats ?? inspection.combatStats, combatRangeType: detailEntry.combatRangeType ?? inspection.combatRangeType, combatRangeHalfWidth: detailEntry.combatRangeHalfWidth ?? inspection.combatRangeHalfWidth, combatInnerRange: detailEntry.combatInnerRange ?? inspection.combatInnerRange } : detailEntry;
  const select = (id: number) => { setError(""); setAiming(undefined); setSelected(id); setPlacing(controlRef.current); setDetails(undefined); setPanel("team"); };
  const openPanel = (next: Panel) => { if (challengeLocksPanel(view, next)) return; if (next === "routes" && panel !== "routes") act({ type: "mapOpen" }); setPanel(next); setDetails(undefined); setPlacing(false); };
  const openDetails = (id: number) => { setDetailToken(undefined); setSelected(id); setDetails(id); setPlacing(false); };
  const inspectCandidate = (id: number) => {
    if (draftToken === undefined || !liveDraft?.offers.includes(id)) return;
    setDetailToken(draftToken);
    setDetails(id);
  };
  const dismissHatch = () => { setHatches(previous => previous.slice(1)); };
  commandHandler.current = command => {
    if (resultPending) return;
    if (hatch) { if (command.type === "close") dismissHatch(); return; }
    if (draftToken !== undefined) { if (command.type === "close") setDetails(undefined); return; }
    if (command.type === "close") { setPanel("team"); setPlacing(false); setAiming(undefined); setDetails(undefined); pointerAim.current?.cancel(); return; }
    if (command.type === "ranges") { setAllRanges(value => !value); return; }
    if (command.type === "damage") { setDamageOpen(value => !value); return; }
    const locked = view.health <= 0;
    if (locked) return;
    if (command.type === "details") {
      if (details !== undefined) { const owned = [...view.team, ...view.box]; const index = owned.findIndex(pokemon => pokemon.id === details); const next = owned[(index + command.direction + owned.length) % owned.length]; if (next) setDetails(next.id); }
      return;
    }
    if (command.type === "panel") { if (placing || details !== undefined) return; openPanel(command.panel); return; }
    if (panel !== "team" || details !== undefined) return;
    if (command.type === "wave") { if (!placing) act({ type: !view.waveActive ? "wave" : view.paused ? "resume" : "pause" }); }
    else if (command.type === "action") { if (!placing) act(command.action); }
    else if (command.type === "select") { const id = view.team[command.slot]?.id; if (id !== undefined) { if (placing && id === selected) setPlacing(false); else if (!placing) select(id); } }
    else if (command.type === "aim") { const pokemon = view.team[command.slot]; if (!view.paused && pokemon && assets.catalog?.species[pokemon.species]?.attackType === "bombardment" && view.towers.some(tower => tower.pokemonID === pokemon.id)) { setAiming(pokemon.id); setPlacing(false); stage.current?.focus(); } }
  };
  return <section ref={root} className="pokepath-board" style={{ ...Object.fromEntries(Object.entries(assets.catalog?.themes?.[view.progression?.settings.theme ?? "original"] ?? {}).map(([key, value]) => [`--pp-${key}`, value])), "--pp-font": view.progression?.settings.font === "system" ? "system-ui" : view.progression?.settings.font ?? "PressStart2P" } as React.CSSProperties} data-game-focus tabIndex={-1} data-pokepath-theme={view.progression?.settings.theme ?? "original"} data-pokepath-board data-route={view.route} data-wave={view.wave} onKeyDown={event => {
    if (resultPending || (draftToken !== undefined || hatch) && event.key !== "Escape") return;
    if (event.currentTarget.closest("[data-controller-navigation]") && event.key !== "Escape" && !event.ctrlKey && !event.altKey) return;
    const typing = Boolean((event.target as HTMLElement).closest("input,select,textarea,[contenteditable=true]"));
    const command = keyboardCommand({ key: event.key, ctrlKey: event.ctrlKey, metaKey: event.metaKey, altKey: event.altKey, repeat: event.repeat, typing }, view.progression?.settings.language ?? 8);
    if (!command) return;
    if (command.type !== "close") event.preventDefault();
    commandHandler.current(command);
  }}>
    <I18nProvider language={view.progression?.settings.language ?? 8} dictionary={assets.uiText} catalog={assets.catalog}><ClassicScene view={view} modalOpen={Boolean(hatch)} assets={assets} store={assetStore} selected={selected} placing={placing} allRanges={allRanges} damageOpen={damageOpen} damageMode={damageMode} onDamageToggle={() => setDamageOpen(value => !value)} onDamageMode={setDamageMode} onPointerPoint={pointerPoint} onPointerCancel={() => pointerAim.current?.cancel()} range={aiming ?? range} panel={panel} disabled={!controllable} stage={stage} onAct={act} onSelect={select} onDetails={openDetails} onAim={id => { setDetails(undefined); setPanel("team"); setPlacing(false); setAiming(id); stage.current?.focus(); }} onInspect={id => { const handle = controller.current?.state(); if (!handle || !fast.current) return; try { return fast.current.inspect<Pokemon>(handle, id); } catch (cause) { fail(cause); } }} onPanel={openPanel} placementPreview={placementPreview} placementRange={placementRange} onPoint={point} onHover={setRange} onReady={onReady}>
      {!resultPending && draftToken === undefined && <div className="pokepath-panel-layer" inert={Boolean(hatch)}>{panel === "editor" ? <Editor view={view} catalog={assets.catalog} onAct={act} disabled={!controllable} onClose={() => setPanel("team")} /> : panel !== "team" && <GamePanel key={panel} panel={panel} controllerBindings={props.controllerBindings} challengePlan={challengePlan} onChallengePlan={setChallengePlan} boxDisplay={boxDisplay} onBoxDisplay={setBoxDisplay} view={view} assets={assets} store={assetStore} selected={selected} disabled={!controllable} onAct={act} onSelect={setSelected} onDetails={openDetails} onPanel={openPanel} onClose={() => setPanel("team")} />}</div>}
      <DraftPanel view={view} assets={assets} store={assetStore} disabled={!controllable} onAct={act} onDetails={inspectCandidate} />
      {detailsPokemon && <PokemonDetails pokemon={detailsPokemon} view={view} assets={assets} store={assetStore} inspectionOnly={draftToken !== undefined} disabled={!controllable || draftToken !== undefined} onAct={act} onClose={() => setDetails(undefined)} onSelect={setDetails} onAim={id => { setDetails(undefined); setPanel("team"); setPlacing(false); setAiming(id); stage.current?.focus(); }} onInventory={() => { setDetails(undefined); setPanel("inventory"); }} />}
      {!resultPending && draftToken === undefined && hatch && <HatchPanel pokemon={hatch} assets={assets} store={assetStore} font={view.progression?.settings.font} onClose={dismissHatch} />}
      {resultPending && <ResultPanel view={view} assets={assets} store={assetStore} disabled={!controllable} onAct={act} />}
      {error && <div className="pokepath-action-error" role="alert"><span>{error}</span><button type="button" aria-label="关闭错误提示" onClick={() => setError("")}>×</button></div>}
    </ClassicScene></I18nProvider>
  </section>;
}
