import type { NativeRuntime } from "./native-runtime";

export const predictionBudgets = { maxLeadMS: 200, maxPendingInputs: 8, normalCopyMS: 8, denseCopyMS: 25, physicsStepMS: 20 } as const;

export type PredictionOperation<Action> =
  | { type: "tick"; milliseconds: number }
  | { type: "action"; action: Action; actor: number; clientInputID?: string };

export type PredictionUpdate<State, Action> = {
  epoch: number;
  sequence: number;
  logicalMS: number;
  fromSequence?: number;
  state?: State;
  operations?: readonly PredictionOperation<Action>[];
  acceptedInputs?: readonly string[];
  verification?: unknown;
};

export type PredictionFrame<View> = {
  epoch: number;
  authoritySequence: number;
  phase: "reset" | "frame" | "input" | "reconcile";
  nativeAt: number;
  authorityMS: number;
  predictedMS: number;
  leadMS: number;
  pendingInputs: number;
  waiting: boolean;
  previousView: View;
  interpolation: number;
  copyMS: number;
};

type Input<Action> = { id: string; action: Action; atMS: number; acknowledged: boolean; coalesceKey?: string; predictionError?: string };
type Runtime<State, View, Action> = Pick<NativeRuntime<State, View, Action>, "apply" | "advance" | "project" | "clone"> & { release?: (state: State) => void };

type Configuration<State, View, Action> = {
  runtime: Runtime<State, View, Action>;
  actor: number | (() => number);
  readClock: (state: State) => number;
  readPaused?: (state: State) => boolean;
  readPhysics?: (state: State) => { step: number; interpolation: number };
  now: () => number;
  measureNow?: () => number;
  requestFrame: (callback: (at: number) => void) => number;
  cancelFrame: (id: number) => void;
  send: (action: Action, clientInputID: string) => void;
  onFrame: (view: View, metadata: PredictionFrame<View>) => void;
  requestSync: (reason: string) => void;
  onAuthority?: (view: View, metadata: { reset: boolean; confirmedInputIDs: string[]; operations: readonly PredictionOperation<Action>[] }) => void;
  onInput?: (view: View, clientInputID: string) => void;
  onReject?: (clientInputID: string, error: string) => void;
  onReplay?: (phase: "canonical" | "pending" | "idle") => void;
  onCost?: (sample: { phase: "clone" | "apply" | "advance" | "project"; milliseconds: number }) => void;
  validateAuthority?: (state: State, view: View, verification: unknown) => boolean;
};

function integer(value: number, name: string) {
  if (!Number.isSafeInteger(value) || value < 0) throw Error(`Invalid ${name}`);
  return value;
}

