import { catanVictoryTarget } from "./catan-rule-context";
import { supportsTwoCatanHelpers } from "./catan-two-helpers";
import { supportsTwoCatanVariants } from "./catan-two-variants";
const scenarios = [
  {
    id: "pirate-lairs",
    name: "探索者与海盗 · 海盗巢穴",
    description: "派船员合力攻陷巢穴、解放金矿；二至六人。",
  },
  {
    id: "fish-for-catan",
    name: "探索者与海盗 · 卡坦鱼群",
    description: "攻陷海盗巢穴、捕鱼并运回议会岛；二至六人。",
  },
  {
    id: "explorers-and-pirates",
    name: "探索者与海盗 · 完整三任务",
    description: "同时探索巢穴、鱼群与香料任务；二至六人。",
  },

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
      "捕鱼、结交农场，将货物运回议会岛；两家中立势力仅作静态障碍，不使用贸易筹码。",
  },
  {
    id: "land-ho",
    name: "探索者与海盗 · 初航",
    description: "装载移民并航行探索新岛；不使用双人贸易筹码。",
  },
  {
    id: "",
    name: "双人基础版",
    description: "两位玩家与两家中立势力，双次生产、贸易筹码。",
  },
  {
    id: "cities-knights",
    name: "双人＋城市与骑士",
    description:
      "两次城市事件与生产，中立骑士、商品与进步牌；贸易筹码限量 20 枚。",
  },
  {
    id: "fishing",
    name: "双人＋渔夫",
    description:
      "每人起始五枚鱼筹码，不使用贸易筹码；公开分数落后者每次鱼行动少付一鱼。",
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
  variantsEnabled = false,
  helpersEnabled = false,
  target,
}: {
  value: string;
  onChange: (scenario: string) => void;
  disabled?: boolean;
  variantsEnabled?: boolean;
  helpersEnabled?: boolean;
  target?: number;
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
            <option
              key={s.id}
              value={s.id}
              disabled={
                (variantsEnabled && !supportsTwoCatanVariants(s.id)) ||
                (helpersEnabled &&
                  !supportsTwoCatanHelpers(s.id) &&
                  ![
                    "land-ho",
                    "spices-for-catan",
                    "pirate-lairs",
                    "fish-for-catan",
                    "explorers-and-pirates",
                  ].includes(s.id))
              }
            >
              {s.name}
            </option>
          ))}
        </select>
      </label>
      {helpersEnabled && (
        <p className="muted small">
          切换到尚未接通助手的剧本前，请先关闭 Helpers。
        </p>
      )}
      {variantsEnabled && (
        <p className="muted small">
          切换到其他双人剧本前，请先关闭友善强盗和港口霸主。
        </p>
      )}
      <p className="muted small">
        {scenarios.find((s) => s.id === value)?.description}
        {[
          "land-ho",
          "pirate-lairs",
          "fish-for-catan",
          "spices-for-catan",
          "explorers-and-pirates",
        ].includes(value) &&
          ` ${target ?? catanVictoryTarget(value, false)} 分获胜。`}
        {value === "land-ho" &&
          ((target ?? 8) > 8
            ? "自由开局，先放城市再放港口。"
            : "沿用印刷开局。")}
        {supportsTwoCatanVariants(value) &&
          ` ${target ?? (value === "cities-knights" ? 13 : 10)} 分获胜。`}
        {value === "fishing" && `旧靴持有者需 ${(target ?? 10) + 1} 分。`}
      </p>
      {["pirate-lairs", "fish-for-catan", "explorers-and-pirates"].includes(
        value,
      ) && (
        <p className="muted small">
          本站巢穴数字配置：3、4、5、9、10、11，随机分配，攻陷前隐藏；不使用双人贸易筹码。
        </p>
      )}
      {![
        "cities-knights",
        "fishing",
        "land-ho",
        "spices-for-catan",
        "pirate-lairs",
        "fish-for-catan",
        "explorers-and-pirates",
      ].includes(value) && (
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
