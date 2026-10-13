import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import type { Options, View } from "./types";
import "./style.css";

export default defineGame<View, Options>({
  id: "starfort",
  name: "星垒",
  description:
    "筑塔守住星光。1～4 人携手抵御敌潮，搭配箭塔、火炮、冰晶与魔法，挑战闯关或无尽守望。",
  minPlayers: 1,
  maxPlayers: 4,
  supportsBots: true,
  ownsSounds: true,
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
        全员准备可提前出波。基地和胜负共享，金币各自持有；波间可全员投票暂停，断线后可回到原局。
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
          波次开始前公布敌群规则。所有人准备后立即开波，倒计时结束也会自动开波。战斗中仍可建造、升级、出售。
        </li>
        <li>
          敌人到达终点会扣除基地生命。敌潮逐步增强，每 5 波精英，每 10
          波首领；后期提高敌人质量，同屏数量有上限。
        </li>
        <li>
          波间全员同意可暂停并自动保存；全员同意后恢复。连续 24
          小时无人操作的房间会自动关闭。观众可以查看战场，不能操作。合作胜负不改变竞争积分。
        </li>
      </ol>
    </div>
  ),
});
