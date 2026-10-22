import {
  selectPokerSound,
  type PokerSoundMapping,
  type PokerEvent,
} from "./audio-events";
export type { PokerEvent } from "./audio-events";
import { nextFeedbackEvents } from "./events";
// Rule-free presentation only: physical card faces, preferences, gestures and
// relative seats. Classification and legal decisions belong to each game.
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import type { GameRoom, GameAudioPort } from "../game-api/module";
import { Modal } from "../ui/board-host";
import "./style.css";
export const skins = [
  { id: "english", name: "传统英式" },
  { id: "illustrated", name: "轻松插画" },
  { id: "fourcolor", name: "大字四色" },
];
export const backs = [
  { id: "pine", name: "松绿菱纹" },
  { id: "claret", name: "酒红菱纹" },
  { id: "ink", name: "靛蓝菱纹" },
];
export function rankName(n: number) {
  return (
    ({ 11: "J", 12: "Q", 13: "K", 14: "A" } as Record<number, string>)[n] ??
    String(n)
  );
}
export function cardLabel(id: number) {
  const b = id % 54;
  return b >= 52
    ? b === 52
      ? "小王"
      : "大王"
    : ["梅花", "方块", "红心", "黑桃"][Math.floor(b / 13)] +
        rankName((b % 13) + 2);
}
export function cardRank(id: number) {
  const b = id % 54;
  return b >= 52 ? b - 36 : (b % 13) + 2;
}
function preference(key: string, choices: { id: string }[]) {
  try {
    const v = localStorage.getItem(key);
    return choices.some((c) => c.id === v) ? v! : choices[0].id;
  } catch {
    return choices[0].id;
  }
}
export function useCardStyle() {
  const [skin, setSkin] = useState(() => preference("wb_poker_skin", skins));
  const [back, setBack] = useState(() => preference("wb_poker_back", backs));
  useEffect(() => {
    const update = () => {
      setSkin(preference("wb_poker_skin", skins));
      setBack(preference("wb_poker_back", backs));
    };
    addEventListener("storage", update);
    addEventListener("poker-style", update);
    return () => {
      removeEventListener("storage", update);
      removeEventListener("poker-style", update);
    };
  }, []);
  return {
    skin,
    back,
    setSkin: (v: string) => {
      setSkin(v);
      try {
        localStorage.setItem("wb_poker_skin", v);
        dispatchEvent(new Event("poker-style"));
      } catch {}
    },
    setBack: (v: string) => {
      setBack(v);
      try {
        localStorage.setItem("wb_poker_back", v);
        dispatchEvent(new Event("poker-style"));
      } catch {}
    },
  };
}
export function mediaSources(base: string, path: string) {
  const local = "/poker/v1/" + path;
  return base ? [base.replace(/\/$/, "") + local, local] : [local];
}
export function assetSources(base: string, path: string) {
  return base ? [base.replace(/\/$/, "") + path, path] : [path];
}
export function Card({
  id,
  skin,
  assets,
  back = "pine",
}: {
  id: number;
  skin: string;
  assets: string;
  back?: string;
}) {
  const path =
    id < 0
      ? `/poker/v1/backs/${back}.webp`
      : `/poker/v4/${skin}/${id % 54}.webp`;
  const sources = assetSources(assets, path);
  const [failed, setFailed] = useState<string[]>([]);
  const src = sources.find((value) => !failed.includes(value));
  const b = id % 54,
    suit = Math.floor(b / 13),
    red = suit === 1 || suit === 2 || b === 53;
  return (
    <span className="poker-card" data-card-face={id} aria-hidden="true">
      {src ? (
        <img
          src={src}
          alt=""
          draggable={false}
          onError={() => setFailed((old) => [...old, src])}
        />
      ) : id < 0 ? (
        <span className="poker-back-fallback">◇</span>
      ) : (
        <span
          className="poker-face-fallback"
          style={{ color: red ? "#a12730" : "#202b29" }}
        >
          {b >= 52 ? (
            <b className="poker-joker-label">JOKER</b>
          ) : (
            <>
              <b>{rankName((b % 13) + 2)}</b>
              <span>{["♣", "♦", "♥", "♠"][suit]}</span>
            </>
          )}
        </span>
      )}
    </span>
  );
}
export function CardStrip({
  ids,
  skin,
  assets,
  back,
  singleRow = false,
}: {
  ids: number[];
  singleRow?: boolean;
  skin: string;
  assets: string;
  back?: string;
}) {
  const ref = useRef<HTMLDivElement>(null),
    [width, setWidth] = useState(160),
    [mobile, setMobile] = useState(
      () => typeof window !== "undefined" && innerWidth <= 600,
    );
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const observer = new ResizeObserver(() => {
      setWidth(el.clientWidth);
      setMobile(innerWidth <= 600);
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  const size = mobile ? 44 : 52,
    step = mobile ? 20 : 24,
    perRow = singleRow
      ? Math.max(1, ids.length)
      : Math.max(1, Math.floor((width - size) / step) + 1);
  const rows = [];
  for (let i = 0; i < ids.length; i += perRow)
    rows.push(ids.slice(i, i + perRow));
  return (
    <div
      className={`poker-card-strip${singleRow ? " poker-card-strip-single" : ""}`}
      ref={ref}
      aria-label={ids.map(cardLabel).join("、")}
      style={
        {
          "--strip-card": size + "px",
          "--strip-step": step + "px",
        } as CSSProperties
      }
    >
      {rows.map((row, i) => (
        <div
          className="poker-strip-row"
          key={i}
          style={{ width: size + (row.length - 1) * step }}
        >
          {row.map((id, j) => (
            <span
              className="poker-strip-slot"
              key={id + "-" + j}
              style={{ left: j * step, zIndex: j }}
            >
              <Card id={id} skin={skin} assets={assets} back={back} />
            </span>
          ))}
        </div>
      ))}
    </div>
  );
}
export function useSelection(hand: number[], windowKey: string) {
  const [selected, setSelected] = useState<number[]>([]);
  const signature = hand.join(",");
  useEffect(() => setSelected([]), [windowKey]);
  useEffect(
    () => setSelected((s) => s.filter((id) => hand.includes(id))),
    [signature],
  );
  return { selected, setSelected };
}
export function Hand({
  cards,
  capacity,
  selected,
  onChange,
  seat,
  skin,
  assets,
  highlights = {},
  disabled = false,
}: {
  cards: number[];
  capacity: number;
  selected: number[];
  onChange: (ids: number[]) => void;
  seat: number;
  skin: string;
  assets: string;
  highlights?: Record<number, string>;
  disabled?: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(300);
  const gesture = useRef<{
    x: number;
    id: number;
    drag: boolean;
    select: boolean;
    baseline: number[];
  } | null>(null);
  useLayoutEffect(() => {
    const e = ref.current;
    if (!e) return;
    const o = new ResizeObserver(() => setWidth(e.clientWidth));
    o.observe(e);
    return () => o.disconnect();
  }, []);
  const cardWidth = width < 420 ? 54 : 68,
    step = width < 420 ? 22 : 30;
  const perRow = Math.max(1, Math.floor((width - cardWidth) / step) + 1);
  const reservedRows = Math.ceil(Math.max(capacity, cards.length) / perRow);
  const filledRows = Math.max(1, Math.ceil(cards.length / perRow));
  const rows = [];
  for (let i = 0; i < cards.length; i += perRow)
    rows.push(cards.slice(i, i + perRow));
  const update = (id: number, select: boolean) =>
    onChange(
      select
        ? selected.includes(id)
          ? selected
          : [...selected, id]
        : selected.filter((c) => c !== id),
    );
  return (
    <div
      className="poker-hand"
      style={{
        paddingTop: 16 + (reservedRows - filledRows) * cardWidth * 0.9,
        minHeight:
          32 +
          Math.max(0, reservedRows - 1) * cardWidth * 0.9 +
          cardWidth * 1.4,
      }}
      ref={ref}
      data-seat={seat}
      aria-label="你的手牌"
      aria-keyshortcuts="Escape"
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.stopPropagation();
          onChange([]);
        }
      }}
      onPointerDownCapture={(e) => {
        if (!(e.target as HTMLElement).closest("button[data-card]"))
          onChange([]);
      }}
      onPointerMove={(e) => {
        const g = gesture.current;
        if (!g) return;
        if (Math.abs(e.clientX - g.x) > 7) g.drag = true;
        if (!g.drag) return;
        const hit = document
          .elementFromPoint(e.clientX, e.clientY)
          ?.closest<HTMLButtonElement>("button[data-card]");
        if (!hit || !ref.current?.contains(hit)) return;
        const id = Number(hit.dataset.card),
          a = cards.indexOf(g.id),
          b = cards.indexOf(id);
        if (a < 0 || b < 0) return;
        const range = cards.slice(Math.min(a, b), Math.max(a, b) + 1);
        onChange(
          g.select
            ? [...new Set([...g.baseline, ...range])]
            : g.baseline.filter((v) => !range.includes(v)),
        );
      }}
      onPointerUp={(e) => {
        const g = gesture.current;
        if (g && !g.drag) update(g.id, g.select);
        gesture.current = null;
        if (e.currentTarget.hasPointerCapture(e.pointerId))
          e.currentTarget.releasePointerCapture(e.pointerId);
      }}
      onPointerCancel={() => {
        gesture.current = null;
      }}
    >
      {rows.map((row, i) => (
        <div
          className="poker-hand-row"
          key={i}
          style={
            {
              "--card-width": `${cardWidth}px`,
              "--step": `${step}px`,
              width: cardWidth + (row.length - 1) * step,
            } as CSSProperties
          }
        >
          {row.map((id, j) => (
            <button
              type="button"
              key={id}
              data-card={id}
              className={selected.includes(id) ? "chosen" : ""}
              aria-label={`${cardLabel(id)}${highlights[id] ? " · " + highlights[id] : ""} · 第 ${Math.floor(id / 54) + 1} 副`}
              aria-pressed={selected.includes(id)}
              disabled={disabled}
              style={{ left: j * step, zIndex: j }}
              onPointerDown={(e) => {
                if (e.button !== 0 || disabled) return;
                gesture.current = {
                  x: e.clientX,
                  id,
                  drag: false,
                  select: !selected.includes(id),
                  baseline: [...selected],
                };
                ref.current?.setPointerCapture(e.pointerId);
              }}
              onClick={(e) => {
                if (e.detail === 0) update(id, !selected.includes(id));
              }}
            >
              <Card id={id} skin={skin} assets={assets} />
            </button>
          ))}
        </div>
      ))}
      {!cards.length && <p className="poker-empty-hand">手牌已出完</p>}
    </div>
  );
}
export function usePokerFeedback(
  events: PokerEvent[],
  roomID: string,
  audio: GameAudioPort,
  assets: string,
  mapping: PokerSoundMapping,
  clock: { serverNow: number; receivedAt: number },
) {
  const last = useRef({
    room: roomID,
    id: Math.max(0, ...events.map((e) => e.id)),
  });
  const [fresh, setFresh] = useState<number[]>([]);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  useEffect(() => {
    const latest = Math.max(0, ...events.map((e) => e.id));
    if (last.current.room !== roomID) {
      last.current = { room: roomID, id: latest };
      clearTimeout(timer.current);
      setFresh([]);
      return;
    }
    const result = nextFeedbackEvents(
      events,
      last.current.id,
      clock,
      performance.now(),
    );
    const incoming = result.events;
    last.current.id = result.id;
    if (!incoming.length) return;
    clearTimeout(timer.current);
    setFresh(
      incoming
        .filter((e) => e.type === "play" || e.type === "deal")
        .map((e) => (e.type === "deal" ? -1 : e.seat)),
    );
    const sound = selectPokerSound(incoming, mapping);
    if (sound) {
      const sources = sound.startsWith("/")
        ? assetSources(assets, sound)
        : mediaSources(assets, `audio/${sound}.ogg`);
      void audio.playWithFallback(sources);
    }
    timer.current = setTimeout(() => setFresh([]), 700);
  }, [
    events,
    roomID,
    audio,
    assets,
    mapping,
    clock.serverNow,
    clock.receivedAt,
  ]);
  return fresh;
}
export function PokerLayout({
  room,
  phase,
  assets,
  style,
  hand,
  handCapacity,
  actionColumns,
  selected,
  onSelect,
  labels,
  counts,
  plays,
  center,
  actions,
  highlights,
  fresh = [],
}: {
  room: GameRoom;
  phase: string;
  assets: string;
  style: ReturnType<typeof useCardStyle>;
  hand: number[];
  handCapacity: number;
  actionColumns: number;
  selected: number[];
  onSelect: (ids: number[]) => void;
  labels: string[];
  counts: number[];
  plays: { seat: number; cards: number[]; pass?: boolean }[];
  center: ReactNode;
  actions: ReactNode;
  highlights?: Record<number, string>;
  fresh?: number[];
}) {
  const [credits, setCredits] = useState(false);
  const [preferences, setPreferences] = useState(false);
  const you = room.spectating || room.you < 0 ? 0 : room.you;
  const n = room.seats.length;
  const position = (seat: number) => {
    const rel = (seat - you + n) % n;
    return rel === 0
      ? "bottom"
      : n === 4 && rel === 2
        ? "top"
        : rel === 1
          ? "right"
          : "left";
  };
  return (
    <section
      className="poker-board"
      data-phase={phase}
      data-action-columns={actionColumns}
      style={
        { "--poker-desktop-action-columns": actionColumns } as CSSProperties
      }
      aria-label="扑克牌桌"
    >
      <div className="poker-table-toolbar">
        <button type="button" onClick={() => setPreferences(true)}>
          牌面设置
        </button>
      </div>
      {preferences && (
        <Modal title="牌面设置" onClose={() => setPreferences(false)}>
          {" "}
          <div className="poker-tools">
            <label>
              牌面风格
              <select
                aria-label="牌面风格"
                value={style.skin}
                onChange={(e) => style.setSkin(e.target.value)}
              >
                {skins.map((s) => (
                  <option value={s.id} key={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              牌背
              <select
                aria-label="牌背"
                value={style.back}
                onChange={(e) => style.setBack(e.target.value)}
              >
                {backs.map((s) => (
                  <option value={s.id} key={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </label>
            <button
              type="button"
              className="subtle"
              onClick={() => {
                setPreferences(false);
                setCredits(true);
              }}
            >
              素材来源
            </button>
          </div>
          <p>点击选牌，拖动可多选；点击手牌空白处或按 Escape 清除选择。</p>
        </Modal>
      )}
      <div className="poker-arena">
        {room.seats.map((p, seat) => {
          const pos = position(seat),
            play = plays.find((v) => v.seat === seat);
          return (
            <div
              key={p.id}
              data-poker-seat={seat}
              data-position={pos}
              className={`poker-seat poker-seat-${pos} ${room.game?.actors?.includes(seat) ? "acting" : ""}`}
            >
              <div className="poker-seat-label">
                <b>
                  {p.name}
                  {!room.spectating && seat === room.you ? " · 你" : ""}
                </b>
                <span className="poker-seat-meta">
                  <span className="poker-seat-role">{labels[seat]}</span>
                  <span className="poker-seat-count">{counts[seat]}张</span>
                </span>
              </div>
              {pos !== "bottom" && (
                <div className="poker-hidden-hand">
                  <Card
                    id={-1}
                    skin={style.skin}
                    back={style.back}
                    assets={assets}
                  />
                </div>
              )}
              <div
                className="poker-play"
                tabIndex={play?.cards.length ? 0 : undefined}
                role="region"
                aria-label={`${p.name}的出牌`}
              >
                <div className={fresh.includes(seat) ? "poker-new-play" : ""}>
                  {play?.pass ? (
                    <span className="poker-pass">不出</span>
                  ) : (
                    <CardStrip
                      singleRow
                      ids={play?.cards ?? []}
                      skin={style.skin}
                      assets={assets}
                    />
                  )}
                </div>
              </div>
            </div>
          );
        })}
        <div className="poker-center">{center}</div>
        <div className="poker-own-hand">
          {!room.spectating && room.you >= 0 ? (
            <div className={fresh.includes(-1) ? "poker-new-deal" : ""}>
              <Hand
                cards={hand}
                capacity={handCapacity}
                selected={selected}
                onChange={onSelect}
                seat={room.you}
                skin={style.skin}
                assets={assets}
                highlights={highlights}
              />
            </div>
          ) : (
            <p className="poker-spectator">正在观战，手牌仅本人可见。</p>
          )}
        </div>
      </div>
      <div className="poker-actions">{actions}</div>
      {credits && (
        <Modal title="扑克牌与音效来源" onClose={() => setCredits(false)}>
          <p>默认传统英式：Byron Knoll，由 notpeter 整理，公有领域／WTFPL。</p>
          <p>
            轻松插画：Nicu（nicubunu），公有领域／CC0。大字四色：Saul（saulspatz）的
            Jumbo Index Playing Cards，CC0。
          </p>
          <p>
            洗牌、出牌和收牌音效：Kenney Casino
            Audio，CC0。牌背为本项目原创。中文报牌使用 Apache-2.0 许可的 Kokoro
            中文模型预先合成，炸弹效果音由本项目生成。
          </p>
          <ul>
            <li>
              <a
                href="https://github.com/notpeter/Vector-Playing-Cards"
                target="_blank"
                rel="noreferrer"
              >
                传统英式原始素材
              </a>
            </li>
            <li>
              <a
                href="https://nicubunu.ro/graphics:playingcards"
                target="_blank"
                rel="noreferrer"
              >
                Nicu 原始素材
              </a>
            </li>
            <li>
              <a
                href="https://opengameart.org/content/jumbo-index-playing-cards"
                target="_blank"
                rel="noreferrer"
              >
                大字四色原始素材
              </a>
            </li>
            <li>
              <a
                href="https://kenney.nl/assets/casino-audio"
                target="_blank"
                rel="noreferrer"
              >
                Kenney 音效
              </a>
            </li>
            <li>
              <a href="/poker/manifest.json" target="_blank" rel="noreferrer">
                素材及校验和清单
              </a>
            </li>
          </ul>
        </Modal>
      )}
    </section>
  );
}
export function PokerOptions({ children }: { children: ReactNode }) {
  return <div className="poker-options">{children}</div>;
}
