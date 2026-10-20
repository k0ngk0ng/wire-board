import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { trackNames, seatColors, type Options, type View } from "./types";
function OptionsForm({ value, onChange, disabled }: OptionsProps<Options>) {
  return (
    <div className="racing-options">
      <label>
        赛道
        <select
          aria-label="赛道"
          disabled={disabled}
          value={value.track}
          onChange={(e) =>
            onChange({ ...value, track: e.target.value as Options["track"] })
          }
        >
          {Object.entries(trackNames).map(([id, name]) => (
            <option key={id} value={id}>
              {name}
            </option>
          ))}
        </select>
      </label>
      <label>
        比赛圈数
        <select
          aria-label="比赛圈数"
          disabled={disabled}
          value={value.laps}
          onChange={(e) =>
            onChange({ ...value, laps: +e.target.value as Options["laps"] })
          }
        >
          <option value={1}>1 圈 · 热身</option>
          <option value={3}>3 圈 · 标准赛</option>
          <option value={5}>5 圈 · 耐力赛</option>
        </select>
      </label>
      <p>
        2～6 人同时竞速，可添加电脑。所有车型性能相同；首车过线后，其余车辆有 30
        秒完成比赛。
      </p>
    </div>
  );
}
export default defineGame<View, Options>({
  id: "racing",
  name: "逐风赛车",
  description: "驾车穿过林间、海岸与峡谷，掌握刹车与漂移，争夺终点。",
  category: "sports",
  minPlayers: 2,
  maxPlayers: 6,
  playerCounts: [2, 3, 4, 5, 6],
  supportsBots: true,
  ownsSounds: true,
  realtime: true,
  defaultOptions: { track: "woodland", laps: 3 },
  Board,
  Cover,
  Options: OptionsForm,
  PlayerSummary: ({ room, data: d, seat }) => {
    const c = d.cars.find((c) => c.seat === seat);
    if (!c) return null;
    return (
      <div className="racing-player-summary" data-racing-seat={seat}>
        <strong>
          <i aria-hidden="true" style={{ background: seatColors[seat] }} />
          {seat + 1} 号车 · 第 {c.rank} 名
          {room.you === seat && !room.spectating ? " · 你" : ""}
        </strong>
        <span>
          {c.finishAt
            ? "已冲线"
            : `${Math.min(c.lap + 1, d.options.laps)} / ${d.options.laps} 圈`}
          {c.offroad && !c.finishAt ? " · 路外减速" : ""}
        </span>
      </div>
    );
  },
  Rules: () => (
    <div className="racing-rules">
      <p>
        2～6 人的俯视竞速。3 秒发车灯后同时起步，按所选的 1、3 或 5
        圈比赛，按最终过线先后排名。五款车身性能、抓地力与碰撞尺寸完全相同。
      </p>
      <p>
        必须沿赛道正向依次通过检查点；逆穿终点和抄近路不会增加圈数。草地、砂地和道路外区域会明显减速。车身可以相撞，没有攻击或防御道具。
      </p>
      <p>
        首车冲线后，其他车辆有 30 秒完赛；比赛总时限为 8
        分钟。未完赛者按有效赛道进度排序，完全没有进度时为和局。计时与圈数由服务器判定。
      </p>
      <p>
        电脑：W／↑ 加速，S／↓ 刹车，A／D 或 ←／→ 转向，空格漂移，R
        救援。手机：默认自动加速，滑动方向盘转向，可同时按住漂移或刹车。离开页面会释放操作；空闲不会自动托管，可随时手动开启托管。
      </p>
      <p>
        救援须先减速，回到最近有效检查点，并等待 2 秒重新起步；每次救援后冷却 5
        秒。拖动赛道查看地图，滚轮或双指缩放，「跟车」恢复驾驶镜头；小地图支持拖动定位。
      </p>
      <details>
        <summary>素材与许可</summary>
        <p>
          车辆、场景来自{" "}
          <a
            href="https://kenney.nl/assets/racing-pack"
            target="_blank"
            rel="noreferrer"
          >
            Kenney Racing Pack
          </a>
          （CC0），封面原创；赛车引擎为 domasx2 的 CC0 素材，轮胎声为 Tom
          Haigh／qubodup 的 CC BY 3.0
          素材，经过裁剪和音量调整。全部音效由平台统一静音控制。
        </p>
        <p>
          <a
            href="/racing/v1/attribution.json"
            target="_blank"
            rel="noreferrer"
          >
            完整素材来源与署名
          </a>
        </p>
      </details>
    </div>
  ),
});
