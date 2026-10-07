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
  {
    id: "shores",
    name: "航海家 · 驶向新海岸",
    description: "从主岛驶向外岛，首次在新区域定居得额外分数，14 分获胜。",
  },
  {
    id: "islands",
    name: "航海家 · 四岛",
    description: "选择自己的家乡岛，向其他岛屿拓展航线，13 分获胜。",
  },
  {
    id: "fog",
    name: "航海家 · 迷雾群岛",
    description: "建设路线揭开未知地形，领取探索奖励，12 分获胜。",
  },
  {
    id: "desert",
    name: "航海家 · 穿越沙漠",
    description: "穿过沙漠或出海定居，争取新区域奖励，14 分获胜。",
  },
];

export const isPublicCatanSea = (scenario?: string) =>
  ["shores", "islands", "fog", "desert"].includes(scenario || "");

export function CatanScenarioPicker({
  value,
  onChange,
  disabled = false,
  helpers = false,
}: {
  value: string;
  onChange: (scenario: string) => void;
  disabled?: boolean;
  helpers?: boolean;
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
            <option
              key={s.id}
              value={s.id}
              disabled={helpers && ["rivers", "caravans"].includes(s.id)}
            >
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
