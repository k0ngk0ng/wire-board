import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import type { Options, View } from "./types";
import "./style.css";

export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  defaultOptions: {
    mapId: "europe",
    difficulty: "normal",
    turnSeconds: 120,
    fog: false,
  },
  Board,
  Options: ({ value, onChange, disabled }) => (
    <div className="cq-options">
      <label>
        地图
        <select
          aria-label="征服地图"
          disabled={disabled}
          value={value.mapId}
          onChange={(e) =>
            onChange({ ...value, mapId: e.target.value as Options["mapId"] })
          }
        >
          <option value="europe">欧洲</option>
          <option value="world">世界</option>
        </select>
      </label>
      <label>
        电脑难度
        <select
          aria-label="征服电脑难度"
          disabled={disabled}
          value={value.difficulty}
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
        每轮规划
        <select
          aria-label="征服规划时限"
          disabled={disabled}
          value={value.turnSeconds}
          onChange={(e) =>
            onChange({
              ...value,
              turnSeconds: Number(e.target.value) as Options["turnSeconds"],
            })
          }
        >
          {[60, 120, 180].map((v) => (
            <option key={v} value={v}>
              {v} 秒
            </option>
          ))}
        </select>
      </label>
      <label className="cq-fog-option">
        <input
          type="checkbox"
          checked={!!value.fog}
          disabled={disabled}
          onChange={(e) => onChange({ ...value, fog: e.target.checked })}
        />
        战争迷雾
      </label>
    </div>
  ),
  PlayerSummary: ({ room, data, seat }) => {
    const p = data.players[seat];
    if (!p) return null;
    return (
      <div className="cq-player-summary" data-cq-player={seat}>
        <span>
          <small>
            {!room.spectating && room.you === seat ? "你的金币" : "金币"}
          </small>
          <strong data-cq-gold={seat}>
            {!room.spectating && room.you === seat
              ? p.gold.toLocaleString("zh-CN")
              : "?"}
          </strong>
        </span>
        <span>
          <small>领土</small>
          <b>{p.territories}</b>
          <small>兵力</small>
          <b>{p.troops < 0 ? "?" : p.troops.toLocaleString("zh-CN")}</b>
        </span>
        <span className="cq-player-state">
          {p.eliminated
            ? "已出局"
            : data.phase === "paused"
              ? "已暂停"
              : p.submitted
                ? "已提交"
                : "规划中"}
        </span>
      </div>
    );
  },
  Rules: () => (
    <div className="cq-rules">
      <p>
        选择世界或欧洲地图，经营领土并消灭所有敌对势力。各势力同时规划，全部存活玩家提交后按服务器次序结算。
      </p>
      <ol>
        <li>
          点选己方领地后，通过地图内图标征兵、进攻或筑堡。进攻时点相邻领地派出剩余可移动兵力的一半，再点继续追加剩余的一半。每次派兵仍消耗行动点，跨海另需金币。加入计划不会立即改变战场。
        </li>
        <li>
          征兵短按最多增加一千兵，长按一次征满；筑堡长按可直接建城镇。Shift+Enter
          等价长按。征募、建设与行军都要预留金币或行动点，计划面板可撤销指令。
        </li>
        <li>
          提交后可以在结算开始前撤回修改。所有人提交或期限到达后推进；超时使用平台托管，回来可随时接管。
        </li>
        <li>
          开启战争迷雾后，未知军力显示问号；瞭望塔揭示邻近地区。未开启时全部兵力公开。
        </li>
        <li>
          军队需要维护，税率影响财政和民心。国政面板展示预计收入与支出。战斗、伤亡和领地归属由服务器决定。
        </li>
        <li>
          存活真人一致同意后暂停并保存，恢复也需一致同意。暂停保留当前计划与剩余操作时间，最多保留
          30 天。
        </li>
        <li>
          含电脑的对局不计竞技积分。观众可以查看公开战场，不能查看任何玩家尚未执行的计划。
        </li>
      </ol>
    </div>
  ),
});
