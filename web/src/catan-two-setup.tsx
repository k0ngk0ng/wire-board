import {
  catanRiversSeaScenarios,
  isPublicCatanRiversSea,
} from "./catan-rivers-sea-options";
import { catanTwoFishingKnightsNote } from "./catan-two-fishing-knights";
import { catanVictoryTarget } from "./catan-rule-context";
import { supportsTwoCatanHelpers } from "./catan-two-helpers";
import { supportsTwoCatanVariants } from "./catan-two-variants";
import {
  twoCatanSeaScenarios,
  supportsTwoCatanSeafarers,
  catanTwoSeafarersNote,
  catanTwoSeafarersKnightsNote,
  catanTwoFishingSeafarersNote,
} from "./catan-two-seafarers";
const scenarios = [
  {
    id: "transport-shores",
    name: "运输＋新海岸",
    description:
      "本站双人运输海图、中立海岸村庄、两次生产和船路过路费，17分获胜。",
  },
  {
    id: "transport-desert",
    name: "运输＋穿越沙漠",
    description: "本站双人运输海图，先村庄再城市和马车，17分获胜。",
  },
  {
    id: "attack-pirates",
    name: "蛮族进攻＋海盗群岛",
    description:
      "本站双人配方保留蓝红两条远征线、中立海岸与共享骑士。授勋可升级战舰；夺回自己的要塞且12分获胜，可加事件牌。",
  },
  {
    id: "attack-wonders",
    name: "蛮族进攻＋卡坦奇迹",
    description:
      "本站双人中立海岸与共享骑士，沙漠蛮族均衡登陆。完成四级奇迹或12分且等级独占领先，可加事件牌。",
  },
  {
    id: "attack-tribe",
    name: "蛮族进攻＋遗忘部落",
    description:
      "本站双人中立海岸与共享骑士，奖励蛮族牌结算后再补中立建设；13分获胜，可加事件牌。",
  },
  {
    id: "attack-desert",
    name: "蛮族进攻＋穿越沙漠",
    description:
      "本站双人四人沙漠地图、中立海岸建设与共享骑士，14分获胜，可加事件牌。",
  },
  {
    id: "attack-shores",
    name: "蛮族进攻＋新海岸",
    description: "本站双人中立海岸与共享骑士规则，14分获胜，可加事件牌。",
  },
  {
    id: "caravans-new-world",
    name: "商队＋新世界",
    description: "本站双人中立玩家与随机居中水源配方，14分获胜，可加事件牌。",
  },
  {
    id: "caravans-islands",
    name: "商队＋四岛",
    description:
      "采用四人组合固定图及本站双人中立玩家规则，15分获胜，可加事件牌。",
  },
  {
    id: "caravans-shores",
    name: "双人商队＋新海岸",
    description:
      "采用四人商队主岛与可变外岛，中立海岸开局、两次生产、双马车投票，16分获胜。",
  },
  {
    id: "caravans-desert",
    name: "双人商队＋穿越沙漠",
    description:
      "四人组合图、中立海岸开局、双次生产，每轮尽量放两辆马车，16分获胜。",
  },
  {
    id: "caravans-tribe",
    name: "双人商队＋遗忘部落",
    description: "中立不领部落奖励；双次生产与双马车投票，15分获胜。",
  },
  ...catanRiversSeaScenarios,
  {
    id: "attack-transport",
    name: "蛮族进攻＋运输",
    description:
      "骑士战斗与货物运输，共用蛮族和金币；14 分获胜。二至六人，可叠加城市骑士与事件牌；五六人使用本站地图。",
  },
  {
    id: "caravans-transport",
    name: "商队＋运输",
    description:
      "运输货物并投票放商队马车；商品地块正常生产，无强盗与最长道路，15 分获胜。二至六人，可叠加城市骑士与事件牌；五六人使用本站地图。",
  },
  {
    id: "caravans-attack",
    name: "商队＋蛮族进攻",
    description:
      "沿海水源出发的商队与蛮族战斗并用，蛮族不阻挡马车；二至六人，12 分获胜，可叠加城市骑士和事件牌。",
  },
  {
    id: "rivers-transport",
    name: "河流＋运输",
    description:
      "沿河建设、修桥并运送货物；共用金币，贫穷不扣分。二至六人，13 分获胜，可叠加城市骑士和事件牌；五六人使用本站地图。",
  },
  {
    id: "rivers-attack",
    name: "河流＋蛮族进攻",
    description:
      "沿河建设、修桥并抵御沿海蛮族；共用金币，贫穷不扣分，12 分获胜。二至六人，可叠加城市骑士和事件牌；五六人使用本站组合地图。",
  },
  {
    id: "rivers-caravans",
    name: "双人＋河流＋商队",
    description:
      "沿河建设赚金币，桥位可放马车；两次生产、中立建设及每轮最多两辆商队马车，12 分获胜。",
  },
  {
    id: "barbarian-attack",
    name: "双人＋蛮族进攻",
    description:
      "12 分获胜；共享中立骑士，真人与中立村庄分别触发登陆，可消费贸易筹码移动蛮族。",
  },
  ...twoCatanSeaScenarios,
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
  fishingEnabled = false,
  knightsEnabled = false,
  target,
}: {
  value: string;
  onChange: (scenario: string) => void;
  disabled?: boolean;
  variantsEnabled?: boolean;
  helpersEnabled?: boolean;
  fishingEnabled?: boolean;
  knightsEnabled?: boolean;
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
                ((isPublicCatanRiversSea(s.id) ||
                  [
                    "caravans-new-world",
                    "caravans-islands",
                    "caravans-shores",
                    "caravans-desert",
                    "caravans-tribe",
                  ].includes(s.id)) &&
                  (knightsEnabled || fishingEnabled)) ||
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
      {supportsTwoCatanSeafarers(value) && (
        <p className="muted small">
          {knightsEnabled
            ? catanTwoSeafarersKnightsNote
            : fishingEnabled
              ? catanTwoFishingSeafarersNote
              : catanTwoSeafarersNote}
        </p>
      )}
      {value === "wonders" && (
        <p className="muted small">
          目标 {target ?? 10} 分，并满足奇迹胜利条件。
        </p>
      )}
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
        {fishingEnabled && value === "caravans"
          ? `渔夫＋商队${knightsEnabled ? "＋城市骑士" : ""}：每人起始五枚鱼筹码，停用贸易筹码，公开分数落后者鱼行动少付1鱼；保留中立建设和每轮最多两辆马车。${knightsEnabled ? "15" : "12"}分获胜，旧靴多需1分。`
          : fishingEnabled && value === "rivers"
            ? `渔夫＋河流${knightsEnabled ? "＋城市骑士" : ""}：不放湖泊，每人起始五枚鱼筹码，不使用贸易筹码；6鱼建桥得3金币，公开分数落后者少付1鱼，随后安排中立建设。${knightsEnabled ? "13" : "10"}分获胜。`
            : knightsEnabled && value === "rivers"
              ? "河流＋城市骑士：13分获胜；每回合两次事件与生产，沿用中立建筑、桥梁和骑士；金币与贸易筹码分别计算。"
              : knightsEnabled && value === "caravans"
                ? "商队＋城市骑士：木材和砖块出价，15分获胜；每回合两次事件与生产，中立骑士不激活，每轮最多放两辆马车。"
                : fishingEnabled && value === "cities-knights"
                  ? catanTwoFishingKnightsNote
                  : scenarios.find((s) => s.id === value)?.description}
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
          value !== "wonders" &&
          ` ${target ?? catanVictoryTarget(value, value === "cities-knights")} 分获胜。`}
        {(value === "fishing" || fishingEnabled) &&
          `旧靴持有者需 ${(target ?? 10) + 1} 分。`}
      </p>
      {["pirate-lairs", "fish-for-catan", "explorers-and-pirates"].includes(
        value,
      ) && (
        <p className="muted small">
          本站巢穴数字配置：3、4、5、9、10、11，随机分配，攻陷前隐藏；不使用双人贸易筹码。
        </p>
      )}
      {!fishingEnabled &&
        !knightsEnabled &&
        ![
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
            {["rivers", "transport", "barbarian-attack"].includes(value)
              ? "与金币"
              : ""}
            用完继续记账发放。
            {value === "caravans" && "商队确实无法放满时，只放能放的数量。"}
          </p>
        )}
    </div>
  );
}
