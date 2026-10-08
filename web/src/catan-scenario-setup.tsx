import { catanVictoryTarget } from "./catan-rule-context";
import { catanScenarioVictory } from "./catan-scenarios";

const scenarios = [
  {
    id: "transport",
    name: "商人与蛮族 · 运输",
    description:
      "升级马车、运送货物赚金币与分数；二至六人，13分获胜。双人加入中立势力与贸易筹码，五六人使用扩大地图与配对回合。",
  },
  {
    id: "barbarian-attack",
    name: "蛮族进攻",
    description:
      "招募骑士、解放沿海村镇，俘虏蛮族得分；12分获胜。三至六人，五六人采用扩大地图与配对回合。",
  },
  {
    id: "cities-knights",
    name: "城市与骑士",
    description:
      "发展科学、贸易和政治，派骑士抵御蛮族；三至六人随机地图，13 分获胜；五六人使用配对回合。",
  },
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
  {
    id: "fishing",
    name: "卡坦渔夫",
    description:
      "湖泊与海岸渔场产鱼，花费鱼筹码换取行动；10分获胜，持有旧靴者需多1分。三至四人。",
  },
  { id: "", name: "基础版", description: "采集资源、贸易和建设，10 分获胜。" },
  {
    id: "rivers",
    name: "河流",
    description:
      "沿河建设赚金币，修桥并争夺财富奖励，10 分获胜；支持三至六人。",
  },
  {
    id: "caravans",
    name: "商队",
    description:
      "建设后共同出价，让商队经过自己的道路与城镇，12 分获胜；支持三至六人。",
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
      "三至六人连通村落收集布匹，每两枚得 1 分；14 分获胜，或回合末五村耗尽结算。本站补充规则：公共库存不足时仅记账补足当次缺额，空村仍不生产。",
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

export const supportsPublicCatanKnightsCombination = (scenario?: string) =>
  [
    "fishing",
    "shores",
    "islands",
    "fog",
    "desert",
    "new_world",
    "wonders",
    "cloth",
  ].includes(scenario || "");

export function CatanCombinationKnightsPicker({
  value,
  onChange,
  disabled = false,
  helpers = false,
  fishing = false,
}: {
  fishing?: boolean;
  value: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  helpers?: boolean;
}) {
  return (
    <fieldset className="catan-helper-options" disabled={disabled || helpers}>
      <label>
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
        />
        城市与骑士＋{fishing ? "渔夫" : "航海家"}
      </label>
      <p className="muted small">
        {helpers
          ? "先关闭 Helpers，才能加入城市与骑士。"
          : fishing
            ? "加入商品、进步牌和骑士；7鱼可选牌堆抽进步牌，13分获胜，持旧靴者需14分。"
            : "加入商品、进步牌和骑士，共同抵御蛮族；沿用所选海图的组合胜利条件。"}
      </p>
    </fieldset>
  );
}

export const supportsPublicCatanFishingSea = (scenario?: string) =>
  [
    "islands",
    "fog",
    "desert",
    "tribe",
    "cloth",
    "wonders",
    "new_world",
  ].includes(scenario || "");

export const supportsPublicCatanFishingSeaExtended = (scenario?: string) =>
  ["fog", "wonders", "new_world"].includes(scenario || "");

export function CatanFishingSeaPicker({
  value,
  onChange,
  disabled = false,
  blocked = false,
  fixedRequired = false,
  extended = false,
}: {
  value: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  blocked?: boolean;
  fixedRequired?: boolean;
  extended?: boolean;
}) {
  return (
    <fieldset
      className="catan-helper-options"
      disabled={disabled || blocked || fixedRequired || extended}
    >
      <label>
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
        />
        渔夫＋航海家
      </label>
      <p className="muted small">
        {extended
          ? "五六人使用扩大地图、8处渔场和配对回合。要关闭渔夫，请先取消下方五至六人扩充。"
          : fixedRequired
            ? "先将海图切为固定布局，才能加入渔夫。"
            : blocked
              ? "先关闭 Helpers、城市骑士、港口霸主和友善强盗，才能加入渔夫。"
              : "海岸渔场产鱼，可花5鱼修路或造船；旧靴提高1分门槛，保留所选海图的特殊终局条件。"}
      </p>
    </fieldset>
  );
}

export const isPublicCatanExplorer = (scenario?: string) =>
  ["land-ho", "spices-for-catan"].includes(scenario || "");

export const isPublicCatanFlexible = (scenario?: string) =>
  isPublicCatanExplorer(scenario) || scenario === "transport";

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
  knights = false,
  fishing = false,
  friendly = false,
  harbors = false,
  players = 3,
  fiveSix = false,
}: {
  value: string;
  onChange: (scenario: string) => void;
  disabled?: boolean;
  helpers?: boolean;
  knights?: boolean;
  fishing?: boolean;
  friendly?: boolean;
  harbors?: boolean;
  players?: number;
  fiveSix?: boolean;
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
                    "barbarian-attack",
                    "transport",
                    "fishing",
                    "rivers",
                    "caravans",
                    "cities-knights",
                    "land-ho",
                    "spices-for-catan",
                  ].includes(s.id)) ||
                (players > 4 &&
                  (fishing
                    ? !supportsPublicCatanFishingSeaExtended(s.id)
                    : fiveSix
                      ? ![
                          "",
                          "cities-knights",
                          "cloth",
                          "rivers",
                          "caravans",
                        ].includes(s.id)
                      : ![
                          "spices-for-catan",
                          "barbarian-attack",
                          "transport",
                        ].includes(s.id))) ||
                (players > 4 && (friendly || harbors) && s.id === "cloth") ||
                (friendly && s.id === "shores" && players < 4) ||
                (friendly &&
                  isPublicCatanSea(s.id) &&
                  ![
                    "shores",
                    "desert",
                    "cloth",
                    "pirate_islands",
                    "wonders",
                  ].includes(s.id)) ||
                (fishing &&
                  isPublicCatanSea(s.id) &&
                  !supportsPublicCatanFishingSea(s.id)) ||
                (knights &&
                  isPublicCatanSea(s.id) &&
                  !supportsPublicCatanKnightsCombination(s.id))
              }
            >
              {s.name}
            </option>
          ))}
        </select>
      </label>
      <p className="muted small">
        {harbors || (knights && supportsPublicCatanKnightsCombination(value))
          ? catanScenarioVictory(
              value,
              catanVictoryTarget(value, knights || value === "cities-knights") +
                (harbors ? 1 : 0),
            )
          : scenarios.find((s) => s.id === value)?.description}
      </p>
      {players > 4 && ["rivers", "caravans"].includes(value) && (
        <p className="muted small">
          本站数字配置：采用固定数列沿逆时针螺旋摆放，数字数量保持原扩充配置；不宣称与2025实体字母背面一致。
        </p>
      )}
      {players > 4 && value === "transport" && (
        <p className="muted small">
          本站牌组配置：五六人增加8张骑士、2张道路建设、2张快速旅程，共37张；按实际开局人数使用。
        </p>
      )}
      {["rivers", "transport"].includes(value) && (
        <p className="muted small">本站补充：金币用完继续记账发放。</p>
      )}
    </div>
  );
}
