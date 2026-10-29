import catalog from "./catalog";
import { useEffect } from "react";
import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { trackNames, seatColors, type Options, type View } from "./types";
function OptionsForm({ value, onChange, disabled }: OptionsProps<Options>) {
  useEffect(() => {
    void import("./chase-renderer").catch(() => {});
  }, []);
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
  ...catalog,
  asyncBoard: true,
  defaultOptions: { track: "metro", laps: 3 },
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
        2～6 人的追车视角竞速，全自动挡。3 秒发车灯后同时起步，按所选的 1、3 或
        5
        圈比赛，按最终过线先后排名。六款经典车型的性能、抓地力与碰撞尺寸相同，选择只改变外观。
      </p>
      <p>
        必须沿赛道正向依次通过检查点；逆穿终点和抄近路不会增加圈数。草地、砂地和道路外区域会明显减速。车身可以相撞，没有攻击或防御道具。
      </p>
      <p>
        首车冲线后，其他车辆有 30 秒完赛；比赛总时限为 20
        分钟。未完赛者按有效赛道进度排序，完全没有进度时为和局。计时与圈数由服务器判定。
      </p>
      <p>
        默认键位：W／↑ 加速，S／↓ 刹车，A／D 或 ←／→ 转向，空格手刹，按住 B
        向后看，R
        重置，可在「控制器设置」修改。手机：滑动方向盘转向，踏板从左到右为手刹、刹车、油门。按住油门加速，松开后惯性滑行，可多指同时操作。离开页面会释放操作；空闲不会自动托管，可随时手动开启托管。
      </p>
      <p>
        重置须先减速，回到最近有效检查点，并等待 2 秒重新起步；每次重置后冷却 5
        秒。点开赛道图后可拖动、滚轮或双指缩放，小地图支持拖动定位。
      </p>
      <details>
        <summary>素材与许可</summary>
        <p>
          赛道与立体车辆为项目绘制，地图总览沿用{" "}
          <a
            href="https://kenney.nl/assets/racing-pack"
            target="_blank"
            rel="noreferrer"
          >
            Kenney Racing Pack
          </a>
          （CC0）；封面原创；赛车引擎为 domasx2 的 CC0 素材，轮胎声为 Tom
          Haigh／qubodup 的 CC BY 3.0
          素材，经过裁剪和音量调整。全部音效由平台统一静音控制。
        </p>
        <p>
          <a
            href="/racing/v2/attribution.json"
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
