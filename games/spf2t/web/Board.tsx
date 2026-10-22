import {
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Controls } from "./Controls";
import { characterMeta, portraitSources } from "./characters";
import {
  fxFor,
  nextEvents,
  soundFor,
  type Seen,
  type SoundKey,
} from "./events";
import {
  ATLAS_CELL,
  GEM_COLORS,
  GEM_DARK,
  GEM_LIGHT,
  atlasSources,
  gemLabel,
  gemTile,
  warningClass,
} from "./gems";
import {
  DAS_DELAY,
  DAS_INTERVAL,
  InputQueue,
  repeatTimes,
  SOFT_DROP_DELAY,
  SOFT_DROP_INTERVAL,
  type InputKind,
} from "./input";
import { BGM_FILES, SOUND_FILES, soundSources } from "./sounds";
import {
  characterName,
  formatLabel,
  phaseLabel,
  scopeKey,
  type Color,
  type GameEvent,
  type GemKind,
  type GemView,
  type PairCell,
  type PlayerView,
  type View,
} from "./types";
import "./style.css";

type Props = BoardProps<View>;

type Atlas = { ok: boolean; w: number; h: number; src: string };

type Fx = {
  key: number;
  seat: number;
  type: string;
  cells: { x: number; y: number }[];
};

const useAtlas = (assets: string): Atlas => {
  const [state, setState] = useState<Atlas>({ ok: false, w: 0, h: 0, src: "" });
  useEffect(() => {
    let cancelled = false;
    const urls = atlasSources(assets);
    const attempt = (index: number) => {
      if (cancelled) return;
      if (index >= urls.length) {
        setState({ ok: false, w: 0, h: 0, src: "" });
        return;
      }
      const image = new Image();
      image.onload = () => {
        if (!cancelled)
          setState({
            ok: true,
            w: image.naturalWidth,
            h: image.naturalHeight,
            src: urls[index],
          });
      };
      image.onerror = () => attempt(index + 1);
      image.src = urls[index];
    };
    attempt(0);
    return () => {
      cancelled = true;
    };
  }, [assets]);
  return state;
};

const useServerNow = (clock: Props["clock"], active: boolean) => {
  const [now, setNow] = useState(
    () =>
      clock.serverNow +
      (clock.paused ? 0 : Math.max(0, performance.now() - clock.receivedAt)),
  );
  useEffect(() => {
    setNow(
      clock.serverNow +
        (clock.paused ? 0 : Math.max(0, performance.now() - clock.receivedAt)),
    );
    if (!active) return;
    const id = setInterval(
      () =>
        setNow(
          clock.serverNow +
            (clock.paused
              ? 0
              : Math.max(0, performance.now() - clock.receivedAt)),
        ),
      250,
    );
    return () => clearInterval(id);
  }, [clock.serverNow, clock.receivedAt, active]);
  return now;
};

const timeText = (ms: number) => {
  const total = Math.max(0, Math.ceil(ms / 1000));
  return `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(
    total % 60,
  ).padStart(2, "0")}`;
};

const gemStyle = (
  x: number,
  y: number,
  w: number,
  h: number,
  tile: { x: number; y: number } | null,
  atlas: Atlas,
): CSSProperties => {
  const style: CSSProperties = {
    left: `calc(var(--cell) * ${x})`,
    top: `calc(var(--cell) * ${y})`,
    width: `calc(var(--cell) * ${w})`,
    height: `calc(var(--cell) * ${h})`,
  };
  if (atlas.ok && tile) {
    style.backgroundImage = `url("${atlas.src}")`;
    style.backgroundSize = `calc(var(--cell) * ${atlas.w / ATLAS_CELL}) calc(var(--cell) * ${
      atlas.h / ATLAS_CELL
    })`;
    style.backgroundPosition = `calc(var(--cell) * ${
      -tile.x / ATLAS_CELL
    }) calc(var(--cell) * ${-tile.y / ATLAS_CELL})`;
  }
  return style;
};

