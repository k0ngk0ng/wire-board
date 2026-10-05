import type { Room } from "./types";
import "./catan-scenarios.css";

const names: Record<string, string> = {
  shores: "驶向新海岸",
  islands: "四岛",
  six_islands: "六岛",
  fog: "迷雾群岛",
  desert: "穿越沙漠",
  tribe: "遗忘的部落",
  cloth: "卡坦布匹",
  pirate_islands: "海盗群岛",
  wonders: "卡坦奇迹",
  new_world: "新世界",
};
export const catanScenarioName = (id: string) => names[id] || "航海家";
export const catanLayoutName = (id: string) =>
  ({
    fixed: "官方固定布局",
    variable: "官方可变布局",
    prepared: "共同确认地图",
  })[id] || "";

export function catanScenarioVictory(id: string, target: number) {
  if (id === "wonders") return `建成4级奇迹，或${target}分且奇迹等级独自领先`;
  if (id === "pirate_islands") return `${target}分且夺回自己的要塞`;
  if (id === "cloth") return `${target}分获胜；回合结束时5座村落耗尽也会结算`;
  return `${target}分获胜`;
}

const descriptions: Record<string, string> = {
  shores: "从主岛出发，探索其他岛屿；每位玩家首次在各个新区域定居额外得2分。",
  islands: "两处起始建筑所在的岛是自己的家乡岛；在其他岛首次定居额外得2分。",
  fog: "沿路线发现未知地形，陆地产生发现奖励；未知地形和数字在探索前保密。",
  desert: "从主岛出发，穿越沙漠或驶向外岛；各个新区域首次定居额外得2分。",
  tribe: "驾船接触部落，领取胜利点、发展卡和可带回安放的港口。",
  cloth:
    "建造三组起始村庄，用船线与村落建立贸易。每两枚布匹得1分，没有最长路线奖励。",
  pirate_islands:
    "从预设村庄出发，延伸自己的远征航线，升级战舰并夺回海盗要塞。",
  wonders:
    "满足条件后领取独占奇迹，支付资源逐级建设。先手先选择强盗所在的沙漠。",
  new_world:
    "开局前确认地形和数字，轮流放港口；每位玩家首次在各个非家乡岛定居额外得1分。",
};

export function CatanSeafarersPicker({
  room,
  disabled,
  command,
}: {
  room: Room;
  disabled: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
}) {
  const setup = room.catanSeafarers;
  const choices = room.catanSeafarersChoices;
  if (!setup || !choices?.length) return null;
  const info = choices.find((s) => s.id === setup.scenario);
  if (!info) return null;
  return (
    <fieldset
      className="catan-helper-options catan-scenario-options"
      disabled={disabled}
    >
      <legend>航海家剧本</legend>
      <label>
        剧本
        <select
          value={setup.scenario}
          onChange={(e) =>
            command("catan_seafarers", {
              catanSeafarers: { scenario: e.target.value },
            })
          }
        >
          {choices.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      {info.layouts.length > 1 ? (
        <label>
          地图布局
          <select
            value={setup.layout}
            onChange={(e) =>
              command("catan_seafarers", {
                catanSeafarers: { ...setup, layout: e.target.value },
              })
            }
          >
            {info.layouts.map((layout) => (
              <option key={layout} value={layout}>
                {catanLayoutName(layout)}
              </option>
            ))}
          </select>
        </label>
      ) : (
        <p className="catan-scenario-layout">{catanLayoutName(setup.layout)}</p>
      )}
      <p>
        {info.id === "cloth" && room.catanCitiesKnights
          ? "依次放置村庄、城市、村庄，仅第三座领取普通起始资源。船线连接村落，每两枚布匹得1分，没有最长路线奖励。"
          : descriptions[info.id]}
      </p>
      <strong className="catan-scenario-victory">
        {catanScenarioVictory(info.id, info.victoryPoints)}
      </strong>
      {room.catanOptions?.fiveSix && info.id === "shores" && (
        <small>随机主岛地形，数字按官方螺旋排列；外围岛屿保持固定。</small>
      )}
      <small>
        更换剧本、布局或人数扩充后，所有人需要重新准备。人数变化时会切换为对应的官方默认布局。
      </small>
    </fieldset>
  );
}
