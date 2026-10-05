import type { Room } from "./types";
import "./catan-scenarios.css";

export function CatanCitiesKnightsSetup({ room }: { room: Room }) {
  if (!room.catanCitiesKnights) return null;
  return (
    <section
      className="catan-helper-options catan-scenario-options"
      aria-label="城市与骑士设置"
    >
      <h3>城市与骑士</h3>
      <p>随机地图 · 13 分获胜</p>
      <p>
        先放村庄，再逆序放城市。建设城墙和大都会，派出骑士抵御蛮族，运用科学、贸易和政治进步牌。
      </p>
      <small>
        使用纸张、布料和钱币三种商品；本扩展不使用基础发展卡和最大骑士军队。
      </small>
    </section>
  );
}
