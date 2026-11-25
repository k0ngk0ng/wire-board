import { useEffect, useRef, useState, type ReactNode } from "react";
import { GripVertical } from "lucide-react";
import { gameModule } from "../game-api/registry";
import { ModuleCover } from "../game-api/host";
import { LegacyCover } from "../legacy/cover";

type Game = { kind: string; name: string; enabled: boolean };
export function GameSort<T extends Game>({
  games,
  visibleGames,
  busy,
  onMove,
  onDragChange,
  children,
}: {
  games: T[];
  visibleGames: T[];
  busy: boolean;
  onMove: (from: string, to: string) => Promise<void>;
  onDragChange: (active: boolean) => void;
  children: (game: T) => ReactNode;
}) {
  const dragged = useRef<string | null>(null);
  const [active, setActive] = useState("");
  const [target, setTarget] = useState("");
  useEffect(() => () => onDragChange(false), [onDragChange]);
  const clear = () => {
    onDragChange(false);
    dragged.current = null;
    setActive("");
    setTarget("");
  };
  const move = (from: string, to: string) => {
    clear();
    if (!busy && from !== to) void onMove(from, to);
  };
  const begin = (kind: string) => {
    onDragChange(true);
    dragged.current = kind;
    setActive(kind);
  };
  return (
    <div className="admin-games" aria-busy={busy}>
      {visibleGames.map((game, index) => (
        <article
          key={game.kind}
          data-game-id={game.kind}
          className={`admin-game-card ${!game.enabled ? "unpublished" : ""} ${active === game.kind ? "dragging" : ""} ${target === game.kind && active !== game.kind ? "drop-target" : ""}`}
          onDragOver={(event) => {
            if (!busy && dragged.current) {
              event.preventDefault();
              event.dataTransfer.dropEffect = "move";
              setTarget(game.kind);
            }
          }}
          onDrop={(event) => {
            event.preventDefault();
            if (dragged.current) move(dragged.current, game.kind);
          }}
        >
          <div
            className="admin-game-cover"
            draggable={!busy}
            onDragStart={(event) => {
              if (busy) {
                event.preventDefault();
                return;
              }
              begin(game.kind);
              event.dataTransfer.effectAllowed = "move";
              event.dataTransfer.setData("text/plain", game.kind);
            }}
            onDragEnd={clear}
          >
            {gameModule(game.kind) ? (
              <ModuleCover kind={game.kind} mini />
            ) : (
              <LegacyCover kind={game.kind} mini />
            )}
          </div>
          <div className="admin-game-body">
            <div className="admin-game-heading">
              <div className="admin-game-number">
                {String(
                  games.findIndex((entry) => entry.kind === game.kind) + 1,
                ).padStart(2, "0")}
              </div>
              <div className="admin-game-title">
                <h3>{game.name}</h3>
                <span
                  className={`admin-badge ${game.enabled ? "green" : "gray"}`}
                >
                  {game.enabled ? "已上架" : "已下架"}
                </span>
              </div>
              <button
                type="button"
                className="admin-sort-handle"
                disabled={busy}
                aria-label={`移动${game.name}`}
                title="拖动或使用方向键排序"
                draggable={!busy}
                onDragStart={(event) => {
                  begin(game.kind);
                  event.dataTransfer.effectAllowed = "move";
                  event.dataTransfer.setData("text/plain", game.kind);
                }}
                onDragEnd={clear}
                onPointerDown={(event) => {
                  if (event.pointerType === "mouse" || busy) return;
                  event.preventDefault();
                  begin(game.kind);
                  event.currentTarget.setPointerCapture(event.pointerId);
                }}
                onPointerMove={(event) => {
                  if (
                    event.pointerType === "mouse" ||
                    dragged.current !== game.kind
                  )
                    return;
                  const element = document
                    .elementFromPoint(event.clientX, event.clientY)
                    ?.closest<HTMLElement>(".admin-game-card");
                  setTarget(element?.dataset.gameId ?? "");
                  if (Math.round(event.clientY) < 72) window.scrollBy(0, -24);
                  else if (Math.round(event.clientY) > window.innerHeight - 72)
                    window.scrollBy(0, 24);
                }}
                onPointerUp={(event) => {
                  if (event.pointerType === "mouse") return;
                  const element = document
                    .elementFromPoint(event.clientX, event.clientY)
                    ?.closest<HTMLElement>(".admin-game-card");
                  if (dragged.current && element?.dataset.gameId)
                    move(dragged.current, element.dataset.gameId);
                  else clear();
                }}
                onPointerCancel={clear}
                onKeyDown={(event) => {
                  const delta = ["ArrowLeft", "ArrowUp"].includes(event.key)
                    ? -1
                    : ["ArrowRight", "ArrowDown"].includes(event.key)
                      ? 1
                      : 0;
                  if (!delta) return;
                  event.preventDefault();
                  if (!event.repeat && visibleGames[index + delta])
                    move(game.kind, visibleGames[index + delta].kind);
                }}
              >
                <GripVertical size={18} />
              </button>
            </div>
            <div className="admin-game-actions">{children(game)}</div>
          </div>
        </article>
      ))}
    </div>
  );
}
