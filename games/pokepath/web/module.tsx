import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import catalog from "./catalog";
import type { Options, StreamView } from "./types";
import "./style.css";
import "./classic.css";

export default defineGame<StreamView, Options>({
  ...catalog,
  asyncBoard: true,
  assetLoading: "board",
  viewportFit: true,
  resources: () => [],
  defaultOptions: { starter: "charmander" },
  Board,
  Options: ({ value, onChange, disabled }) => <div className="pokepath-options">
    <label>初始伙伴<select aria-label="初始伙伴" disabled={disabled} value={value.starter ?? "charmander"} onChange={event => onChange({ ...value, starter: event.target.value as Options["starter"] })}>
      <option value="charmander">小火龙</option>
      <option value="treecko">木守宫</option>
      <option value="froaki">呱呱泡蛙</option>
    </select></label>
  </div>,
  PlayerSummary: ({ data }) => <div className="pokepath-player-summary">
    <strong data-pokepath-gold>金币 {data.gold ?? 0}</strong>
    <span data-pokepath-health>生命 {data.health ?? 0}</span>
    <span>第 {data.wave ?? 1} 波</span>
  </div>,
  Rules: () => <div className="pokepath-rules">
    <p>选择队伍中的宝可梦，再点选合适地形部署。点击场上的宝可梦可查看、升级、改变目标优先级或收回。宝可梦自动攻击经过小径的敌人，漏过的敌人会消耗生命。</p>
    <p>击败敌人获得金币，花费金币升级、进化或在商店招募伙伴。队伍、仓库、背包和图鉴保留原版内容，地形、道具、天气和特殊能力共同影响布阵。多人合作共享金币、队伍和路线进度。</p>
    <p>守过波次获得星数并解锁路线，继续挑战关卡和首领。可以重复刷波、切换路线、保存编队，也可以组合限制条件挑战奖带。暂停、自动波次和速度控制位于战场内；托管、全屏和保存退出沿用房间入口。</p>
  </div>,
});