function GemFace({
  kind,
  color,
  timer,
  atlas,
  small = false,
}: {
  kind: GemKind;
  color?: Color;
  timer?: number;
  atlas: Atlas;
  small?: boolean;
}) {
  const tile = gemTile(kind, color);
  const style: CSSProperties = {};
  if (atlas.ok && tile) {
    style.backgroundImage = `url("${atlas.src}")`;
    style.backgroundSize = small
      ? `${(atlas.w / ATLAS_CELL) * 16}px ${(atlas.h / ATLAS_CELL) * 16}px`
      : `calc(var(--cell) * ${atlas.w / ATLAS_CELL}) calc(var(--cell) * ${
          atlas.h / ATLAS_CELL
        })`;
    style.backgroundPosition = small
      ? `${(-tile.x / ATLAS_CELL) * 16}px ${(-tile.y / ATLAS_CELL) * 16}px`
      : `calc(var(--cell) * ${-tile.x / ATLAS_CELL}) calc(var(--cell) * ${
          -tile.y / ATLAS_CELL
        })`;
  }
  return (
    <span
      className={`spf2t-gem${small ? " is-mini" : ""}${
        kind === "power" ? " is-power" : ""
      }${atlas.ok ? " has-atlas" : ""}`}
      data-kind={kind}
      data-color={color}
      style={style}
      aria-hidden="true"
    >
      {kind === "counter" && timer ? <b>{timer}</b> : null}
      {kind === "crash" && !atlas.ok ? <i aria-hidden="true" /> : null}
      {kind === "diamond" && !atlas.ok ? <i aria-hidden="true" /> : null}
      {!atlas.ok && kind !== "diamond" && color ? (
        <em
          aria-hidden="true"
          style={{
            background: `linear-gradient(135deg, ${GEM_LIGHT[color]}, ${
              GEM_COLORS[color]
            } 55%, ${GEM_DARK[color]})`,
          }}
        />
      ) : null}
      {kind === "diamond" && !atlas.ok ? (
        <em
          aria-hidden="true"
          style={{
            background:
              "conic-gradient(from 25deg, #e78a8a, #e8d27a, #7fca8c, #7fb7e8, #c18ad8, #e78a8a)",
          }}
        />
      ) : null}
    </span>
  );
}

const Gem = memo(function Gem({ gem, atlas }: { gem: GemView; atlas: Atlas }) {
  if (gem.kind === "power" && gem.color) {
    const cells = Array.from(
      { length: Math.max(1, gem.w * gem.h) },
      (_, i) => i,
    );
    return (
      <span
        className={`spf2t-power ${atlas.ok ? "has-atlas" : ""}`}
        style={gemStyle(
          gem.x,
          gem.y,
          gem.w,
          gem.h,
          gemTile("power", gem.color),
          atlas,
        )}
        data-kind="power"
        data-color={gem.color}
        title={gemLabel(gem)}
        aria-hidden="true"
      >
        {cells.map((cell) => {
          const style = gemStyle(
            cell % gem.w,
            Math.floor(cell / gem.w),
            1,
            1,
            atlas.ok ? gemTile("power", gem.color) : null,
            atlas,
          );
          if (!atlas.ok) style.background = GEM_COLORS[gem.color!];
          return (
            <span
              key={cell}
              className="spf2t-power-cell"
              data-color={gem.color}
              style={style}
            />
          );
        })}
      </span>
    );
  }
  return (
    <span
      className="spf2t-cell-wrap"
      style={gemStyle(
        gem.x,
        gem.y,
        gem.w,
        gem.h,
        gemTile(gem.kind, gem.color),
        atlas,
      )}
      data-kind={gem.kind}
      data-color={gem.color}
      title={gemLabel(gem)}
      aria-hidden="true"
    >
      <GemFace
        kind={gem.kind}
        color={gem.color}
        timer={gem.timer}
        atlas={atlas}
      />
    </span>
  );
});

