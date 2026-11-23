import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { BoardProps } from '../../../web/src/game-api/module';
import { useGameController } from '../../../web/src/controllers/use-game-controller';
import { loadBattleAssets, type LoadedAssets } from './assets';
import { validateRulesIdentity } from './asset-contract';
import { BuyMenu } from './BuyMenu';
import { Scoreboard } from './Scoreboard';
import { inventorySlots, itemName, mapName, mapMode, weapons } from './content';
import { emptyInput, InputChannel } from './input';
import { announcementSound, attackEventInCurrentLife, attackFeedbackBlocked, AttackFeedbackQueue, eventFeedback, localFeedback, type FeedbackKind } from './feedback';
import { EventCursor, MotionPrediction } from './prediction';
import { BattleRenderer } from './renderer';
import { BrushCache } from './brush-cache';
import { matchClock, observerCandidates, viewAngles } from './presentation';
import { useOverlayInsets } from './overlay-insets';
import { WeaponHistory } from './weapon-history';
import { TouchControls, type TouchControlsHandle, type TouchState } from './TouchControls';
import { PointerLockHandoff } from './touch-ownership';
import { classicBuyChoice } from './classic-menu';
import { actionProgress, BombAudioCursor, CueCursor, plantClick, soundCues } from './classic-hud';
import { ClassicStatus, HudNumber, HudSprite } from './ClassicHud';
import type { GameEvent, GameInput, InputAction, Player, View } from './types';
import './style.css';
import './mobile-controls.css';
import './clock.css';

type Metrics = { inputs: { at: number; createdAt: number; dispatchedAt: number; type: string; trusted: boolean }[]; responses: { inputAt: number; visibleAt: number; type: string; seq?: number }[]; frames: number[] };
type ObservedCanvas = HTMLCanvasElement & { __cs16Metrics?: Metrics; __cs16Frame?: Record<string, unknown>; __cs16Assets?: Record<string, unknown> };
const angle = (value: number) => (Math.round(value) % 65536 + 65536) % 65536;
const safePitch = (value: number) => Math.max(-16000, Math.min(16000, Math.round(value)));
let keyboardOwner: symbol | undefined;

