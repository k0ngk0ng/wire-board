const scenarios = [
  {
    id: "transport",
    name: "双人＋运输",
    description:
      "升级马车并运送货物，13分获胜；两次生产、贸易筹码和中立建设，通行中立道路累计支付过路费。",
  },
  {
    id: "spices-for-catan",
    name: "探索者与海盗 · 卡坦香料",
    description:
      "捕鱼、结交农场，将货物运回议会岛，15 分获胜；两家中立势力仅作静态障碍，不使用贸易筹码。",
  },
  {
    id: "land-ho",
    name: "探索者与海盗 · 初航",
    description:
      "印刷开局，装载移民并航行探索新岛，8 分获胜；不使用双人贸易筹码。",
  },
  {
    id: "",
    name: "双人基础版",
    description: "两位玩家与两家中立势力，双次生产、贸易筹码，10 分获胜。",
  },
  {
    id: "rivers",
    name: "双人＋河流",
    description: "沿河建设赚金币，修桥并争夺财富奖励，10 分获胜。",
  },
  {
    id: "caravans",
    name: "双人＋商队",
    description: "为商队出价，每轮尽量放满两辆车，12 分获胜。",
  },
];

export function CatanTwoScenarioPicker({
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
        双人卡坦剧本
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
      {!["land-ho", "spices-for-catan"].includes(value) && (
        <p className="muted small">
          采用 2025 双人规则。本站补充：贸易筹码
          {["rivers", "transport"].includes(value) ? "与金币" : ""}
          用完继续记账发放。
          {value === "caravans" && "商队确实无法放满时，只放能放的数量。"}
        </p>
      )}
    </div>
  );
}