function Portrait({
  id,
  name,
  assets,
  className = "",
}: {
  id?: string;
  name: string;
  assets: string;
  className?: string;
}) {
  const meta = characterMeta(id);
  const sources = portraitSources(assets, id ?? "");
  const [index, setIndex] = useState(0);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setIndex(0);
    setFailed(false);
  }, [id, assets]);
  if (failed || !id || sources.length === 0) {
    return (
      <span
        className={`spf2t-portrait is-fallback ${className}`}
        style={{
          background: "#1b1e24",
          color: "#f5f0e6",
          borderColor: meta.accent,
        }}
        aria-hidden="true"
      >
        {name.slice(0, 1)}
      </span>
    );
  }
  return (
    <img
      className={`spf2t-portrait ${className}`}
      src={sources[index]}
      alt=""
      aria-hidden="true"
      draggable={false}
      onError={() => {
        if (index + 1 < sources.length) setIndex(index + 1);
        else setFailed(true);
      }}
    />
  );
}

function Warning({ pending }: { pending: number }) {
  if (pending <= 0) return null;
  const label = pending <= 10 ? "注意" : pending <= 30 ? "警告" : "危险";
  return (
    <span
      className={`spf2t-warning ${warningClass(pending)}`}
      aria-label={`待落反击宝石 ${pending}，${label}`}
    >
      {label} {pending}
    </span>
  );
}

function WinPips({ wins, target }: { wins: number; target: number }) {
  return (
    <span className="spf2t-wins" aria-label={`${wins} 回合胜场`}>
      {Array.from({ length: target }, (_, i) => (
        <i key={i} className={i < wins ? "is-on" : ""} aria-hidden="true" />
      ))}
    </span>
  );
}

function NextPair({ player, atlas }: { player: PlayerView; atlas: Atlas }) {
  const specs = [player.next?.a, player.next?.b].filter(Boolean);
  return (
    <div className="spf2t-next" aria-label="下一对宝石">
      <span>下一对</span>
      <div>
        {specs.map((spec, index) => (
          <GemFace
            key={index}
            kind={spec.kind}
            color={spec.color}
            atlas={atlas}
            small
          />
        ))}
      </div>
    </div>
  );
}

function BoardPanel({
  player,
  name,
  self,
  data,
  assets,
  atlas,
  fx,
}: {
  player: PlayerView;
  name: string;
  self: boolean;
  data: View;
  assets: string;
  atlas: Atlas;
  fx: Fx[];
}) {
  const charName = player.selected
    ? characterName(data, player.character)
    : "选择角色中";
  const active = player.active;
  const gems = player.gems ?? [];
  return (
    <section
      className={`spf2t-panel${self ? " is-self" : ""}${
        player.lost ? " is-lost" : ""
      }`}
      data-spf2t-seat={player.seat}
      data-spf2t-active-x={player.active?.x ?? ""}
      data-spf2t-active-rot={player.active?.rot ?? ""}
      data-spf2t-active-id={player.active?.id ?? ""}
      data-spf2t-seq={player.seq ?? ""}
      data-spf2t-pending={player.pendingCount}
      data-spf2t-round-wins={player.roundWins}
      data-spf2t-resolving={player.resolving ? "1" : "0"}
      data-spf2t-lost={player.lost ? "1" : "0"}
      data-spf2t-gems={gems.length}
      aria-label={`${name}${self ? "（你）" : ""}的棋盘`}
    >
      <header className="spf2t-panel-head">
        <Portrait id={player.character} name={charName} assets={assets} />
        <div className="spf2t-who">
          <strong>
            {name}
            {self ? " · 你" : ""}
          </strong>
          <small>{charName}</small>
        </div>
        <WinPips wins={player.roundWins} target={data.roundsToWin} />
      </header>
      <div
        className="spf2t-board"
        role="img"
        aria-label={`${name}的棋盘，${gems.length} 组宝石${
          player.pendingCount ? `，待落反击 ${player.pendingCount}` : ""
        }`}
      >
        {gems.map((gem) => (
          <Gem key={gem.id} gem={gem} atlas={atlas} />
        ))}
        {active
          ? active.cells.map((cell, index) => (
              <span
                key={`active-${active.id}-${index}`}
                className="spf2t-cell-wrap is-active"
                style={gemStyle(
                  cell.x,
                  cell.y,
                  1,
                  1,
                  gemTile(cell.kind, cell.color),
                  atlas,
                )}
                data-kind={cell.kind}
                data-color={cell.color}
                aria-hidden="true"
              >
                <GemFace kind={cell.kind} color={cell.color} atlas={atlas} />
              </span>
            ))
          : null}
        {self && active?.ghost
          ? active.ghost.map((cell, index) => (
              <span
                key={`ghost-${active.id}-${index}`}
                className="spf2t-ghost"
                style={gemStyle(cell.x, cell.y, 1, 1, null, atlas)}
                aria-hidden="true"
              />
            ))
          : null}
        {fx.map((effect) =>
          effect.cells.map((cell, index) => (
            <span
              key={`fx-${effect.key}-${index}`}
              className={`spf2t-flash is-${effect.type}`}
              style={gemStyle(cell.x, cell.y, 1, 1, null, atlas)}
              aria-hidden="true"
            />
          )),
        )}
        {player.resolving ? (
          <span className="spf2t-board-state" aria-live="polite">
            连锁处理中
          </span>
        ) : null}
        {player.lost ? (
          <span className="spf2t-board-state is-lost">本回合失败</span>
        ) : null}
      </div>
      <div className="spf2t-panel-foot">
        <NextPair player={player} atlas={atlas} />
        <Warning pending={player.pendingCount} />
      </div>
    </section>
  );
}

