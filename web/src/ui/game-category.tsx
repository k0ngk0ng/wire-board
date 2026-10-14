import { gameModule } from "../game-api/registry";
export const categories = [
  ["all", "全部分类"],
  ["board", "桌游"],
  ["poker", "扑克牌"],
  ["realtime", "即时策略"],
  ["sports", "体育"],
] as const;
export function matchesCategory(kind: string, category: string) {
  return (
    category === "all" || (gameModule(kind)?.category ?? "board") === category
  );
}
export function CategoryFilter({
  value,
  onChange,
  label = "游戏分类",
}: {
  value: string;
  onChange: (value: string) => void;
  label?: string;
}) {
  return (
    <label className="game-category">
      {label}
      <select
        aria-label={label}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        {categories.map(([key, name]) => (
          <option value={key} key={key}>
            {name}
          </option>
        ))}
      </select>
    </label>
  );
}
