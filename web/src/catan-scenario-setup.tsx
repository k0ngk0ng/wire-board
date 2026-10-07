const scenarios = [
  {
    id: "spices-for-catan",
    name: "探索者与海盗 · 卡坦香料",
    description:
      "捕捞鱼群、结交农场，将鱼群与香料运回议会岛，15 分获胜；两至六人，五六人采用扩大地图与配对回合。",
  },
  {
    id: "land-ho",
    name: "探索者与海盗 · 初航",
    description: "印刷开局，装载移民并航行探索新岛，8 分获胜；支持两至四人。",
  },
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
  {
    id: "tribe",
    name: "航海家 · 遗忘的部落",
    description: "航行领取部落的胜利点、发展卡与港口，13 分获胜。",
  },
  {
    id: "cloth",
    name: "航海家 · 卡坦布匹",
    description:
      "连通村落收集布匹，每两枚得 1 分；14 分获胜，或回合末五个村落耗尽时比较分数与布匹。",
  },
  {
    id: "pirate_islands",
    name: "航海家 · 海盗群岛",
    description: "组建战舰，夺回自己的要塞并达到 10 分获胜；使用固定地图。",
  },
  {
    id: "wonders",
    name: "航海家 · 卡坦奇迹",
    description: "建成四级奇迹，或达到 10 分且已建奇迹等级独自领先，即可获胜。",
  },
  {
    id: "new_world",
    name: "航海家 · 新世界",
    description: "开局前共同确认地图，轮流放置港口，再探索岛屿，12 分获胜。",
  },
];

export const isPublicCatanExplorer = (scenario?: string) =>
  ["land-ho", "spices-for-catan"].includes(scenario || "");

export const isPublicCatanSea = (scenario?: string) =>
  [
    "shores",
    "islands",
    "fog",
    "desert",
    "tribe",
    "cloth",
    "pirate_islands",
    "wonders",
    "new_world",
  ].includes(scenario || "");

export function CatanScenarioPicker({
  value,
  onChange,
  disabled = false,
  helpers = false,
  players = 3,
}: {
  value: string;
  onChange: (scenario: string) => void;
  disabled?: boolean;
  helpers?: boolean;
  players?: number;
}) {
  return (
    <div className="catan-two-picker">
      <label>
        卡坦剧本
        <select
          value={value}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value)}
        >
          {scenarios.map((s) => (
            <option
              key={s.id}
              value={s.id}
              disabled={
                (helpers &&
                  [
                    "rivers",
                    "caravans",
                    "land-ho",
                    "spices-for-catan",
                  ].includes(s.id)) ||
                (players > 4 && s.id !== "spices-for-catan")
              }
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
