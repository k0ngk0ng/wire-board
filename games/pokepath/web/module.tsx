import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import catalog from "./catalog";
import type { Options, StreamView } from "./types";
import "./style.css";
import "./classic.css";
import "./mobile.css";

export default defineGame<StreamView, Options>({
  ...catalog,
  asyncBoard: true,
  assetLoading: "board",
  viewportFit: false,
  resources: () => [],
  defaultOptions: { starter: "charmander" },
  Board,
  PlayerSummary: ({ data, seat }) => <div className="pokepath-player-summary">
    <strong data-pokepath-gold>金币 {data.balances?.[seat] ?? data.trainers?.[seat]?.gold ?? data.gold ?? 0}</strong>
    <span data-pokepath-health>生命 {data.health ?? 0}</span>
    <span>第 {data.wave ?? 1} 波</span>
  </div>,
  Rules: () => <div className="pokepath-rules">
    <p>宝可梦就是塔。1～4 人合作守同一张地图，每人各有 50 金币和小火龙、木守宫、呱呱泡蛙，独立管理队伍、仓库和商店；共享生命与波次。宝可梦自动攻击经过小径的敌人，漏过的敌人会消耗生命；普通路线守住 100 波即可通关。</p>
    <ol>
      <li>点队伍里的宝可梦卡片，或卡片上的精灵球按钮，选中要部署的伙伴。</li>
      <li>点地图上亮起的可建造位置完成部署。不同伙伴适应不同地形，例如小火龙放在草地，不能放在道路或水面上。</li>
      <li>点「下一波」开始防守。击败敌人获得金币，点卡片上的「+1」升级伙伴；也可开启自动波次连续推进。</li>
    </ol>
    <p>点击场上的宝可梦可查看、升级、改变目标优先级或收回。地图可拖动平移、双指缩放；鼠标滚轮或「＋／−」调整比例，「全图」查看整张地图。</p>
    <p>击败敌人获得金币，花费金币升级、进化或在商店招募伙伴。队伍、仓库、背包和图鉴保留原版内容，地形、道具、天气和特殊能力共同影响布阵。多人合作各自获得击杀与波次金币，花费只扣本人余额，队友的宝可梦只读；路线进度共同推进。</p>
    <p>守过波次获得星数并解锁路线，继续挑战关卡和首领。可以重复刷波、切换路线、保存编队，也可以组合限制条件挑战奖带。暂停、自动波次和速度控制位于战场内；托管、全屏和保存退出沿用房间入口。</p>
    <p>生命耗尽后，「重新开始」回到当前路线第 1 波，恢复 14 生命；「重试」从第 25 波起可用，回到最近的第 25／50／75 波检查点，通常恢复 7 生命，星数奖励可增加生命。第 100 波失败则从第 100 波以 1 生命重试。两种操作都保留队伍、等级、金币、道具和合法部署。</p>
  </div>,
});
