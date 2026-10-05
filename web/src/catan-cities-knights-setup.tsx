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
          ? `航海家 · ${catanScenarioName(info.scenario)} · ${catanLayoutName(info.layout)}`
          : "随机地图"}{" "}
        · {catanScenarioVictory(info.scenario, info.target)}
      </p>
      <p>
        {info.scenario === "cloth"
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