function PatternStrip({ pattern }: { pattern?: string[] }) {
  if (!pattern?.length) return null;
  const rows = pattern.slice(0, 3);
  return (
    <span className="spf2t-pattern" aria-label="反击宝石图案预览">
      {rows.map((row, y) => (
        <span key={y}>
          {row.split("").map((color, x) => (
            <i
              key={x}
              style={{ background: GEM_COLORS[color as Color] ?? "#555" }}
              aria-hidden="true"
            />
          ))}
        </span>
      ))}
    </span>
  );
}

function Center({
  data,
  names,
  assets,
}: {
  data: View;
  names: string[];
  assets: string;
}) {
  const winner =
    data.phase === "round_break"
      ? data.players.find((p) => !p.lost)
      : undefined;
  return (
    <section className="spf2t-center" aria-label="对局信息">
      {data.players.map((player, index) => (
        <div
          key={player.seat}
          className={`spf2t-center-player${player.lost ? " is-lost" : ""}`}
        >
          <Portrait
            id={player.character}
            name={
              player.selected
                ? characterName(data, player.character)
                : names[index]
            }
            assets={assets}
            className="is-large"
          />
          <strong>
            {player.selected ? characterName(data, player.character) : "选角中"}
          </strong>
          <span>{names[index]}</span>
          <PatternStrip pattern={player.pattern} />
        </div>
      ))}
      {data.phase === "round_break" && winner ? (
        <p className="spf2t-center-note">
          第 {data.round} 回合 · {names[winner.seat]} 获胜
        </p>
      ) : null}
    </section>
  );
}

