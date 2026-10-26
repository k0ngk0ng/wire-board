import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import type { Options, View } from "./types";
import "./style.css";
import "./viewport.css";

export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  defaultOptions: {
    mode: "campaign",
    waves: 10,
    difficulty: "normal",
    prepSeconds: 30,
  },
  Board,
  Cover,
  PlayerSummary: ({ room, data, seat }) => (
    <div className="sf-seat-summary">
      <small>
        {!room.spectating && room.you === seat ? "你的金币" : "金币"}
      </small>
      <strong data-sf-gold={seat}>
        <span aria-hidden="true">◈ </span>
        {data.players[seat]?.gold ?? 0}
      </strong>
      <span>
        {data.players[seat]?.ready
          ? "已准备"
          : `防御塔 ${data.towers.filter((t) => t.owner === seat).length} 座`}
      </span>
    </div>
  ),
  Options: ({ value, onChange, disabled }) => (
    <div className="sf-options">
      <label>
        守望模式
        <select
          aria-label="守望模式"
          value={
            value.mode === "endless" ? "endless" : `campaign${value.waves}`
          }
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...value,
              mode: e.target.value === "endless" ? "endless" : "campaign",
              waves: e.target.value === "campaign20" ? 20 : 10,
            })
          }
        >
          <option value="campaign10">闯关 · 10 波</option>
          <option value="campaign20">闯关 · 20 波</option>
          <option value="endless">无尽守望</option>
        </select>
      </label>
      <label>
        难度
        <select
          aria-label="难度"
          value={value.difficulty}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...value,
              difficulty: e.target.value as Options["difficulty"],
            })
          }
        >
          <option value="easy">简单</option>
          <option value="normal">普通</option>
          <option value="hard">困难</option>
        </select>
      </label>
      <label>
        波间准备
        <select
          aria-label="波间准备"
          value={value.prepSeconds}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...value,
              prepSeconds: Number(e.target.value) as Options["prepSeconds"],
            })
          }
        >
          {[20, 30, 45].map((s) => (
            <option key={s} value={s}>
              {s} 秒
            </option>
          ))}
        </select>
      </label>
      <p>
        每 5 波后选择团队成长，第 6、11、16
        波扩展地图。完成选择后全员准备可提前出波。基地和胜负共享，金币各自持有；波间可投票暂停，断线后可回到原局。
      </p>
    </div>
  ),
  Rules: () => (
    <div className="sf-rules">
      <p>
        1～4
        人共同守护星核。闯关守完设定波数且基地仍有生命，全队获胜；无尽持续抵御敌潮，直到基地生命归零，记录守住的波数和团队击杀。
      </p>
      <ol>
        <li>
          选择地图上的建造点，再建造防御塔。每人拥有自己的金币和塔；击杀、过波奖励由全队分享，无需抢最后一击。
        </li>
        <li>
          箭塔快速攻击单体，炮塔爆炸伤害成群敌人，冰塔减速，法师塔穿透护甲。塔会自动瞄准射程内的敌人。
        </li>
        <li>
          可以升级或出售自己的塔。三级后选择威力或攻速分支，之后仍可继续强化。金币可赠予队友。
        </li>
        <li>
          波次开始前公布敌群与天气事件。第 6、11、16 波选择扩展路线；守过每 5
          波选择团队成长。按全员多数票确定，平票选择靠前方案，超时按已有票数确定。所有选择完成后重新给足整备时间，全员准备或倒计时结束开波。战斗中仍可建造、升级、出售。
        </li>
        <li>
          敌人到达终点会扣除基地生命。敌潮逐步增强，每 5 波精英，每 10
          波首领；后期提高敌人质量，同屏数量有上限。
        </li>
        <li>
          扩展保留已有塔和建造点，并开放新入口、高地与支线。高地增加 15%
          射程；浓雾缩短射程，寒潮减慢敌人，星辉强化法师，多路突袭开放所有入口。界面提前预告后两波事件。无尽模式展开全部战区后继续轮换入口与事件。
        </li>
        <li>
          团队成长可增加伤害、射程或金币收益，能够叠加。支线由一人出资、全队受益：守护矿场一波获得金币，修复瞭望塔并守一波增加射程，护送补给队抵达星核获得金币并修复星核。据点和补给队有独立生命，附近敌人会攻击它们；生命归零则目标失败。
        </li>
        <li>
          治疗者、旗手、分裂怪和工兵带来不同威胁；首领半血时进入新阶段并召来护卫。自己的塔可选择最接近星核、最强敌人或支援敌人优先攻击。选中塔可查看计入地形、成长和事件后的实际属性。
        </li>
        <li>
          波间全员同意可暂停并自动保存；全员同意后恢复。连续 24
          小时无人操作的房间会自动关闭。观众可以查看战场，不能操作。合作胜负不改变竞争积分。
        </li>
      </ol>
    </div>
  ),
});
