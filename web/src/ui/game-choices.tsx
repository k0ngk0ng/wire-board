import { CategoryFilter, matchesCategory } from "./game-category";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Check } from "lucide-react";
import { gameModule, gameName } from "../game-api/registry";

/** Keep the shared illustrated choices usable as the catalogue grows. */
export function GameChoices({
  games,
  value,
  onChange,
  renderCover,
  disabled = false,
}: {
  games: string[];
  value: string;
  onChange: (kind: string) => void;
  renderCover: (kind: string) => ReactNode;
  disabled?: boolean;
}) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");
  const list = useRef<HTMLDivElement>(null);
  const selectedGame = gameModule(value);
  const term = query.trim().toLocaleLowerCase();
  const visible = games.filter(
    (id) =>
      matchesCategory(id, category) &&
      `${gameName(id)} ${id}`.toLocaleLowerCase().includes(term),
  );
  useEffect(() => {
    const grid = list.current;
    const selected = grid?.querySelector<HTMLElement>(".selected");
    if (grid)
      grid.scrollTop = selected
        ? selected.offsetTop - grid.clientHeight / 2
        : 0;
  }, [value, query]);
  return (
    <section className="game-choices" aria-label="选择游戏">
      <CategoryFilter
        value={category}
        onChange={setCategory}
        label="可选游戏分类"
      />
      <label className="game-search">
        搜索游戏
        <input
          type="search"
          aria-label="搜索可选游戏"
          placeholder="输入游戏名称"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.nativeEvent.isComposing)
              event.preventDefault();
          }}
        />
      </label>
      <p className="game-choice-summary">
        已选：{gameName(value)}
        {selectedGame && (
          <span className="game-choice-players">
            {selectedGame.playerCounts
              ? selectedGame.playerCounts.join(" / ")
              : selectedGame.minPlayers === selectedGame.maxPlayers
                ? selectedGame.minPlayers
                : `${selectedGame.minPlayers}–${selectedGame.maxPlayers}`}{" "}
            人
          </span>
        )}
      </p>
      {selectedGame && (
        <p className="game-choice-description">{selectedGame.description}</p>
      )}
      <div className="choose-games" ref={list}>
        {visible.map((id) => (
          <button
            type="button"
            key={id}
            className={id === value ? "selected" : ""}
            aria-pressed={id === value}
            disabled={disabled}
            onClick={() => onChange(id)}
          >
            {renderCover(id)}
            <span>
              {gameName(id)}
              {id === value && <Check size={16} />}
            </span>
          </button>
        ))}
      </div>
      {!visible.length && (
        <p role="status" className="game-search-empty">
          没有找到匹配的游戏，请换个名称搜索。
        </p>
      )}
    </section>
  );
}
