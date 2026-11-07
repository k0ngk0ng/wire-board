import { useRef, useState } from "react";
import { GripVertical } from "lucide-react";
import { Modal } from "../ui/board-host";
import { CategoryFilter, matchesCategory } from "../ui/game-category";
import { gameModule } from "../game-api/registry";
import { ModuleCover } from "../game-api/host";
import { LegacyCover } from "../legacy/cover";
import { reorderVisible } from "./game-order";

type Game = { kind: string; name: string; enabled: boolean };
export function GameSort({
  games,
  busy,
  onSave,
  onClose,
}: {
  games: Game[];
  busy: boolean;
  onSave: (order: string[], expected: string[]) => Promise<void>;
  onClose: () => void;
}) {
  const base = useRef(games.map((g) => g.kind));
  const [order, setOrder] = useState(base.current);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");
  const [error, setError] = useState("");
  const [announcement, setAnnouncement] = useState("");
  const dragged = useRef<string | null>(null);
  const visible = order
    .map((id) => games.find((g) => g.kind === id)!)
    .filter(
      (g) =>
        g &&
        matchesCategory(g.kind, category) &&
        `${g.name} ${g.kind}`
          .toLocaleLowerCase()
          .includes(query.trim().toLocaleLowerCase()),
    );
  const move = (from: string, to: string) => {
    if (busy || from === to) return;
    setOrder((old) =>
      reorderVisible(
        old,
        visible.map((g) => g.kind),
        from,
        to,
      ),
    );
    setAnnouncement(`${games.find((g) => g.kind === from)?.name}的位置已调整`);
  };
  return (
    <Modal
      title="游戏排序"
      className="create-room-modal create-room-choosing admin-sort-modal"
      dismissible={!busy}
      onClose={onClose}
    >
      <p className="admin-sort-hint">
        拖动封面调整顺序；触屏拖动右下角手柄，键盘聚焦手柄后使用方向键。
      </p>
      <div className="game-choice-filters">
        <CategoryFilter
          value={category}
          onChange={setCategory}
          label="排序游戏分类"
        />
        <label className="game-search">
          搜索游戏
          <input
            type="search"
            aria-label="搜索排序游戏"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
      </div>
      <div className="choose-games admin-sort-grid">
        {visible.map((g, index) => (
          <article
            className="admin-sort-card"
            key={g.kind}
            data-game-id={g.kind}
            draggable={!busy}
            onDragStart={(e) => {
              dragged.current = g.kind;
              e.dataTransfer.effectAllowed = "move";
              e.dataTransfer.setData("text/plain", g.kind);
            }}
            onDragOver={(e) => {
              if (dragged.current && !busy) {
                e.preventDefault();
                e.dataTransfer.dropEffect = "move";
              }
            }}
            onDrop={(e) => {
              e.preventDefault();
              if (dragged.current) move(dragged.current, g.kind);
              dragged.current = null;
            }}
            onDragEnd={() => {
              dragged.current = null;
            }}
          >
            {gameModule(g.kind) ? (
              <ModuleCover kind={g.kind} mini />
            ) : (
              <LegacyCover kind={g.kind} mini />
            )}
            <div>
              <span>
                {g.name}
                {!g.enabled && <small>已下架</small>}
              </span>
              <button
                type="button"
                className="admin-sort-handle"
                disabled={busy}
                aria-label={`移动${g.name}`}
                title="拖动或使用方向键排序"
                onPointerDown={(e) => {
                  if (e.pointerType === "mouse") return;
                  dragged.current = g.kind;
                  e.currentTarget.setPointerCapture(e.pointerId);
                }}
                onPointerUp={(e) => {
                  if (e.pointerType === "mouse") return;
                  const target = document
                    .elementFromPoint(e.clientX, e.clientY)
                    ?.closest<HTMLElement>("[data-game-id]");
                  if (dragged.current && target?.dataset.gameId)
                    move(dragged.current, target.dataset.gameId);
                  dragged.current = null;
                }}
                onPointerCancel={() => {
                  dragged.current = null;
                }}
                onKeyDown={(e) => {
                  const delta = ["ArrowLeft", "ArrowUp"].includes(e.key)
                    ? -1
                    : ["ArrowRight", "ArrowDown"].includes(e.key)
                      ? 1
                      : 0;
                  if (delta) {
                    e.preventDefault();
                    const target = visible[index + delta];
                    if (target) move(g.kind, target.kind);
                  }
                }}
              >
                <GripVertical size={18} />
              </button>
            </div>
          </article>
        ))}
      </div>
      {!visible.length && <p role="status">没有找到匹配的游戏。</p>}
      {error && <p role="alert">{error}</p>}
      <span role="status" className="sr-only">
        {announcement}
      </span>
      <div className="modal-actions">
        <button className="outline" disabled={busy} onClick={onClose}>
          取消排序
        </button>
        <button
          className="primary"
          aria-label="保存游戏顺序"
          disabled={busy || order.join() === base.current.join()}
          onClick={async () => {
            setError("");
            try {
              await onSave(order, base.current);
            } catch (e) {
              setError((e as Error).message);
            }
          }}
        >
          {busy ? "正在保存…" : "保存顺序"}
        </button>
      </div>
    </Modal>
  );
}
