import {
  catanRiversSeaScenarios,
  isPublicCatanRiversSea,
  catanRiversSeaAvailable,
} from "./catan-rivers-sea-options";
import { catanExplorerTwoKnightsNote } from "./catan-explorer-two-knights";
import { catanVictoryTarget } from "./catan-rule-context";
import { catanScenarioVictory } from "./catan-scenarios";

const scenarios = [
  {
    id: "attack-shores",
    name: "蛮族进攻＋新海岸",
    description:
      "三四人官方地图、五六人本站扩大配方，蛮族与骑士仅在主岛活动，无强盗海盗，14分获胜，可加事件牌。",
  },
  {
    id: "caravans-new-world",
    name: "商队＋新世界",
    description:
      "随机地图居中放水源，先放港口再建设，14分获胜。二至六人，可加事件牌；双人及五六人使用本站适配。",
  },
  {
    id: "caravans-islands",
    name: "商队＋四岛／六岛",
    description:
      "中央水源向海面延伸马车，船与己方马车同边计两段路线，15分获胜。二至六人固定图，可加事件牌；五六人采用本站双水源六岛配方，双人采用本站中立玩家规则，三人海格上的孤立4点圆片按本站补充规则不使用。",
  },
  {
    id: "caravans-shores",
    name: "商队＋新海岸",
    description:
      "商队双水源主岛与外岛探索，16分获胜。支持二至六人；三人为官方固定图，四人商队主岛及可变外岛，五六人本站配方，可加事件牌。",
  },
  {
    id: "caravans-tribe",
    name: "商队＋遗忘部落",
    description:
      "水源替换12点麦田，12叠到2点牧场；商队可走海边，保留部落奖励，15分获胜。二至六人，可加事件牌；五六人采用本站双水源配方。",
  },
  {
    id: "caravans-desert",
    name: "商队＋穿越沙漠",
    description:
      "商队可沿海格边延伸，与己方船同边计两段最长路线；水源数字移到指定地块，16分获胜。支持二至六人，可加事件牌；五六人采用本站双水源配方。",
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
    name: "河流＋商队",
    description:
      "沿河建设与商队投票并用；桥位可放马车，2／12 分别叠放在两块 3 点地上。二至六人，12 分获胜；可叠加城市骑士和事件牌。",
  },
  {
    id: "pirate-lairs",
    name: "探索者与海盗 · 海盗巢穴",
    description: "派船员合力攻陷巢穴、解放金矿，12 分获胜；二至六人。",
  },
  {
    id: "fish-for-catan",
    name: "探索者与海盗 · 卡坦鱼群",
    description: "攻陷海盗巢穴、捕鱼并运回议会岛，15 分获胜；二至六人。",
  },
  {
    id: "explorers-and-pirates",
    name: "探索者与海盗 · 完整三任务",
    description: "同时探索巢穴、鱼群与香料任务，17 分获胜；二至六人。",
  },

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
    description:
      "二至四人保留印刷开局；本站五六人采用扩大地图和自由开局。8 分获胜，可叠加二至六人骑士组合（13 分）。",
  },
  {
    id: "fishing",
    name: "卡坦渔夫",
    description:
      "湖泊与海岸渔场产鱼，花费鱼筹码换取行动；10 分获胜，持有旧靴者需多 1 分。支持三至六人，五六人使用配对回合。",
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

export const supportsPublicExplorerKnights = (scenario?: string) =>
  [
    "land-ho",
    "pirate-lairs",
    "fish-for-catan",
    "spices-for-catan",
    "explorers-and-pirates",
  ].includes(scenario || "");

export const supportsPublicCatanKnightsCombination = (scenario?: string) =>
  supportsPublicExplorerKnights(scenario) ||
  scenario === "transport" ||
  [
    "rivers-caravans",
    "rivers-attack",
    "rivers-transport",
    "caravans-attack",
    "caravans-transport",
    "attack-transport",
  ].includes(scenario || "") ||
  scenario === "barbarian-attack" ||
  [
    "rivers",
    "caravans",
    "fishing",
    "shores",
    "islands",
    "fog",
    "desert",
    "new_world",
    "wonders",
    "cloth",
    "tribe",
    "pirate_islands",
  ].includes(scenario || "");

export function CatanCombinationKnightsPicker({
  attack = false,
  transport = false,
  rivers = false,
  riversCaravans = false,
  riversAttack = false,
  riversTransport = false,
  caravansAttack = false,
  caravansTransport = false,
  attackTransport = false,
  caravans = false,
  tribe = false,
  pirateIslands = false,
  value,
  onChange,
  disabled = false,
  helpers = false,
  fishing = false,
  explorer = false,
  intro = false,
  two = false,
  harbors = false,
}: {
  attack?: boolean;
  transport?: boolean;
  rivers?: boolean;
  riversCaravans?: boolean;
  riversAttack?: boolean;
  riversTransport?: boolean;
  caravansAttack?: boolean;
  caravansTransport?: boolean;
  attackTransport?: boolean;
  caravans?: boolean;
  tribe?: boolean;
  pirateIslands?: boolean;
  explorer?: boolean;
  intro?: boolean;
  two?: boolean;
  fishing?: boolean;
  harbors?: boolean;
  value: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  helpers?: boolean;
}) {
  return (
    <fieldset className="catan-helper-options" disabled={disabled}>
      <label>
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
        />
        城市与骑士＋
        {attackTransport
          ? "蛮族进攻＋运输"
          : caravansTransport
            ? "商队＋运输"
            : caravansAttack
              ? "商队＋蛮族进攻"
              : riversTransport
                ? "河流＋运输"
                : riversAttack
                  ? "河流＋蛮族进攻"
                  : riversCaravans
                    ? "河流＋商队"
                    : attack
                      ? "蛮族进攻"
                      : transport
                        ? "运输"
                        : rivers
                          ? "河流"
                          : caravans
                            ? "商队"
                            : explorer
                              ? "探索者与海盗"
                              : fishing
                                ? "渔夫"
                                : "航海家"}
      </label>
      <p className="muted small">
        {attackTransport
          ? "本站三模块适配：14 分获胜；使用道路骑士与进步牌，无海上蛮族，共用蛮族和金币；回合末先战斗、后运输。2／12 正常生产并登陆。"
          : caravansTransport
            ? "本站三模块适配：15 分获胜；木材或砖块出价，运输完成后投票。使用进步牌，无强盗与最长道路；商品地块正常生产，2／12 不重掷。"
            : caravansAttack
              ? "本站三模块适配：15 分获胜，商队用木材或砖块出价；先完成道路骑士移动和战斗，再投票放马车。不使用海上蛮族或强盗。"
              : riversTransport
                ? "本站三模块适配：15 分获胜；商品地块正常生产，2／12 不重掷，建桥领 2 金币，贫穷不扣分。可付 5 金币保城。"
                : riversAttack
                  ? "本站三模块适配：13 分获胜；保留河流与桥梁，共用金币，贫穷不扣分。使用道路骑士和进步牌，不使用强盗或海上蛮族。"
                  : riversCaravans
                    ? "本站三模块适配：15 分获胜；木材／砖块出价，保留河流金币、桥梁骑士和 5 金币保城规则。"
                    : attack
                      ? "二至六人，13分获胜。道路骑士在城堡招募、回合末移动；按激活骑士等级战斗，每3个俘虏计1分。船面触发沿海登陆，不使用蛮族船轨道。"
                      : transport
                        ? "二至六人，15分获胜。运输马车与城市骑士并用，蛮族船和道路蛮族分别结算。"
                        : rivers
                          ? "13分获胜；起始河岸村庄和城市均领1金币。商品可换金币，金币只买普通资源；骑士可过桥，5金币可保住被劫掠城市，外交拆河岸道路须付1金币。"
                          : caravans
                            ? "商队出价改用木材和砖块；15分获胜。先放村庄、逆序放城市，首次蛮族进攻前强盗不入场。双人沿用两次事件与生产、中立骑士及最多两辆马车规则。"
                            : explorer
                              ? intro
                                ? "本站初航骑士：二至六人自由开局，先放城市、逆序放港口，13分获胜。无海盗和任务组件；征税没有海盗效果。"
                                : "二至六人；先放城市，再逆序放港口。所选任务目标加5分：巢穴17、鱼群与香料20、三任务22分。"
                              : fishing
                                ? `加入商品、进步牌和骑士；7 鱼可选牌堆抽进步牌。获胜条件按所选剧本，持旧靴者额外需要 1 分。${harbors ? "港口霸主再提高 1 分门槛。" : ""}`
                                : "加入商品、进步牌和骑士，共同抵御蛮族；沿用所选海图的组合胜利条件。"}
      </p>
      {helpers && !explorer && (
        <p className="muted small">
          本站骑士助手：发展卡能力适配进步牌；格雷戈尔归还实体骑士建造，资源助手不处理商品。
        </p>
      )}
      {attack && two && (
        <p className="muted small">
          本站双人组合：每回合两次生产；两家中立道路骑士由当前玩家代办移动与退让，不激活、不参战。贸易筹码有限，损失补偿3金币。
        </p>
      )}
      {explorer && two && value && (
        <p className="muted small">{catanExplorerTwoKnightsNote}</p>
      )}
      {pirateIslands && (
        <p className="muted small">
          本站补充规则：此前已激活的骑士可转为未激活，升级远征线最近的普通船为战舰。首次蛮族进攻后舰队入场；先城市事件、再舰队、最后生产。征税选择陆地但不移动棋子。
        </p>
      )}
      {tribe && (
        <p className="muted small">
          本站补充规则：部落奖励改为从54张进步牌中随机预留，领取前隐藏牌面和类别；领取后按进步牌正常时机使用，胜利点牌立即公开，其余牌遵守回合末四张上限。
        </p>
      )}
    </fieldset>
  );
}

export const supportsPublicCatanFishingSea = (scenario?: string) =>
  [
    "shores",
    "islands",
    "fog",
    "desert",
    "tribe",
    "cloth",
    "wonders",
    "new_world",
  ].includes(scenario || "");

export const supportsPublicCatanFishingSeaExtended = (scenario?: string) =>
  supportsPublicCatanFishingSea(scenario);

export function CatanFishingSeaPicker({
  value,
  onChange,
  disabled = false,
  blocked = false,
  fixedRequired = false,
  shores = false,
  rivers = false,
  caravans = false,
  attack = false,
  transport = false,
  knights = false,
  two = false,
  extended = false,
  explorer = false,
  lakes = false,
  onLakes,
}: {
  value: boolean;
  onChange: (enabled: boolean) => void;
  disabled?: boolean;
  blocked?: boolean;
  fixedRequired?: boolean;
  shores?: boolean;
  rivers?: boolean;
  caravans?: boolean;
  attack?: boolean;
  transport?: boolean;
  knights?: boolean;
  two?: boolean;
  extended?: boolean;
  explorer?: boolean;
  lakes?: boolean;
  onLakes?: (enabled: boolean) => void;
}) {
  return (
    <fieldset
      className="catan-helper-options"
      disabled={disabled || blocked || fixedRequired}
    >
      <label>
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
        />
        渔夫＋
        {transport
          ? "运输任务"
          : attack
            ? "蛮族进攻"
            : caravans
              ? "商队"
              : rivers
                ? "河流"
                : explorer
                  ? "探索者与海盗"
                  : knights
                    ? "城市与骑士"
                    : "航海家"}
      </label>
      <p className="muted small">
        {transport
          ? "不放湖泊；2 鱼或 1 粮食让马车增加 2 移动点，每回合共用一次。12 分获胜，城市骑士组合 15 分，旧靴多需 1 分；双人以鱼替代贸易筹码。可叠加事件牌。"
          : attack
            ? "不放湖泊；2鱼在回合末延长己方骑士移动到5步，7鱼购买并立即使用专用发展卡。被征服建筑不产鱼；可叠加城市骑士与事件牌。"
            : caravans
              ? "湖泊替换水源旁的森林，2和12共用地块。12分获胜，旧靴多需1分；可叠加城市骑士与事件牌，骑士组合15分。本站双人以鱼替代贸易筹码，五六人采用双湖及两组2/12。"
              : rivers
                ? "保留河流地图，不放湖泊；6鱼免费建桥并领取3金币。可叠加城市骑士与事件牌；双人以鱼筹码替代贸易筹码，建桥后照常安排中立建设。"
                : knights
                  ? "双人使用鱼筹码替代贸易筹码，起始建筑不另领鱼；中立骑士和两次城市事件照常保留，可叠加事件牌、友善强盗和港口霸主。"
                  : explorer
                    ? "渔夫鱼筹码与船运鱼群分开计算。2鱼免海盗通行费，5鱼修路或造船，7鱼让一艘船再次航行；可叠加城市骑士与事件生产牌。"
                    : fixedRequired
                      ? "先将海图切为固定布局，才能加入渔夫。"
                      : blocked
                        ? "先关闭城市骑士，才能加入渔夫海图；可叠加 Helpers、港口霸主与友善强盗。"
                        : extended
                          ? "五六人扩大地图、8处渔场、44枚鱼筹码和配对回合；六岛、沙漠、部落、布匹采用标明的本站配方。关闭渔夫后保留海图和人数。"
                          : "海岸渔场产鱼，可花5鱼修路或造船；旧靴提高1分门槛，保留所选海图的特殊终局条件。"}
      </p>
      {shores && value && (
        <p className="muted small">
          {two
            ? "本站双人新海岸：采用四人地图，以主岛内陆沙漠换湖，强盗从场外开始。"
            : "本站新海岸配方：三人以内陆地块换湖并移除原数字；四至六人以内陆沙漠换湖，五六人使用双湖。强盗从场外开始。"}
        </p>
      )}
      {explorer && value && onLakes && (
        <label>
          <input
            type="checkbox"
            checked={lakes}
            onChange={(e) => onLakes(e.target.checked)}
          />
          加入湖泊
          {extended ? "（本站五六人双湖配方）" : "（替换起始岛12点山地）"}
        </label>
      )}
    </fieldset>
  );
}

export const isPublicCatanExplorer = (scenario?: string) =>
  [
    "land-ho",
    "spices-for-catan",
    "pirate-lairs",
    "fish-for-catan",
    "explorers-and-pirates",
  ].includes(scenario || "");

export const isPublicCatanFlexible = (scenario?: string) =>
  scenario === "caravans-new-world" ||
  scenario === "caravans-islands" ||
  scenario === "caravans-shores" ||
  ["caravans-desert", "caravans-tribe"].includes(scenario || "") ||
  isPublicCatanRiversSea(scenario) ||
  isPublicCatanExplorer(scenario) ||
  scenario === "transport" ||
  [
    "rivers-caravans",
    "rivers-attack",
    "rivers-transport",
    "caravans-attack",
    "caravans-transport",
    "attack-transport",
  ].includes(scenario || "");

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
                s.id === "attack-shores"
                  ? players < 3 ||
                    players > 4 ||
                    helpers ||
                    knights ||
                    fishing ||
                    friendly ||
                    harbors
                  : ["caravans-new-world", "caravans-islands"].includes(s.id)
                    ? players < 2 || players > 6 || knights || fishing
                    : s.id === "caravans-shores"
                      ? players > 6 || knights || fishing
                      : ["caravans-desert", "caravans-tribe"].includes(s.id)
                        ? players < 2 || players > 6 || knights || fishing
                        : isPublicCatanRiversSea(s.id)
                          ? !catanRiversSeaAvailable(s.id, players) ||
                            helpers ||
                            knights ||
                            fishing ||
                            friendly ||
                            harbors
                          : (helpers &&
                              [
                                "barbarian-attack",
                                "transport",
                                "rivers-caravans",
                                "rivers-attack",
                                "rivers-transport",
                                "caravans-attack",
                                "caravans-transport",
                                "attack-transport",
                                "rivers",
                                "caravans",
                              ].includes(s.id)) ||
                            (players > 4 &&
                              (fishing
                                ? !(
                                    [
                                      "rivers",
                                      "caravans",
                                      "barbarian-attack",
                                      "transport",
                                    ].includes(s.id) ||
                                    supportsPublicCatanFishingSeaExtended(
                                      s.id,
                                    ) ||
                                    supportsPublicExplorerKnights(s.id)
                                  )
                                : fiveSix
                                  ? !(
                                      isPublicCatanSea(s.id) ||
                                      isPublicCatanExplorer(s.id) ||
                                      [
                                        "",
                                        "cities-knights",
                                        "rivers",
                                        "caravans",
                                        "fishing",
                                      ].includes(s.id)
                                    )
                                  : ![
                                      "land-ho",
                                      "spices-for-catan",
                                      "pirate-lairs",
                                      "fish-for-catan",
                                      "explorers-and-pirates",
                                      "barbarian-attack",
                                      "transport",
                                      "rivers-caravans",
                                      "rivers-attack",
                                      "rivers-transport",
                                      "caravans-attack",
                                      "caravans-transport",
                                      "attack-transport",
                                    ].includes(s.id))) ||
                            (fishing &&
                              isPublicCatanSea(s.id) &&
                              !supportsPublicCatanFishingSea(s.id)) ||
                            (knights &&
                              isPublicCatanSea(s.id) &&
                              !supportsPublicCatanKnightsCombination(s.id))
              }
            >
              {players > 4 && s.id === "islands" ? "航海家 · 六岛" : s.name}
            </option>
          ))}
        </select>
      </label>
      <p className="muted small">
        {fishing && value === "transport"
          ? `渔夫＋运输：${knights ? 15 : 12} 分获胜，持旧靴多需 1 分；双人以鱼筹码替代贸易筹码。`
          : harbors || (knights && supportsPublicCatanKnightsCombination(value))
            ? catanScenarioVictory(
                value,
                catanVictoryTarget(
                  value,
                  knights || value === "cities-knights",
                ) + (harbors ? 1 : 0),
              )
            : scenarios.find((s) => s.id === value)?.description}
      </p>
      {players > 4 &&
        ["rivers", "caravans", "shores", "fishing"].includes(value) && (
          <p className="muted small">
            本站数字配置：采用固定数列沿逆时针螺旋摆放，数字数量保持原扩充配置；不宣称与2025实体字母背面一致。
          </p>
        )}
      {players > 4 && value === "transport" && !knights && (
        <p className="muted small">
          本站牌组配置：五六人增加8张骑士、2张道路建设、2张快速旅程，共37张；按实际开局人数使用。
        </p>
      )}
      {["pirate-lairs", "fish-for-catan", "explorers-and-pirates"].includes(
        value,
      ) && (
        <p className="muted small">
          本站巢穴数字配置：3、4、5、9、10、11，五六人再加入6、8；随机分配，攻陷前隐藏。
        </p>
      )}
      {value === "transport" && knights && (
        <p className="muted small">
          运输＋城市骑士：不使用运输发展牌，15分获胜；保留蛮族船与道路蛮族。二至四人2或12只重掷生产骰。本站组合说明：五六人地图同样用一块粮田替换森林；小地图炼金术仅选3至11点。
        </p>
      )}
      {["rivers", "transport"].includes(value) && (
        <p className="muted small">本站补充：金币用完继续记账发放。</p>
      )}
    </div>
  );
}