export function Board(props: BoardProps<View>) {
  const brushCache = useRef(new BrushCache());
  const brushes = brushCache.current.update(props.data.epoch, props.data.mapId, props.data.brushes);
  const latest = useRef(props);
  latest.current = { ...props, data: { ...props.data, brushes } };
  const root = useRef<HTMLDivElement>(null);
  useOverlayInsets(root);
  const canvas = useRef<ObservedCanvas>(null);
  const loaded = useRef<LoadedAssets | undefined>(undefined);
  const renderer = useRef<BattleRenderer | undefined>(undefined);
  const prediction = useRef<MotionPrediction | undefined>(undefined);
  const channel = useRef<InputChannel | undefined>(undefined);
  const cursor = useRef(new EventCursor());
  const initialized = useRef(false);
  const facing = useRef({ yaw: 0, pitch: 0 });
  const mouse = useRef({ fire: false, secondary: false });
  const touch = useRef<TouchState>({ moveX: 0, moveY: 0, actions: new Set() });
  const touchControls = useRef<TouchControlsHandle>(null);
  const touchHandoff = useRef(new PointerLockHandoff());
  const selection = useRef('');
  const buying = useRef(false);
  const buyCategoryRef = useRef(0);
  const restoreMouse = useRef(false);
  const touchPointer = useRef(false);
  const clickCursor = useRef(new CueCursor());
  const beepCursor = useRef(new BombAudioCursor());
  const beepActive = useRef(false);
  const actionsBefore = useRef(new Set<string>());
  const lastInput = useRef({ input: emptyInput(), at: 0 });
  const lifeBefore = useRef(0);
  const wasEnabled = useRef(false);
  const wasLive = useRef(false);
  const automatedBefore = useRef(false);
  const cancellation = useRef(0);
  const process = useRef<(force?: boolean) => void>(() => {});
  const draw = useRef<(at: number) => void>(() => {});
  const receivedAt = useRef(performance.now());
  const metrics = useRef<Metrics>({ inputs: [], responses: [], frames: [] });
  const waiting = useRef<{ at: number; type: string; seq?: number } | undefined>(undefined);
  const processedAt = useRef(0);
  const lastVisible = useRef('');
  const drawCount = useRef(0);
  const lastShot = useRef({ at: -10000, weapon: '' });
  const attackFeedbacks = useRef(new AttackFeedbackQueue());
  const preparedWeapon = useRef('');
  const localFlash = useRef(0);
  const localAnimation = useRef(0);
  const lastFeedback = useRef<{ kind: FeedbackKind; at: number } | undefined>(undefined);
  const predictedWeapon = useRef<{ seq: number; life: number; id: string } | undefined>(undefined);
  const weaponHistory = useRef(new WeaponHistory());
  const [ready, setReady] = useState(false);
  const [, setControlRevision] = useState(0);
  const [error, setError] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [buyOpen, setBuyOpen] = useState(false);
  const [buyCategory, setBuyCategory] = useState(0);
  const [scoreboard, setScoreboard] = useState(false);
  const [now, setNow] = useState(performance.now());
  const [feed, setFeed] = useState<GameEvent[]>([]);
  const [watch, setWatch] = useState(props.room.you);
  const [locked, setLocked] = useState(false);
  const [hitUntil, setHitUntil] = useState(0);
  const [damage, setDamage] = useState<{ group: number; until: number }>();
  const own = props.data.players.find(player => player.seat === props.room.you);
  const identity = `${props.room.id}:${props.data.epoch}:${props.data.mapId}:${props.room.you}`;

  function canControl() {
    const current = latest.current;
    const player = current.data.players.find(player => player.seat === current.room.you);
    return initialized.current && !buying.current && !channel.current?.recovering && !current.busy && !current.clock.paused && !document.hidden && document.hasFocus() && !current.room.spectating && current.room.you >= 0 && !current.room.seats[current.room.you]?.autoPlay && !current.room.seats[current.room.you]?.left && current.data.canAct && !current.data.finished && !!player?.alive && (current.inputWindow ?? 0) > 0;
  }

  const controller = useGameController({ gameId: 'cs16', userId: props.room.seats[props.room.you]?.id ?? props.room.seats[0]?.id ?? '', scope: identity, root,
    enabled: () => initialized.current, menu: () => root.current?.querySelector<HTMLElement>('.cs16-buy-menu') ?? null,
    captureMappedModifiers: () => document.pointerLockElement === canvas.current && canControl(),
    menuActions: { forward: 'up', backward: 'down', left: 'left', right: 'right', use: 'confirm', jump: 'confirm', buy: 'cancel', buyEquipment: 'cancel' },
    changed: () => process.current(true), error: setError,
  });
  if (!channel.current) channel.current = new InputChannel(action => latest.current.act({ ...action }), setError);

  useLayoutEffect(() => {
    controller.current?.sample(performance.now());
  }, [buyOpen, ready, identity]);

  function record(event: Event) {
    const at = performance.now();
    const createdAt = Number.isFinite(event.timeStamp) && event.timeStamp >= 0 && event.timeStamp <= at + 1000 ? event.timeStamp : at;
    metrics.current.inputs.push({ at, createdAt, dispatchedAt: at, type: event.type, trusted: event.isTrusted });
    if (metrics.current.inputs.length > 1024) metrics.current.inputs.shift();
    if (canControl()) waiting.current = { at: createdAt, type: event.type };
  }

  function release(send = true) {
    setScoreboard(false);
    touchHandoff.current.reset();
    wasEnabled.current = false;
    mouse.current = { fire: false, secondary: false };
    touch.current = { moveX: 0, moveY: 0, actions: new Set() };
    selection.current = '';
    predictedWeapon.current = undefined;
    actionsBefore.current.clear();
    controller.current?.clear();
    if (send && (latest.current.inputWindow ?? 0) > 0) channel.current?.release(facing.current.yaw, facing.current.pitch);
    prediction.current?.suspend();
    lastInput.current = { input: emptyInput(facing.current.yaw, facing.current.pitch), at: 0 };
  }

  function beginTouch() {
    root.current?.focus({ preventScroll: true });
    touchHandoff.current.begin(!!document.pointerLockElement, () => release(), () => document.exitPointerLock());
  }

  function lockMouse() {
    if (!canControl() || !canvas.current || document.pointerLockElement === canvas.current) return;
    root.current?.focus({ preventScroll: true });
    try { const request = canvas.current.requestPointerLock(); if (request) void request.catch(() => {}); } catch {}
  }

  function closeBuy() {
    buying.current = false;
    setBuyOpen(false);
    controller.current?.clear();
    root.current?.focus({ preventScroll: true });
    prediction.current?.resume(performance.now());
    process.current(true);
    if (restoreMouse.current && !touchPointer.current) lockMouse();
  }

  function openBuy(category = 0) {
    const current = latest.current;
    if (!current.data.players.find(player => player.seat === current.room.you)?.alive || current.room.spectating) return;
    restoreMouse.current = !touchPointer.current && (document.pointerLockElement === canvas.current || !matchMedia('(pointer: coarse)').matches);
    release();
    buying.current = true;
    buyCategoryRef.current = category;
    setBuyCategory(category);
    setBuyOpen(true);
    if (touchPointer.current && document.pointerLockElement === canvas.current) document.exitPointerLock();
  }

  function buyItem(item: string) {
    const current = latest.current;
    const player = current.data.players.find(player => player.seat === current.room.you);
    if (!player?.alive || !current.data.buyAllowed) return;
    void current.act({ type: 'buy', epoch: current.data.epoch, life: player.life, round: current.data.round, item }).catch(() => {});
  }

  function chooseBuy(key: number) {
    const current = latest.current;
    const player = current.data.players.find(player => player.seat === current.room.you);
    if (!player || !buying.current) return;
    const choice = classicBuyChoice(player.team, buyCategoryRef.current, key, mapMode(current.data.mapId) ?? '');
    if (choice.category !== undefined) { buyCategoryRef.current = choice.category; setBuyCategory(choice.category); }
    if (choice.item) buyItem(choice.item);
    if (choice.close) closeBuy();
  }

  function select(id: string) {
    if (!canControl()) return;
    selection.current = id;
    process.current(true);
  }

  function soundFor(event: GameEvent) {
    const sounds = loaded.current?.sounds;
    if (!sounds) return;
    if (event.global) { const key = announcementSound(event); return key ? sounds.get(key) : undefined; }
    const cue = soundCues(event);
    const key = cue.files.find(candidate => sounds.has(candidate));
    return key ? sounds.get(key) : undefined;
  }

  function playEvent(event: GameEvent) {
    const current = latest.current;
    if (!current.sound || document.hidden) return;
    const source = soundFor(event);
    if (!source) return;
    if (event.global && event.type === 'bomb_beep') {
      beepActive.current = true;
      void current.audio.effect('cs16:c4-beep', [source], { playbackID: `${current.data.epoch}:${current.data.round}:${event.id}`, reuseLoaded: true });
      return;
    }
    if (event.global) { void current.audio.playWithFallback([source]); return; }
    const cue = soundCues(event);
    if (cue.local) { void current.audio.playWithFallback([source]); return; }
    const player = current.data.players.find(player => player.seat === current.room.you);
    const candidates = observerCandidates(current.data.players, player, !!current.room.spectating);
    const listener = player?.alive ? player : candidates.find(player => player.seat === watch) ?? candidates[0] ?? player;
    if (!listener || !Number.isFinite(listener.x)) return;
    const layers = [{ source, volumeMilli: cue.volumeMilli ?? 1000 }, ...cue.layers?.flatMap(layer => {
      const key = layer.files.find(key => loaded.current?.sounds.has(key));
      const source = key && loaded.current?.sounds.get(key);
      return source ? [{ source, volumeMilli: layer.volumeMilli }] : [];
    }) ?? []];
    for (const [index, layer] of layers.entries()) {
      const suffix = index === 0 ? '' : `:${index}`;
      void current.audio.spatial(`cs16:${event.type}:${event.seat}${suffix}`, [layer.source], { playbackID: `${current.data.epoch}:${event.life ?? 0}:${event.id}${suffix}`, source: { x: (event.x ?? listener.x) / 256, y: (event.y ?? listener.y) / 256, z: (event.z ?? listener.z) / 256 }, listener: { x: listener.x / 256, y: listener.y / 256, z: listener.z / 256, yaw: facing.current.yaw * Math.PI * 2 / 65536 }, maxDistance: cue.range, volume: layer.volumeMilli / 1000 });
    }
  }

  process.current = (force = false) => {
    const current = latest.current;
    const player = current.data.players.find(player => player.seat === current.room.you);
    const at = performance.now();
    const enabled = canControl();
    channel.current!.update({ identity, epoch: current.data.epoch, life: player?.life ?? 0, receivedSeq: player?.receivedSeq ?? 0, enabled, window: current.inputWindow ?? 0, frame: current.data.frame });
    const control = controller.current;
    if (enabled && !wasEnabled.current) control?.clear();
    const actions = control?.sample(at) ?? new Set<string>();
    const cancelled = control?.cancelVersion ?? 0;
    if (cancelled !== cancellation.current) { cancellation.current = cancelled; if (!actions.has('fire') && !touch.current.actions.has('fire') && !mouse.current.fire) channel.current!.release(facing.current.yaw, facing.current.pitch); }
    if (actions.has('buy') && !actionsBefore.current.has('buy')) { if (buying.current) closeBuy(); else openBuy(); }
    if (actions.has('buyEquipment') && !actionsBefore.current.has('buyEquipment')) { if (buying.current) closeBuy(); else openBuy(8); }
    if (actions.has('scoreboard') !== actionsBefore.current.has('scoreboard')) setScoreboard(actions.has('scoreboard'));
    if (!enabled || !player) {
      if (wasEnabled.current) { release(); wasEnabled.current = false; }
      actionsBefore.current = actions;
      return;
    }
    if (!wasEnabled.current) prediction.current?.resume(at);
    wasEnabled.current = true;
    if (actions.has('lastWeapon') && !actionsBefore.current.has('lastWeapon') && player.inventory?.some(item => item.id === weaponHistory.current.previous)) selection.current = weaponHistory.current.previous;
    for (const [action, item] of [['buyPrimaryAmmo', 'primammo'], ['buySecondaryAmmo', 'secammo']]) if (actions.has(action) && !actionsBefore.current.has(action)) {
      buyItem(item);
    }
    const strength = (id: string) => Math.round((control?.values.get(id) ?? (actions.has(id) ? 1 : 0)) * 1000);
    const dt = Math.max(1, Math.min(40, at - processedAt.current));
    processedAt.current = at;
    facing.current.yaw = angle(facing.current.yaw + (strength('lookLeft') - strength('lookRight')) * dt * 3 / 100);
    facing.current.pitch = safePitch(facing.current.pitch + (strength('lookDown') - strength('lookUp')) * dt * 3 / 100);
    prediction.current?.face(facing.current.yaw, facing.current.pitch);
    for (const slot of [1, 2, 3, 4, 5]) if (actions.has(`slot${slot}`) && !actionsBefore.current.has(`slot${slot}`)) {
      const items = player.inventory?.filter(item => slot === 3 ? item.id === 'knife' : slot === 4 ? ['hegrenade', 'flashbang', 'smokegrenade'].includes(item.id) : slot === 5 ? item.id === 'c4' : weapons.find(weapon => weapon.id === item.id)?.slot === slot) ?? [];
      if (items.length) selection.current = items[(items.findIndex(item => item.id === player.weapon) + 1) % items.length].id;
    }
    const held = (id: string) => actions.has(id) || touch.current.actions.has(id);
    const input: GameInput = {
      moveX: Math.max(-1000, Math.min(1000, strength('right') - strength('left') + touch.current.moveX)),
      moveY: Math.max(-1000, Math.min(1000, strength('forward') - strength('backward') + touch.current.moveY)),
      yaw: facing.current.yaw, pitch: facing.current.pitch, fire: held('fire') || mouse.current.fire, secondary: held('secondary') || mouse.current.secondary,
      jump: held('jump'), crouch: held('crouch'), walk: held('walk'), reload: held('reload'), use: held('use'), drop: held('drop'), ...(held('flashlight') ? { flashlight: true } : {}), ...(held('nightVision') ? { nightVision: true } : {}), select: selection.current,
    };
    const previousInput = lastInput.current.input;
    const changed = [...new Set([...Object.keys(input), ...Object.keys(previousInput)])].some(key => input[key as keyof GameInput] !== previousInput[key as keyof GameInput]);
    if (force || changed || at - lastInput.current.at >= 40) {
      const sent = channel.current!.push(input, action => {
        if (current.data.phase === 'live') prediction.current?.input(action, at);
        if (action.select && weaponHistory.current.select(action.select, action.seq, player.inventory)) predictedWeapon.current = { seq: action.seq, life: player.life, id: action.select };
        if (waiting.current) waiting.current.seq = action.seq;
      });
      if (sent) { lastInput.current = { input: { ...input }, at }; selection.current = ''; }
    }
    const weapon = loaded.current?.weapons.find(weapon => weapon.id === player.weapon);
    const previousFire = actionsBefore.current.has('__fire');
    const feedback = localFeedback(player.weapon, input, previousInput, player.ammo);
    const attackFeedback = feedback === 'shot' || feedback === 'slash' || feedback === 'stab';
    const cooldown = feedback === 'slash' ? 350 : feedback === 'stab' ? 1000 : Number(weapon?.cycle ?? 10) * 10;
    const feedbackFrame = current.data.frame + Math.max(0, Math.min(15, Math.floor((Math.round(at) - Math.round(receivedAt.current)) / 10)));
    const blocked = feedback && attackFeedbackBlocked({ kind: feedback, heldBefore: previousFire, nextAttack: player.nextAttack ?? 0, frame: feedbackFrame, reloadUntil: player.inventory?.find(item => item.id === player.weapon)?.reloadUntil ?? 0 });
    const reloadAllowed = feedback !== 'reload' || player.reserve > 0 && player.ammo < Number(weapon?.ammo ?? 0);
    if (feedback && !blocked && reloadAllowed && (player.weapon !== 'c4' || current.data.plantZone) && current.data.phase === 'live' && (!attackFeedback || (weapon?.automatic !== false || !previousFire || player.weapon === 'knife') && at - lastShot.current.at >= cooldown)) {
      lastShot.current = { ...lastShot.current, at, weapon: player.weapon };
      lastFeedback.current = { kind: feedback, at };
      localAnimation.current++;
      if (feedback === 'shot') localFlash.current = at + 70;
      renderer.current?.feedback(feedback, at);
      if (attackFeedback) {
        const serial = attackFeedbacks.current.predict(feedback, player.weapon, player.life, at, channel.current!.seq);
        if (player.weapon !== 'knife') playEvent({ id: -serial, type: 'shot', frame: current.data.frame, seat: player.seat, weapon: player.weapon, life: player.life, x: player.x, y: player.y, z: player.z, value: player.inventory?.find(item => item.id === player.weapon)?.silenced ? 1 : 0 });
      }
    }
    const nextActions = new Set(actions);
    if (input.fire) nextActions.add('__fire');
    actionsBefore.current = nextActions;
  };

  function stopBombBeep() {
    if (!beepActive.current) return;
    beepActive.current = false;
    latest.current.audio.stopEffect('cs16:c4-beep');
  }

  useLayoutEffect(() => { beepCursor.current.suspend(); stopBombBeep(); }, [props.sound, props.audioEnabled, props.clock.paused, identity]);

  useLayoutEffect(() => {
    const current = latest.current;
    const enabled = initialized.current && current.sound && current.audioEnabled !== false && !document.hidden && !current.clock.paused && current.data.phase === 'live' && !current.data.finished;
    const stage = beepCursor.current.consume(`${identity}:${current.data.round}`, current.data.bomb?.planted ? current.data.bombCue : 0, enabled, !!current.data.recover);
    if (!enabled || !current.data.bomb?.planted) stopBombBeep();
    if (stage) playEvent({ id: -(current.data.bombCue ?? 0), frame: current.data.frame, type: 'bomb_beep', seat: -1, value: stage, global: true });
  }, [props.data, identity]);

  useEffect(() => {
    const suspend = () => { beepCursor.current.suspend(); stopBombBeep(); };
    document.addEventListener('visibilitychange', suspend);
    return () => { document.removeEventListener('visibilitychange', suspend); stopBombBeep(); };
  }, [identity]);

  useLayoutEffect(() => {
    receivedAt.current = performance.now();
    if ((props.data.finished || props.room.seats[props.room.you]?.autoPlay) && document.pointerLockElement === canvas.current) document.exitPointerLock();
    if (!initialized.current || !loaded.current || !prediction.current) return;
    try { validateRulesIdentity(loaded.current.manifest.maps[props.data.mapId]?.rules, props.data.rulesSHA256); }
    catch (error) { release(); initialized.current = false; setError(error instanceof Error ? error.message : '地图规则版本不一致'); return; }
    const player = props.data.players.find(player => player.seat === props.room.you);
    loaded.current.dynamics(brushes, props.data.players);
    renderer.current?.update(latest.current.data, receivedAt.current);
    if (player && Number.isFinite(player.x)) {
      const automated = !!props.room.seats[props.room.you]?.autoPlay;
      facing.current = viewAngles(player, facing.current, automated || automatedBefore.current);
      automatedBefore.current = automated;
      if (automated || props.data.recover) { lastFeedback.current = undefined; attackFeedbacks.current.reset(); }
      if (lifeBefore.current !== player.life) {
        attackFeedbacks.current.reset();
        lifeBefore.current = player.life;
        facing.current = { yaw: player.yaw, pitch: player.pitch };
        mouse.current = { fire: false, secondary: false };
        touch.current = { moveX: 0, moveY: 0, actions: new Set() };
        controller.current?.clear();
      }
      if (preparedWeapon.current !== player.weapon) {
        preparedWeapon.current = player.weapon;
        const source = soundFor({ id: 0, type: 'shot', frame: props.data.frame, seat: player.seat, weapon: player.weapon });
        if (source) void props.audio.prepare([source]).catch(() => {});
      }
      weaponHistory.current.confirm(player, identity, !!props.data.recover);
      prediction.current.confirm(player, props.data.frame, receivedAt.current, identity, !!props.data.recover);
      if (predictedWeapon.current && (props.data.recover || player.life !== predictedWeapon.current.life || player.appliedSeq >= predictedWeapon.current.seq)) predictedWeapon.current = undefined;
      if (props.data.phase !== 'live') prediction.current.suspend();
      else if (!wasLive.current) prediction.current.resume(receivedAt.current);
      wasLive.current = props.data.phase === 'live';
    }
    const events = cursor.current.consume(props.data.epoch, props.data.eventId, props.data.events ?? []);
    for (const event of events) {
      if (event.type === 'hit' && event.seat === props.room.you) setHitUntil(performance.now() + 180);
      if (event.type === 'hit' && event.target === props.room.you) setDamage({ group: event.hitgroup ?? 0, until: performance.now() + 900 });
      const feedback = lastFeedback.current;
      const kind = eventFeedback(event);
      const ownAttack = event.seat === props.room.you && (kind === 'shot' || kind === 'slash' || kind === 'stab');
      if (ownAttack && !attackEventInCurrentLife(event, props.room.you, player?.life ?? 0)) continue;
      const predictedAttack = ownAttack && attackFeedbacks.current.confirm(kind, event.weapon ?? '', event.life ?? player?.life ?? 0, receivedAt.current);
      const predicted = predictedAttack || !ownAttack && feedback && event.seat === props.room.you && feedback.kind === kind && receivedAt.current - feedback.at < 350;
      if (!predicted) renderer.current?.event(event, receivedAt.current, props.room.seats[props.room.you]?.autoPlay ? -1 : props.room.you);
      if (ownAttack && !predictedAttack && kind === 'shot') lastShot.current = { at: receivedAt.current, weapon: event.weapon ?? '' };
      if (!predictedAttack) playEvent(event);
    }
    if (events.some(event => event.type === 'kill')) setFeed(previous => [...previous, ...events.filter(event => event.type === 'kill')].slice(-5));
    process.current();
  }, [props.data, props.inputWindow, props.clock.paused, props.busy, identity]);

  useEffect(() => {
    const abort = new AbortController();
    let active = true;
    initialized.current = false;
    attackFeedbacks.current.reset();
    preparedWeapon.current = '';
    channel.current?.dispose();
    const ownedChannel = new InputChannel(action => {
      const current = latest.current;
      const scope = `${current.room.id}:${current.data.epoch}:${current.data.mapId}:${current.room.you}`;
      if (!active || scope !== identity) return Promise.resolve();
      return current.act({ ...action }).catch(error => {
        if (active) attackFeedbacks.current.reject(action.seq, action.life);
        const owner = latest.current.data.players.find(player => player.seat === latest.current.room.you);
        if (active && action.select && owner && weaponHistory.current.reject(action.seq, owner, identity)) predictedWeapon.current = undefined;
        throw error;
      });
    }, message => { if (active) { setError(message); setControlRevision(value => value + 1); } }, undefined, () => { prediction.current?.suspend(); wasEnabled.current = false; mouse.current = { fire: false, secondary: false }; touch.current = { moveX: 0, moveY: 0, actions: new Set() }; controller.current?.clear(); });
    channel.current = ownedChannel;
    setReady(false);
    setError('');
    let bytes = { loaded: 0, total: 0 };
    void loadBattleAssets(props.assets, props.data.mapId, [...new Set(props.data.options.characters)], abort.signal, (percent, label, value) => {
      if (value) bytes = value;
      if (active) latest.current.onLoadProgress?.(percent, label, value);
    }, props.data.rulesSHA256).then(async assets => {
      if (!active || !canvas.current) return;
      loaded.current = assets;
      canvas.current.__cs16Assets = assets.audit;
      renderer.current = new BattleRenderer(canvas.current, assets);
      prediction.current = new MotionPrediction(assets.motion, assets.canSmooth);
      const current = latest.current;
      assets.dynamics(current.data.brushes ?? [], current.data.players);
      const player = current.data.players.find(player => player.seat === current.room.you);
      if (player && Number.isFinite(player.x)) {
        facing.current = { yaw: player.yaw, pitch: player.pitch };
        lifeBefore.current = player.life;
        weaponHistory.current.confirm(player, identity, true);
        prediction.current.confirm(player, current.data.frame, performance.now(), identity);
        if (current.data.phase !== 'live') prediction.current.suspend();
      }
      cursor.current.consume(current.data.epoch, current.data.eventId, current.data.events ?? []);
      renderer.current.update(current.data, performance.now());
      renderer.current.draw(current.data, player?.alive ? player : undefined, current.room.you, performance.now(), player && !current.room.spectating ? player.team : undefined);
      await renderer.current.prepare();
      if (!active) return;
      const preparedSound = soundFor({ id: 0, type: 'shot', frame: current.data.frame, seat: current.room.you, weapon: player?.weapon ?? 'glock18' });
      if (preparedSound) await current.audio.prepare([preparedSound], { signal: abort.signal });
      if (!active) return;
      initialized.current = true;
      setReady(true);
      current.onLoadProgress?.(100, '战场已就绪', bytes);
      current.onReady?.();
      process.current(true);
    }).catch(error => {
      if (!active || abort.signal.aborted) return;
      const message = error instanceof Error ? error.message : '战场加载失败';
      setError(message);
      latest.current.onLoadError?.(message);
    });
    return () => { active = false; abort.abort(); initialized.current = false; ownedChannel.dispose(); release(false); if (channel.current === ownedChannel) channel.current = undefined; renderer.current?.dispose(); renderer.current = undefined; prediction.current = undefined; loaded.current = undefined; props.audio.stopSpatial(); };
  }, [identity, props.assets, attempt]);

  useEffect(() => {
    let animation = 0;
    const tick = (at: number) => { draw.current(at); animation = requestAnimationFrame(tick); };
    animation = requestAnimationFrame(tick);
    const interval = window.setInterval(() => setNow(performance.now()), 100);
    return () => { cancelAnimationFrame(animation); clearInterval(interval); };
  }, []);

  draw.current = at => {
    if (!initialized.current || !renderer.current) return;
    try {
      process.current();
      const current = latest.current;
      const player = current.data.players.find(player => player.seat === current.room.you);
      const automated = !!current.room.seats[current.room.you]?.autoPlay;
      const local = player?.alive && !automated ? current.data.phase === 'live' && canControl() && prediction.current ? prediction.current.sample(at) : { ...player, yaw: facing.current.yaw, pitch: facing.current.pitch } : undefined;
      const selectedWeapon = predictedWeapon.current && local?.inventory?.find(item => item.id === predictedWeapon.current?.id);
      if (local && selectedWeapon) { local.weapon = selectedWeapon.id; local.ammo = selectedWeapon.ammo; local.reserve = selectedWeapon.reserve; }
      const presented = local && current.data.phase === 'live' && canControl() ? prediction.current?.present(local, at) ?? local : local;
      const candidates = observerCandidates(current.data.players, player, !!current.room.spectating);
      const candidate = presented ?? candidates.find(player => player.seat === watch) ?? candidates[0] ?? player;
      const seat = player?.alive ? current.room.you : candidate?.seat ?? -1;
      const viewPlayer = renderer.current.draw(current.data, presented, seat, at, player && !current.room.spectating ? player.team : undefined) ?? candidate;
      const cueFrame = current.data.frame + (current.clock.paused ? 0 : Math.max(0, Math.min(15, Math.floor((at - receivedAt.current) / 10))));
      const click = player?.alive && player.action === 'plant' ? plantClick(cueFrame, player.actionSince ?? cueFrame) : undefined;
      if (clickCursor.current.consume(`${identity}:${current.data.round}:${player?.actionSince ?? 0}`, click === undefined ? '' : String(click))) playEvent({ id: -cueFrame, frame: cueFrame, type: 'bomb_click', seat: current.room.you, x: player?.x, y: player?.y, z: player?.z });
      if (viewPlayer && current.sound) current.audio.spatialListener({ x: viewPlayer.x / 256, y: viewPlayer.y / 256, z: viewPlayer.z / 256, yaw: viewPlayer.yaw * Math.PI * 2 / 65536 });
      const visible = `${viewPlayer?.x}:${viewPlayer?.y}:${viewPlayer?.z}:${viewPlayer?.yaw}:${viewPlayer?.pitch}:${viewPlayer?.weapon}:${at < localFlash.current}:${localAnimation.current}`;
      if (waiting.current && lastVisible.current && visible !== lastVisible.current) {
        metrics.current.responses.push({ inputAt: waiting.current.at, visibleAt: performance.now(), type: waiting.current.type, seq: waiting.current.seq });
        if (metrics.current.responses.length > 1024) metrics.current.responses.shift();
        waiting.current = undefined;
      }
      lastVisible.current = visible;
      metrics.current.frames.push(at);
      if (metrics.current.frames.length > 8192) metrics.current.frames.shift();
      drawCount.current++;
      if (canvas.current) {
        canvas.current.__cs16Metrics = metrics.current;
        canvas.current.__cs16Frame = { at, drawCount: drawCount.current, x: viewPlayer?.x, y: viewPlayer?.y, z: viewPlayer?.z, yaw: viewPlayer?.yaw, pitch: viewPlayer?.pitch, serverFrame: current.data.frame, predictedFrame: prediction.current?.predictedFrame ?? current.data.frame, receivedSeq: player?.receivedSeq ?? 0, appliedSeq: player?.appliedSeq ?? 0, inputWindow: current.inputWindow ?? 0, pending: prediction.current?.pendingCount ?? 0, inputEnabled: canControl(), weapon: viewPlayer?.weapon, life: player?.life, phase: current.data.phase, mapId: current.data.mapId, previousWeapon: weaponHistory.current.previous, currentWeapon: weaponHistory.current.current, controllerActions: [...actionsBefore.current], touch: { moveX: touch.current.moveX, moveY: touch.current.moveY, actions: [...touch.current.actions] }, effects: renderer.current.visualStats };
      }
    } catch (error) {
      initialized.current = false;
      release();
      setError(error instanceof Error ? error.message : '画面暂时无法继续，请重试');
    }
  };

  useLayoutEffect(() => {
    const element = canvas.current;
    if (!element) return;
    const keyboard = (navigator as Navigator & { keyboard?: { lock: (keys: string[]) => Promise<void>; unlock: () => void } }).keyboard;
    const owner = Symbol('cs16-keyboard');
    let disposed = false;
    const unlockKeyboard = () => { if (keyboard && keyboardOwner === owner) { keyboard.unlock(); keyboardOwner = undefined; } };
    const keyboardActive = () => !disposed && document.pointerLockElement === element && !!document.fullscreenElement?.contains(element) && canControl();
    const syncKeyboard = () => {
      if (!keyboard || !keyboardActive()) { unlockKeyboard(); return; }
      keyboardOwner = owner;
      void keyboard.lock(['ControlLeft', 'ControlRight', 'KeyW', 'KeyR']).then(() => {
        if (!keyboardActive() && (keyboardOwner === owner || keyboardOwner === undefined)) { keyboard.unlock(); if (keyboardOwner === owner) keyboardOwner = undefined; }
      }).catch(() => { if (keyboardOwner === owner) keyboardOwner = undefined; });
    };
    const key = (event: KeyboardEvent) => {
      if (!root.current?.contains(document.activeElement) && document.pointerLockElement !== element) return;
      touchControls.current?.clear();
      record(event);
      touchPointer.current = false;
      if (!buying.current) return;
      const digit = /^(?:Digit|Numpad)([0-9])$/.exec(event.code);
      if (!digit && !['Escape', 'KeyB', 'KeyO'].includes(event.code)) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      if (!event.repeat) { if (digit) chooseBuy(Number(digit[1])); else closeBuy(); }
    };
    const mouseMove = (event: MouseEvent) => {
      if (touchHandoff.current.pending || document.pointerLockElement !== element || !canControl()) return;
      record(event);
      facing.current.yaw = angle(facing.current.yaw - event.movementX * 10);
      facing.current.pitch = safePitch(facing.current.pitch + event.movementY * 10);
      prediction.current?.face(facing.current.yaw, facing.current.pitch);
    };
    const down = (event: MouseEvent) => {
      touchControls.current?.clear();
      if (touchHandoff.current.pending || !canControl() || event.button > 2 || event.button === 1) return;
      record(event);
      root.current?.focus({ preventScroll: true });
      if (document.pointerLockElement !== element) {
        if (event.button === 0) lockMouse();
        event.preventDefault();
        return;
      }
      if (event.button === 0) mouse.current.fire = true;
      if (event.button === 2) mouse.current.secondary = true;
      event.preventDefault();
      process.current(true);
    };
    const up = (event: MouseEvent) => {
      if (event.button === 0) mouse.current.fire = false;
      if (event.button === 2) mouse.current.secondary = false;
      process.current(true);
    };
    const lock = () => { const enabled = document.pointerLockElement === element; setLocked(enabled); if (touchHandoff.current.lockChanged(enabled)) release(); syncKeyboard(); };
    const clear = () => { release(); unlockKeyboard(); };
    const surface = root.current?.closest('[data-game-fullscreen-surface]');
    const fullscreen = () => { release(); syncKeyboard(); };
    const tab = (event: KeyboardEvent) => {
      const target = event.target;
      if (event.code !== 'Tab' || event.ctrlKey || event.altKey || event.metaKey || !(target instanceof Element) || !surface?.hasAttribute('data-room-fullscreen') || !surface.contains(target)) return;
      event.preventDefault();
    };
    const fullscreenChanges = new MutationObserver(fullscreen);
    if (surface) fullscreenChanges.observe(surface, { attributes: true, attributeFilter: ['data-room-fullscreen'] });
    const hidden = () => { if (document.hidden) clear(); };
    const wheel = (event: WheelEvent) => {
      if (!canControl()) return;
      event.preventDefault();
      const player = latest.current.data.players.find(player => player.seat === latest.current.room.you);
      const inventory = player?.inventory ?? [];
      if (!inventory.length) return;
      const index = inventory.findIndex(item => item.id === player?.weapon);
      select(inventory[(index + (event.deltaY > 0 ? 1 : inventory.length - 1)) % inventory.length].id);
    };
    window.addEventListener('keydown', key, true);
    window.addEventListener('keydown', tab);
    document.addEventListener('mousemove', mouseMove);
    element.addEventListener('mousedown', down);
    window.addEventListener('mouseup', up);
    document.addEventListener('pointerlockchange', lock);
    document.addEventListener('fullscreenchange', fullscreen);
    window.addEventListener('blur', clear);
    document.addEventListener('visibilitychange', hidden);
    element.addEventListener('wheel', wheel, { passive: false });
    return () => { disposed = true; fullscreenChanges.disconnect(); unlockKeyboard(); window.removeEventListener('keydown', key, true); window.removeEventListener('keydown', tab); document.removeEventListener('mousemove', mouseMove); element.removeEventListener('mousedown', down); window.removeEventListener('mouseup', up); document.removeEventListener('pointerlockchange', lock); document.removeEventListener('fullscreenchange', fullscreen); window.removeEventListener('blur', clear); document.removeEventListener('visibilitychange', hidden); element.removeEventListener('wheel', wheel); };
  }, [identity]);

  const frame = props.data.frame + (props.clock.paused ? 0 : Math.max(0, Math.min(15, Math.floor((now - receivedAt.current) / 10))));
  const zoom = own?.inventory?.find(item => item.id === own.weapon)?.zoom ?? 0;
  const flash = own && (own.flashUntil ?? 0) > frame ? Math.max(0, Math.min(1, (own.flashStrength ?? 1000) / 1000)) : 0;
  const phaseLabel = props.data.finished ? '比赛结束' : props.clock.paused ? '已暂停' : props.data.phase === 'freeze' ? '准备开战' : props.data.phase === 'round_end' ? '回合结束' : props.data.bomb?.planted ? '炸弹已安装' : '';
  const mode = mapMode(props.data.mapId) ?? '';
  const clock = matchClock(mode, props.data.phase, props.data.phaseEndsAt - frame, !!props.data.bomb?.planted);
  const hud = loaded.current?.manifest.hud;
  const progress = own ? actionProgress(own, props.data.bomb, frame) : undefined;
  const scoped = !!zoom && ['scout', 'awp', 'g3sg1', 'sg550'].includes(own?.weapon ?? '');
  return <div className={`cs16-board${buyOpen ? ' cs16-buying' : ''}${scoreboard ? ' cs16-scoring' : ''}`} data-cs16-board data-ready={ready ? 'true' : 'false'} ref={root} tabIndex={-1} aria-label="反恐精英战场" onPointerDownCapture={event => { touchPointer.current = event.pointerType === 'touch'; if (event.pointerType === 'touch' && document.pointerLockElement === canvas.current) { try { beginTouch(); } catch {} } }}>
    <canvas ref={canvas} data-cs16-canvas aria-label={`${mapName(props.data.mapId)}第一人称战场`} />
    {ready && <>
      {clock && <div data-chat-protected className="cs16-match-hud"><HudSprite hud={hud} base={props.assets} name="stopwatch" /><strong>{clock}</strong></div>}
      {phaseLabel && <div className="cs16-round-message">{phaseLabel}</div>}
      <div className={`cs16-crosshair${scoped ? ' cs16-scoped' : ''}${now < hitUntil ? ' cs16-hit-confirmed' : ''}`} aria-hidden="true"><i /><b /></div>
      {!!flash && <div className="cs16-flash" style={{ opacity: flash }} aria-hidden="true" />}
      <div className="cs16-kill-feed">{feed.filter(event => event.frame > frame - 700).map(event => <div key={event.id} aria-label={`${props.room.seats[event.seat]?.name ?? '玩家'}使用${itemName(event.weapon ?? '')}${event.value === 1 ? '爆头' : '击杀'}${props.room.seats[event.target ?? -1]?.name ?? '玩家'}`}><span className={`cs16-name-${props.data.players[event.seat]?.team}`}>{props.room.seats[event.seat]?.name ?? '玩家'}</span><HudSprite hud={hud} base={props.assets} name={hud?.killIcons[event.weapon === 'hegrenade' ? 'grenade' : event.weapon ?? ''] ?? 'd_skull'} label={itemName(event.weapon ?? '')} />{event.value === 1 && <HudSprite hud={hud} base={props.assets} name="d_headshot" label="爆头" />}<span className={`cs16-name-${props.data.players[event.target ?? -1]?.team}`}>{props.room.seats[event.target ?? -1]?.name ?? '玩家'}</span></div>)}</div>
      {own && <>
        <ClassicStatus hud={hud} base={props.assets} player={own} view={props.data} frame={frame} />
        <div data-chat-protected className="cs16-vitals"><span><HudSprite hud={hud} base={props.assets} name="cross" /><HudNumber hud={hud} base={props.assets} value={own.health ?? 0} label={`生命 ${own.health ?? 0}`} /></span><span><HudSprite hud={hud} base={props.assets} name={own.helmet ? 'suithelmet_full' : 'suit_full'} /><HudNumber hud={hud} base={props.assets} value={own.armor ?? 0} label={`护甲 ${own.armor ?? 0}`} /></span></div>
        <div data-chat-protected className="cs16-money"><HudSprite hud={hud} base={props.assets} name="dollar" /><HudNumber hud={hud} base={props.assets} value={own.money ?? 0} label={`金钱 ${own.money ?? 0}`} /></div>
        <div data-chat-protected className="cs16-ammo" aria-label={`${itemName(own.weapon)}，弹匣 ${own.ammo ?? 0}，备弹 ${own.reserve ?? 0}`}><HudNumber hud={hud} base={props.assets} value={own.ammo ?? 0} /><span className="cs16-ammo-divider" /><HudNumber hud={hud} base={props.assets} value={own.reserve ?? 0} /><HudSprite hud={hud} base={props.assets} name={hud?.weapons[own.weapon]?.ammo ?? ''} /></div>
        {own.flashlight && <div className="cs16-flashlight-status" aria-label={`手电筒，电量 ${own.flashlightCharge ?? 0}`}><HudSprite hud={hud} base={props.assets} name="flash_full" /></div>}
        {damage && now < damage.until && <div className="cs16-damage-info">{damage.group === 1 ? '头部中弹' : damage.group === 6 || damage.group === 7 ? '腿部中弹' : '身体中弹'}</div>}
        {progress && <div className="cs16-action-progress" role="progressbar" aria-label={progress.kind === 'plant' ? '安装 C4' : '拆除 C4'} aria-valuemin={0} aria-valuemax={1000} aria-valuenow={progress.value}><span style={{ width: `${progress.value / 10}%` }} /></div>}
        <div className="cs16-inventory" aria-label="携带装备">{inventorySlots(own.inventory ?? [], own.weapon).map(item => <button type="button" key={item.id} data-touch-control data-cs16-select={item.id} data-cs16-slot={weapons.find(weapon => weapon.id === item.id)?.slot ?? (item.id === 'knife' ? 3 : 4)} aria-label={`切换${item.label === '投掷物' ? `${item.label}：${itemName(item.next)}` : item.label}`} aria-pressed={own.weapon === item.id} disabled={!canControl()} onClick={() => select(item.next)}>{item.label}</button>)}</div>
        <div className="cs16-battle-tools"><button type="button" data-cs16-open-buy disabled={!own.alive || props.room.spectating} onClick={() => openBuy()}>购买</button><button type="button" aria-pressed={scoreboard} onClick={() => setScoreboard(value => !value)}>战绩</button></div>
      </>}
      {(!own?.alive || props.room.spectating) && <div className="cs16-spectator"><strong>{own ? '阵亡观战' : '观战'}</strong><select aria-label="观战玩家" value={watch} onChange={event => setWatch(Number(event.target.value))}><option value={-1}>自动选择</option>{observerCandidates(props.data.players, own, !!props.room.spectating).map(player => <option value={player.seat} key={player.seat}>{props.room.seats[player.seat]?.name ?? '玩家'}</option>)}</select></div>}
      {!locked && own?.alive && !buyOpen && !props.room.seats[props.room.you]?.autoPlay && <button type="button" className="cs16-lock-hint" onClick={lockMouse}>点击战场锁定鼠标</button>}
      {own?.alive && <TouchControls ref={touchControls} nightVisionOwned={!!own.nightVisionOwned} nightVision={!!own.nightVision} flashlight={!!own.flashlight} disabled={!canControl()} weapon={own.weapon} weaponState={own.inventory?.find(item => item.id === own.weapon)} useLabel={props.data.bomb?.planted && own.team === 'ct' ? '拆弹' : mode === 'hostage' && own.team === 'ct' ? '营救' : '使用'} buy={props.data.buyZone && props.data.buyAllowed ? openBuy : undefined} score={() => setScoreboard(true)} native={record} begin={beginTouch} error={setError} interrupted={() => channel.current?.release(facing.current.yaw, facing.current.pitch)} changed={value => { touch.current = value; process.current(true); }} look={(x, y) => { if (!canControl()) return; facing.current.yaw = angle(facing.current.yaw - x * 144); facing.current.pitch = safePitch(facing.current.pitch + y * 96); prediction.current?.face(facing.current.yaw, facing.current.pitch); }} />}
    </>}
    {scoreboard && <Scoreboard room={props.room} view={props.data} close={() => setScoreboard(false)} />}
    {buyOpen && own && <BuyMenu player={own} prices={loaded.current?.weapons ?? []} mode={mode} allowed={!!props.data.buyAllowed} close={closeBuy} error="" buy={buyItem} category={buyCategory} choose={chooseBuy} />}
    {error && !buyOpen && !initialized.current && <button type="button" className="cs16-retry" onClick={() => setAttempt(value => value + 1)}>重新加载</button>}
  </div>;
}
