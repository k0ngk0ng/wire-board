import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { createPlayback } from "./presentation";
import type { Options, View } from "./types";
import "./style.css";

export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  assetLoading: "board",
  createPresentation: createPlayback,
  defaultOptions: {
    turnSeconds: 25,
    preparationSeconds: 180,
    maxRounds: 25,
    teams: [],
  },
  Board,
  Options: ({ value, onChange, capacity, disabled }) => (
    <div className="sa-options">
      <label>
        每回合时限
        <select
          value={value.turnSeconds}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...value,
              turnSeconds: Number(e.target.value),
              teams: value.teams.length === capacity ? value.teams : [],
            })
          }
        >
          {[20, 25, 30, 45].map((n) => (
            <option key={n} value={n}>
              {n} 秒
            </option>
          ))}
        </select>
      </label>
      <label>
        选配时间
        <select
          value={value.preparationSeconds}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...value,
              preparationSeconds: Number(e.target.value),
              teams: value.teams.length === capacity ? value.teams : [],
            })
          }
        >
          {[120, 180, 240, 300].map((n) => (
            <option key={n} value={n}>
              {n / 60} 分钟
            </option>
          ))}
        </select>
      </label>
      <label>
        回合上限
        <select
          value={value.maxRounds}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...value,
              maxRounds: Number(e.target.value),
              teams: value.teams.length === capacity ? value.teams : [],
            })
          }
        >
          {[15, 20, 25, 30].map((n) => (
            <option key={n} value={n}>
              {n} 回合
            </option>
          ))}
        </select>
      </label>
    </div>
  ),
  Preparation: ({ room, options, host, busy, prepare }) => (
    <div className="sa-preparation">
      <div>
        {room.seats.map((seat, i) => (
          <label key={seat.id}>
            {seat.name}
            <select
              aria-label={`${seat.name}的队伍`}
              disabled={busy || (!host && room.you !== i)}
              value={options.teams[i] ?? i % 2}
              onChange={(e) =>
                void prepare({
                  type: "team",
                  seat: i,
                  team: Number(e.target.value),
                })
              }
            >
              <option value={0}>红队</option>
              <option value={1}>蓝队</option>
            </select>
          </label>
        ))}
      </div>
    </div>
  ),
  PlayerSummary: ({ room, data, seat }) => {
    const p = data.players[seat];
    if (!p) return null;
    return (
      <div className="sa-player-summary" data-stoneage-player={seat}>
        {data.phase === "selection" ? (
          <span>
            {p.team === 0 ? "红队" : "蓝队"} · {p.locked ? "已锁定" : "选配中"}
          </span>
        ) : (
          <>
            <span>
              人物{" "}
              <strong>
                {p.body.hp}/{p.body.maxHp}
              </strong>
            </span>
            <span>
              宠物{" "}
              <strong>
                {p.pet.fled ? "已逃跑" : `${p.pet.hp}/${p.pet.maxHp}`}
              </strong>
            </span>
            <span className="sa-summary-ride">
              骑宠{" "}
              <strong>
                {p.riding ? `${p.ride.hp}/${p.ride.maxHp}` : "已落马"}
              </strong>
            </span>
            {!room.spectating && room.you === seat && (
              <span>
                气 {p.body.mp}/{p.body.maxMp}
              </span>
            )}
          </>
        )}
      </div>
    );
  },
  Rules: () => (
    <div>
      <p>
        双方使用同一份随机候选，各自独立选配。支持 1v1、2v2、3v3、4v4 与
        5v5，每位玩家控制一个人物、一只出战宠及一只骑宠。人物开局有相同的 480
        点属性点，可自由分配体力、腕力、耐力和速度。
      </p>
      <ol>
        <li>
          帽子、武器和衣服各选一件，饰品最多两件。回血、回气、解除异常补给全部保底；战术道具最多两份。
        </li>
        <li>
          出战宠最多学习七项技能，含攻击与防御；同族技能选择一档。锁定后不能更换配置。
        </li>
        <li>
          出战宠候选随机 0～100 忠诚度，骑宠固定为
          100，双方共用相同结果，选定后不能修改。出战宠忠诚低于 80
          可能拒令、改变目标或技能；低于 20 可能攻击主人，低于 10
          还可能逃跑。骑宠不独立接收出战指令。
        </li>
        <li>
          每轮先选择人物与出战宠的指令，再按敏捷顺序逐个行动；相近敏捷存在随机波动。地克水、水克火、火克风、风克地。
        </li>
        <li>
          骑宠影响人物的攻击、防御和敏捷，并分担伤害。骑宠死亡或被落马术命中后退出骑乘，本场不能再次骑乘。
        </li>
        <li>
          每人只有一只出战宠，没有替补、换宠与复活。人物全灭的一方失败；达到回合上限时和局。
        </li>
        <li>
          补给与装备精灵占用人物行动，气力和物品有限。超时由平台托管，可随时接管；含电脑的对局不计积分。
        </li>
      </ol>
    </div>
  ),
  ResultDetails: ({ room, data }) => (
    <div className="sa-result-details">
      <p>
        {data.reason === "round_limit"
          ? "达到回合上限，双方和局。"
          : "双方人物的存活情况决定团队胜负。"}
      </p>
      <table>
        <thead>
          <tr>
            <th>玩家</th>
            <th>伤害</th>
            <th>治疗</th>
            <th>击倒</th>
          </tr>
        </thead>
        <tbody>
          {data.players.map((p) => (
            <tr key={p.seat}>
              <td>{room.seats[p.seat]?.name}</td>
              <td>{p.damage}</td>
              <td>{p.healing}</td>
              <td>{p.kills}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  ),
});
