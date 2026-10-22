import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import type { View } from "./types";
import "./style.css";
export default defineGame<View>({
  ...catalog,
  defaultOptions: {},
  Board,
  Cover,
  PlayerSummary: ({ data, seat }) => {
    const p = data.players[seat];
    return (
      <div className="pairs-progress">
        <strong>
          剩余 {p?.remaining ?? data.total} / {data.total}
        </strong>
        <progress
          aria-label="消除进度"
          max={data.total}
          value={data.total - (p?.remaining ?? data.total)}
        />
        <span>
          {data.winner === seat
            ? "率先通关"
            : p?.combo > 1
              ? `${p.combo} 连击`
              : "同图竞速"}
        </span>
      </div>
    );
  },
  Rules: () => (
    <div>
      <p>
        每局随机地图和难度，所有玩家同时看到相同初始棋盘，独立消除；最先清空者获胜，其他玩家并列第二。单人可练习通关速度。
      </p>
      <p>
        两个相同且未冰封的图案，可用不超过两次转弯的线路连接。线路可经过棋盘外侧，不能穿过图案或障碍。相邻配对会击碎木箱、解冻冰层；石块需要绕行或用锤子破除。
      </p>
      <p>
        每人有 3 次提示、2 次重排、2
        次破障锤。无解自动免费重排，不消耗道具。连击只作反馈，不按分数或剩余图案提前判胜。120
        秒无操作自动托管，取消托管可继续操作。
      </p>
    </div>
  ),
});
