const scenarios = [
  { id: "", name: "基础版", description: "采集资源、贸易和建设，10 分获胜。" },
  {
    id: "rivers",
    name: "河流",
    description: "沿河建设赚金币，修桥并争夺财富奖励，10 分获胜。",
  },
  {
    id: "caravans",
    name: "商队",
    description: "建设后共同出价，让商队经过自己的道路与城镇，12 分获胜。",
  },
];

export function CatanScenarioPicker({
  value,
  onChange,
  disabled = false,
}: {
  value: string;
  onChange: (scenario: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="catan-two-picker">
      <label>
        三至四人卡坦剧本
        <select
          value={value}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value)}
        >
          {scenarios.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      <p className="muted small">
        {scenarios.find((s) => s.id === value)?.description}
      </p>
      {value === "rivers" && (
        <p className="muted small">本站补充：金币用完继续记账发放。</p>
      )}
    </div>
  );
}