function SelectOverlay({
  data,
  room,
  assets,
  busy,
  act,
  now,
}: {
  data: View;
  room: Props["room"];
  assets: string;
  busy: boolean;
  act: Props["act"];
  now: number;
}) {
  const self = data.players[room.you];
  const spectating = !!room.spectating || room.you < 0;
  const remaining = Math.max(0, Math.ceil((data.selectUntil - now) / 1000));
  const [pending, setPending] = useState<string | null>(null);
  useEffect(() => setPending(null), [data.epoch]);
  const choose = (id: string) => {
    if (spectating || busy || self?.selected || pending) return;
    setPending(id);
    void act({ type: "select", epoch: data.epoch, character: id }).catch(() =>
      setPending(null),
    );
  };
  return (
    <div
      className="spf2t-overlay spf2t-select"
      role="dialog"
      aria-label="选择角色"
    >
      <div className="spf2t-overlay-card">
        <header>
          <strong>
            {spectating
              ? "观战 · 双方选角"
              : self?.selected
                ? "已选择，等待对手"
                : "选择角色"}
          </strong>
          <span aria-live="polite">剩余 {remaining} 秒</span>
        </header>
        <div className="spf2t-picks" aria-live="polite">
          <span>
            {spectating ? "蓝方" : "你"}：
            {self?.selected ? characterName(data, self.character) : "未选择"}
          </span>
          <span>
            对手：
            {data.players[1 - (room.you < 0 ? 0 : room.you)]?.selected
              ? characterName(data, data.players[1 - room.you]?.character)
              : "未选择"}
          </span>
        </div>
        <div className="spf2t-cards">
          {data.characters.map((character) => {
            const meta = characterMeta(character.id);
            const picked =
              self?.character === character.id || pending === character.id;
            const rivalPicked = data.players.some(
              (p) => p.seat !== room.you && p.character === character.id,
            );
            return (
              <button
                key={character.id}
                type="button"
                className={`spf2t-card${picked ? " is-picked" : ""}${
                  rivalPicked ? " is-rival" : ""
                }`}
                data-character={character.id}
                disabled={spectating || busy || !!self?.selected || !!pending}
                onClick={() => choose(character.id)}
                aria-pressed={picked}
              >
                <Portrait
                  id={character.id}
                  name={character.name}
                  assets={assets}
                />
                <strong>{character.name}</strong>
                <PatternStrip pattern={character.pattern} />
                {rivalPicked ? <small>对手已选</small> : null}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function CountdownOverlay({ data, now }: { data: View; now: number }) {
  const remaining = Math.max(0, Math.ceil((data.startAt - now) / 1000));
  return (
    <div className="spf2t-overlay spf2t-countdown" aria-live="polite">
      <div>
        <strong>{remaining > 0 ? remaining : "开始"}</strong>
        <span>双方准备</span>
      </div>
    </div>
  );
}

function RoundBreakOverlay({
  data,
  names,
  now,
}: {
  data: View;
  names: string[];
  now: number;
}) {
  const remaining = Math.max(0, Math.ceil((data.breakUntil - now) / 1000));
  const lost = data.players.filter((p) => p.lost);
  const text =
    lost.length === 2
      ? "双方同时顶出，本回合重赛"
      : lost.length === 1
        ? `${names[1 - lost[0].seat]} 赢下本回合`
        : "回合结束";
  return (
    <div className="spf2t-overlay spf2t-round" aria-live="polite">
      <div>
        <strong>第 {data.round} 回合</strong>
        <span>{text}</span>
        <small>
          {remaining} 秒后继续 · {data.players[0].roundWins} :{" "}
          {data.players[1].roundWins}
        </small>
      </div>
    </div>
  );
}

const keyMap: Record<string, { kind: InputKind; held: boolean }> = {
  ArrowLeft: { kind: "left", held: true },
  KeyA: { kind: "left", held: true },
  ArrowRight: { kind: "right", held: true },
  KeyD: { kind: "right", held: true },
  ArrowUp: { kind: "rotateCW", held: false },
  KeyX: { kind: "rotateCW", held: false },
  KeyZ: { kind: "rotateCCW", held: false },
  ControlLeft: { kind: "rotateCCW", held: false },
  ControlRight: { kind: "rotateCCW", held: false },
  ArrowDown: { kind: "softDrop", held: true },
  KeyS: { kind: "softDrop", held: true },
  Space: { kind: "hardDrop", held: false },
};

const ignoredTarget = (target: EventTarget | null) => {
  const element = target as HTMLElement | null;
  if (!element) return false;
  const tag = element.tagName;
  return (
    tag === "INPUT" ||
    tag === "TEXTAREA" ||
    tag === "SELECT" ||
    element.isContentEditable
  );
};

export function Board({
  room,
  data,
  act,
  busy,
  assets,
  audio,
  clock,
  inputWindow = 1,
}: Props) {
  const atlas = useAtlas(assets);
  const self = room.you >= 0 ? data.players[room.you] : undefined;
  const selfSeq = self?.seq ?? 0;
  const hosted = !!room.seats[room.you]?.autoPlay;
  const canPlay =
    !!self &&
    data.controls.canPlay &&
    data.phase === "playing" &&
    !hosted &&
    !busy;
  const now = useServerNow(clock, data.phase !== "finished");
  const scope = `${scopeKey(room.id, data, room.you)}:${clock.epoch ?? 0}`;
  const inputScope = useRef({ scope, seq: selfSeq });
  inputScope.current = { scope, seq: selfSeq };
  const queue = useRef<InputQueue>(new InputQueue(selfSeq));
  const sending = useRef(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const [fx, setFx] = useState<Fx[]>([]);
  const fxTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const seen = useRef<Seen>({ scope: "", id: 0 });

  useEffect(() => {
    queue.current.reset(selfSeq);
    sending.current = false;
    setSendError(null);
    setFx([]);
  }, [scope]);

  useEffect(() => {
    queue.current.acknowledge(selfSeq);
  }, [selfSeq]);

  useEffect(() => {
    if (!canPlay) queue.current.clearPending();
  }, [canPlay]);

  useEffect(() => {
    if (!canPlay) return;
    const id = setInterval(() => {
      if ((sending.current && inputWindow <= 1) || document.hidden) return;
      const batch = queue.current.take();
      if (!batch) return;
      sending.current = true;
      const sentScope = scope;
      void act({
        type: "inputs",
        epoch: data.epoch,
        seq: batch.seq,
        events: batch.events,
      })
        .catch((error: unknown) => {
          if (inputScope.current.scope !== sentScope) return;
          queue.current.reset(inputScope.current.seq);
          setSendError(
            error instanceof TypeError
              ? "网络连接中断，正在恢复操作"
              : error instanceof Error
                ? error.message
                : String(error),
          );
        })
        .finally(() => {
          if (inputScope.current.scope === sentScope) sending.current = false;
        });
    }, 40);
    return () => clearInterval(id);
  }, [canPlay, scope, data.epoch, selfSeq, act, inputWindow]);

  useEffect(() => {
    if (!canPlay) return;
    const held = new Map<
      string,
      { kind: InputKind; since: number; sent: number }
    >();
    const down = (event: KeyboardEvent) => {
      if (ignoredTarget(event.target)) return;
      if (
        event.code === "Space" &&
        (event.target as HTMLElement | null)?.tagName === "BUTTON"
      )
        return;
      const mapped = keyMap[event.code];
      if (!mapped) return;
      event.preventDefault();
      if (event.repeat) return;
      queue.current.push(mapped.kind);
      if (mapped.held) {
        const delay = mapped.kind === "softDrop" ? SOFT_DROP_DELAY : DAS_DELAY;
        held.set(event.code, {
          kind: mapped.kind,
          since: performance.now() + delay,
          sent: 0,
        });
      }
    };
    const up = (event: KeyboardEvent) => {
      held.delete(event.code);
    };
    const clear = () => held.clear();
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", clear);
    document.addEventListener("visibilitychange", clear);
    const timer = setInterval(() => {
      const time = performance.now();
      const intervalKind = (kind: InputKind) =>
        kind === "softDrop" ? SOFT_DROP_INTERVAL : DAS_INTERVAL;
      for (const state of held.values()) {
        const due = repeatTimes(state.since, time, 0, intervalKind(state.kind));
        if (due > state.sent) {
          for (let i = state.sent; i < due; i++) queue.current.push(state.kind);
          state.sent = due;
        }
      }
    }, 16);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", clear);
      document.removeEventListener("visibilitychange", clear);
      clearInterval(timer);
      held.clear();
    };
  }, [canPlay, scope]);

  useEffect(() => {
    const file = data.phase === "select" ? BGM_FILES.select : BGM_FILES.battle;
    if (
      data.phase === "select" ||
      data.phase === "countdown" ||
      data.phase === "playing" ||
      data.phase === "round_break"
    ) {
      void audio.music(soundSources(assets, file));
    } else {
      audio.stopMusic();
    }
    return () => audio.stopMusic();
  }, [data.phase, data.epoch, assets, audio]);

  useEffect(() => {
    const next = nextEvents(data.events, seen.current, scope);
    seen.current = next.seen;
    if (!next.events.length) return;
    const won = room.game?.winners?.includes(room.you) ?? false;
    for (const event of next.events) {
      const key = soundFor(event, room.you, !!room.spectating, won);
      if (key) {
        void audio.playWithFallback(
          soundSources(assets, SOUND_FILES[key as SoundKey]),
        );
      }
      const effect = fxFor(event);
      if (effect) {
        setFx((previous) =>
          [...previous, { key: event.id, seat: event.seat, ...effect }].slice(
            -10,
          ),
        );
      }
    }
    if (fxTimer.current) clearTimeout(fxTimer.current);
    fxTimer.current = setTimeout(() => setFx([]), 280);
  }, [
    data.events,
    data.epoch,
    scope,
    room.you,
    room.spectating,
    room.game?.winners,
    assets,
    audio,
  ]);

  useEffect(
    () => () => {
      if (fxTimer.current) clearTimeout(fxTimer.current);
    },
    [],
  );

  useEffect(() => () => audio.stop(), [audio]);

  const pushInput = useCallback(
    (kind: InputKind) => {
      if (canPlay) queue.current.push(kind);
    },
    [canPlay],
  );

  const names = useMemo(
    () =>
      data.players.map(
        (player) => room.seats[player.seat]?.name ?? `玩家 ${player.seat + 1}`,
      ),
    [data.players, room.seats],
  );
  const matchLeft = data.matchEndsAt > 0 ? data.matchEndsAt - now : 0;
  const format = data.options.format;
  const phase = data.phase;
  const spectator = !!room.spectating || room.you < 0;

  return (
    <div
      className="spf2t-game"
      data-spf2t-phase={phase}
      data-spf2t-format={format}
      data-spf2t-round={data.round}
      data-spf2t-epoch={data.epoch}
    >
      <header className="spf2t-hud">
        <div>
          <h2>街霸方块</h2>
          <span>
            {formatLabel(format)} · 第 {data.round} 回合
          </span>
        </div>
        <div className="spf2t-hud-right">
          {spectator ? <span className="spf2t-badge">观战 · 只读</span> : null}
          <span className="spf2t-badge">{phaseLabel(phase)}</span>
          <span className="spf2t-clock" aria-label="本局剩余时间">
            {phase === "playing" || phase === "round_break"
              ? timeText(matchLeft)
              : "--:--"}
          </span>
        </div>
      </header>
      {sendError ? (
        <div className="spf2t-notice" role="status">
          {sendError}
          <button type="button" onClick={() => setSendError(null)}>
            继续
          </button>
        </div>
      ) : null}
      <div className="spf2t-stage">
        <BoardPanel
          player={data.players[0]}
          name={names[0]}
          self={room.you === 0}
          data={data}
          assets={assets}
          atlas={atlas}
          fx={fx.filter((effect) => effect.seat === 0)}
        />
        <Center data={data} names={names} assets={assets} />
        <BoardPanel
          player={data.players[1]}
          name={names[1]}
          self={room.you === 1}
          data={data}
          assets={assets}
          atlas={atlas}
          fx={fx.filter((effect) => effect.seat === 1)}
        />
      </div>
      {phase === "select" ? (
        <SelectOverlay
          data={data}
          room={room}
          assets={assets}
          busy={busy}
          act={act}
          now={now}
        />
      ) : null}
      {phase === "countdown" ? (
        <CountdownOverlay data={data} now={now} />
      ) : null}
      {phase === "round_break" ? (
        <RoundBreakOverlay data={data} names={names} now={now} />
      ) : null}
      {!spectator && self ? (
        <div className="spf2t-self-controls">
          <div className="spf2t-self-status">
            <span>
              {self.selected
                ? `${characterName(data, self.character)} · 待落 ${self.pendingCount}`
                : "等待选角"}
            </span>
            <span>
              回合 {self.roundWins} / {data.roundsToWin}
            </span>
          </div>
          <Controls disabled={!canPlay} onInput={pushInput} />
        </div>
      ) : null}
    </div>
  );
}