export function createPredictionController<State, View, Action extends Record<string, unknown>>(configuration: Configuration<State, View, Action>) {
  const { runtime, now, requestFrame, cancelFrame } = configuration;
  const measureNow = configuration.measureNow ?? now;
  let authority: State | undefined;
  let predicted: State | undefined;
  let lastView: View | undefined;
  let previousView: View | undefined;
  let stepView: View | undefined;
  let physicalStep = -1;
  let epoch = 0;
  let sequence = 0;
  let authorityMS = 0;
  let shownMS = 0;
  let anchorWallMS = 0;
  let anchorLogicalMS = 0;
  let copyMS = 0;
  let request: number | undefined;
  let previousRAF: number | undefined;
  let pausedAt: number | undefined;
  let externallyPaused = false;
  let suspended = false;
  let destroyed = false;
  let waiting = true;
  let recoveryRequested = false;
  let controllable = true;
  let pending: Input<Action>[] = [];

  const actor = () => typeof configuration.actor === "function" ? configuration.actor() : configuration.actor;
  const clock = (state: State) => integer(configuration.readClock(state), "rule clock");
  const measure = <Value>(phase: "clone" | "apply" | "advance" | "project", operation: () => Value) => {
    const started = measureNow();
    const value = operation();
    const milliseconds = Math.max(0, measureNow() - started);
    if (phase === "clone") copyMS = milliseconds;
    configuration.onCost?.({ phase, milliseconds });
    return value;
  };
  const clone = (state: State) => measure("clone", () => runtime.clone(state));
  const project = (state: State) => measure("project", () => runtime.project(state));
  const apply = (state: State, owner: number, action: Action) => measure("apply", () => runtime.apply(state, owner, action));
  const advance = (state: State, milliseconds: number) => milliseconds > 0 ? measure("advance", () => runtime.advance(state, integer(milliseconds, "advance duration"))) : state;
  const release = (state: State | undefined) => { if (state !== undefined) runtime.release?.(state); };

  function replacePredicted(next: State) {
    const previous = predicted;
    predicted = next;
    if (previous !== next && previous !== authority) release(previous);
  }

  function syncPause(at: number) {
    const paused = externallyPaused || Boolean(predicted && configuration.readPaused?.(predicted));
    if (paused) pausedAt ??= at;
    else if (pausedAt !== undefined) {
      anchorWallMS += at - pausedAt;
      pausedAt = undefined;
    }
  }

  function targetClock(at: number) {
    const elapsed = Math.max(0, (pausedAt ?? at) - anchorWallMS);
    const maximum = authorityMS + predictionBudgets.maxLeadMS;
    const overflow = anchorLogicalMS + elapsed - maximum;
    if (overflow > 0) anchorLogicalMS -= overflow;
    return Math.min(maximum, Math.max(authorityMS, shownMS, Math.floor(anchorLogicalMS + elapsed + 1e-6)));
  }

  function publish(phase: PredictionFrame<View>["phase"], at = now()) {
    if (!predicted) return;
    const view = project(predicted);
    const predictedMS = clock(predicted);
    const physics = configuration.readPhysics?.(predicted) ?? { step: Math.floor(predictedMS / predictionBudgets.physicsStepMS), interpolation: predictedMS % predictionBudgets.physicsStepMS / predictionBudgets.physicsStepMS };
    const step = physics.step;
    if (!Number.isFinite(step) || !Number.isFinite(physics.interpolation) || physics.interpolation < 0 || physics.interpolation > 1) throw Error("Invalid native physical display phase");
    if (step !== physicalStep) {
      previousView = stepView ?? view;
      physicalStep = step;
    }
    stepView = view;
    previousView ??= view;
    lastView = view;
    shownMS = Math.max(shownMS, predictedMS);
    configuration.onFrame(view, { epoch, authoritySequence: sequence, phase, nativeAt: at, authorityMS, predictedMS, leadMS: Math.max(0, predictedMS - authorityMS), pendingInputs: pending.length, waiting, previousView, interpolation: physics.interpolation, copyMS });
  }

  function schedule() {
    if (request === undefined && !destroyed && !suspended) request = requestFrame(frame);
  }

  function recover(reason: string) {
    if (recoveryRequested || destroyed) return;
    waiting = true;
    recoveryRequested = true;
    pending = [];
    if (authority !== undefined) replacePredicted(clone(authority));
    configuration.requestSync(reason);
  }

  function replay(target: number) {
    if (authority === undefined) return;
    configuration.onReplay?.("pending");
    try {
      replacePredicted(clone(authority));
      for (const input of pending) {
        const atMS = Math.min(authorityMS + predictionBudgets.maxLeadMS, Math.max(clock(predicted!), input.atMS));
        if (!externallyPaused && !configuration.readPaused?.(predicted!)) replacePredicted(advance(predicted!, atMS - clock(predicted!)));
        try {
          replacePredicted(apply(predicted!, actor(), input.action));
          input.predictionError = undefined;
        } catch (error) {
          input.predictionError = error instanceof Error ? error.message : String(error);
        }
      }
      if (!externallyPaused && !configuration.readPaused?.(predicted!)) replacePredicted(advance(predicted!, Math.max(0, Math.min(authorityMS + predictionBudgets.maxLeadMS, target) - clock(predicted!))));
    } finally {
      configuration.onReplay?.("idle");
    }
  }

  function frame(at: number) {
    request = undefined;
    if (destroyed || suspended) return;
    if (!Number.isFinite(at)) return recover("Invalid native display clock");
    if (previousRAF !== undefined && at - previousRAF > 1000) recover("Display resumed after a long pause");
    previousRAF = at;
    if (!waiting && predicted !== undefined) {
      syncPause(at);
      const target = targetClock(at);
      if (!externallyPaused && !configuration.readPaused?.(predicted)) replacePredicted(advance(predicted, Math.max(0, target - clock(predicted))));
      publish("frame", at);
    }
    schedule();
  }

  function removeAccepted(ids: readonly string[]) {
    const accepted = new Set(ids);
    const confirmed = pending.filter(input => accepted.has(input.id)).map(input => input.id);
    pending = pending.filter(input => !accepted.has(input.id));
    return confirmed;
  }

  function reset(update: PredictionUpdate<State, Action>) {
    if (destroyed) return;
    if (update.state === undefined) throw Error("Prediction reset requires an authoritative state");
    integer(update.epoch, "epoch");
    integer(update.sequence, "sequence");
    integer(update.logicalMS, "authority clock");
    if (clock(update.state) !== update.logicalMS) throw Error("Snapshot clock differs from its authority metadata");
    const nextAuthority = clone(update.state);
    let nextPredicted: State;
    try { nextPredicted = clone(nextAuthority); }
    catch (error) { release(nextAuthority); throw error; }
    const previousStates = new Set([authority, predicted]);
    authority = nextAuthority;
    predicted = nextPredicted;
    for (const previous of previousStates) if (previous !== authority && previous !== predicted) release(previous);
    epoch = update.epoch;
    sequence = update.sequence;
    authorityMS = update.logicalMS;
    pending = [];
    anchorWallMS = now();
    anchorLogicalMS = authorityMS;
    shownMS = authorityMS;
    physicalStep = -1;
    previousView = undefined;
    stepView = undefined;
    previousRAF = undefined;
    pausedAt = undefined;
    waiting = false;
    recoveryRequested = false;
    syncPause(anchorWallMS);
    const view = project(authority);
    configuration.onAuthority?.(view, { reset: true, confirmedInputIDs: [], operations: [] });
    publish("reset");
    schedule();
  }

  function receive(update: PredictionUpdate<State, Action>) {
    integer(update.epoch, "epoch");
    integer(update.sequence, "sequence");
    integer(update.logicalMS, "authority clock");
    if (destroyed || suspended) return false;
    if (update.epoch !== epoch) {
      if (update.state !== undefined) { reset(update); return true; }
      recover("Authority epoch changed");
      return false;
    }
    if (waiting) {
      if (update.state !== undefined) { reset(update); return true; }
      return false;
    }
    if (update.sequence < sequence) return false;
    if (update.sequence === sequence) {
      const confirmed = removeAccepted(update.acceptedInputs ?? []);
      if (confirmed.length) { replay(targetClock(now())); publish("reconcile"); }
      return false;
    }
    if (update.state === undefined && update.fromSequence !== sequence) {
      recover("Authority operation cursor is missing or expired");
      return false;
    }
    const at = now();
    syncPause(at);
    const target = targetClock(at);
    const operations = update.operations ?? [];
    const accepted = [...(update.acceptedInputs ?? [])];
    let candidate: State = authority!;
    const replaceCandidate = (next: State) => {
      const previous = candidate;
      candidate = next;
      if (previous !== next && previous !== authority && previous !== predicted) release(previous);
    };
    configuration.onReplay?.("canonical");
    try {
      if (update.state !== undefined) replaceCandidate(clone(update.state));
      else {
        for (const operation of operations) {
          if (operation.type === "tick") replaceCandidate(advance(candidate, integer(operation.milliseconds, "authority duration")));
          else if (operation.type === "action") {
            if (!Number.isSafeInteger(operation.actor) || operation.actor < 0) throw Error("Invalid authority actor");
            replaceCandidate(apply(candidate, operation.actor, operation.action));
          } else throw Error("Invalid authority operation");
        }
      }
      if (clock(candidate) !== update.logicalMS || update.logicalMS < authorityMS) throw Error("Authority rule clock is inconsistent");
      const canonicalView = project(candidate);
      if (update.verification !== undefined && (!configuration.validateAuthority || !configuration.validateAuthority(candidate, canonicalView, update.verification))) throw Error("Authority rule verification differs from the local replay");
      const previousAuthority = authority;
      authority = candidate;
      if (previousAuthority !== authority && previousAuthority !== predicted) release(previousAuthority);
      for (const operation of operations) if (operation.type === "action" && operation.actor === actor() && operation.clientInputID) accepted.push(operation.clientInputID);
      const confirmedInputIDs = removeAccepted(accepted);
      sequence = update.sequence;
      authorityMS = update.logicalMS;
      configuration.onAuthority?.(canonicalView, { reset: false, confirmedInputIDs, operations });
      replay(Math.max(shownMS, authorityMS, target));
      syncPause(at);
      const elapsed = Math.max(0, (pausedAt ?? at) - anchorWallMS);
      anchorLogicalMS += Math.max(0, authorityMS - anchorLogicalMS - elapsed);
      publish("reconcile", at);
    } catch (error) {
      if (candidate !== authority && candidate !== predicted) release(candidate);
      recover(error instanceof Error ? error.message : String(error));
      return false;
    } finally {
      configuration.onReplay?.("idle");
    }
    return true;
  }

  function input(action: Action, clientInputID: string, options: { coalesceKey?: string } = {}) {
    if (destroyed || suspended || waiting || !controllable || predicted === undefined || actor() < 0) return { accepted: false, error: "当前不能操作" };
    if (!/^[A-Za-z0-9_.:-]{1,128}$/.test(clientInputID) || pending.some(input => input.id === clientInputID)) return { accepted: false, error: "操作标识无效或重复" };
    const previous = pending;
    if (options.coalesceKey) pending = pending.filter(input => input.coalesceKey !== options.coalesceKey);
    if (pending.length >= predictionBudgets.maxPendingInputs) { pending = previous; return { accepted: false, error: "请等待前面的操作确认" }; }
    const at = now();
    syncPause(at);
    const target = targetClock(at);
    if (pending.length !== previous.length) replay(target);
    const normalized = { ...action, epoch } as Action;
    try {
      const next = apply(predicted!, actor(), normalized);
      const record: Input<Action> = { id: clientInputID, action: normalized, atMS: clock(predicted!), acknowledged: false, coalesceKey: options.coalesceKey };
      pending.push(record);
      replacePredicted(next);
      syncPause(at);
      publish("input", at);
      configuration.send(normalized, clientInputID);
      configuration.onInput?.(lastView!, clientInputID);
      return { accepted: true, clientInputID };
    } catch (error) {
      pending = previous.filter(input => input.id !== clientInputID);
      replay(target);
      publish("reconcile", at);
      return { accepted: false, error: error instanceof Error ? error.message : String(error) };
    }
  }

  function acknowledge(clientInputID: string, accepted: boolean) {
    const input = pending.find(input => input.id === clientInputID);
    if (!input) return false;
    if (accepted) input.acknowledged = true;
    else {
      pending = pending.filter(input => input.id !== clientInputID);
      replay(targetClock(now()));
      publish("reconcile");
      configuration.onReject?.(clientInputID, "服务器未接受此操作");
    }
    return true;
  }

  function suspend() {
    suspended = true;
    pending = [];
    if (request !== undefined) cancelFrame(request);
    request = undefined;
    waiting = true;
    recoveryRequested = false;
  }

  function resume() {
    if (destroyed) return;
    suspended = false;
    previousRAF = undefined;
    recover("Display resumed; authoritative snapshot required");
    schedule();
  }

  function sample() {
    return { epoch, sequence, authorityMS, predictedMS: predicted === undefined ? 0 : clock(predicted), leadMS: predicted === undefined ? 0 : Math.max(0, clock(predicted) - authorityMS), anchorWallMS, pendingInputs: pending.length, pending: pending.map(input => ({ id: input.id, atMS: input.atMS, acknowledged: input.acknowledged })), waiting, suspended, copyMS };
  }

  return {
    reset, receive, input, acknowledge, suspend, resume, sample,
    view: () => lastView,
    state: () => predicted,
    authorityState: () => authority,
    setPaused(value: boolean) { externallyPaused = value; syncPause(now()); },
    setControllable(value: boolean) { controllable = value; if (!value && pending.length) { pending = []; replay(targetClock(now())); publish("reconcile"); } },
    destroy() { destroyed = true; pending = []; if (request !== undefined) cancelFrame(request); request = undefined; const previous = new Set([authority, predicted]); authority = undefined; predicted = undefined; for (const state of previous) release(state); },
  };
}
