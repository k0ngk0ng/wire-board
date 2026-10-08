import { catanRuleContext } from "./catan-rule-context";
import {
  catanLayoutName,
  catanScenarioName,
  catanScenarioVictory,
} from "./catan-scenarios";
import type { Room } from "./types";
import "./catan-scenarios.css";

export function CatanCitiesKnightsSetup({ room }: { room: Room }) {
  if (!room.catanCitiesKnights) return null;
  const info = catanRuleContext(room);
  return (
    <section
      className="catan-helper-options catan-scenario-options"
      aria-label="城市与骑士设置"
    >
      <h3>城市与骑士</h3>
      <p>
        {info.scenario
          ? `${info.explorer ? "" : "航海家 · "}${catanScenarioName(info.scenario)} · ${catanLayoutName(info.layout)}`
          : info.fishing
            ? "渔夫 · 随机湖泊与海岸渔场"
            : "随机地图"}{" "}
        · {catanScenarioVictory(info.scenario, info.target)}
      </p>
      <p>
        {info.explorer
          ? "先放城市，再逆序放港口，随后放道路与移民船；港口不计入蛮族兵力，船员不能代替骑士防御。"
          : info.scenario === "cloth"
            ? "顺序村庄、逆序城市、再顺序村庄，仅第三座领取普通起始资源。"
            : "先放村庄，再逆序放城市。"}
        建设城墙和大都会，派出骑士抵御蛮族，运用科学、贸易和政治进步牌。
      </p>
      <small>
        使用纸张、布料和钱币三种商品；本扩展不使用基础发展卡和最大骑士军队。
      </small>
    </section>
  );
}
