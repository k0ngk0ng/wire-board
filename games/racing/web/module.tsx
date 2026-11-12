import catalog from "./catalog";
import {
  defineGame,
  type OptionsProps,
  type PreparationProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { seatColors, type Options, type View } from "./types";
import {
  classicTrackNames,
  classicVehicles,
  classicVehicleByID,
} from "./classic-catalog";
import { classicLegacySounds } from "./classic-loading";
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
          {Object.entries(classicTrackNames).map(([id, name]) => (
            <option key={id} value={id}>
              {name}
            </option>
          ))}
        </select>
      </label>
      <label>
        时段
        <select
          aria-label="时段"
          disabled={disabled}
          value={value.timeOfDay ?? "day"}
          onChange={(e) =>
            onChange({ ...value, timeOfDay: e.target.value as "day" | "night" })
          }
        >
          <option value="day">白天</option>
          <option value="night">夜间</option>
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
        2～6 人同时竞速，可添加电脑。车型在等待房间选择；首车过线后，其余车辆有
        30 秒完成比赛。
      </p>
    </div>
  );
}
function Preparation({
  room,
  options,
  busy,
  prepare,
}: PreparationProps<Options>) {
  if (room.you < 0) return null;
  const id =
    options.vehicles?.[room.you] ??
    classicVehicles[room.you % classicVehicles.length].id;
  const vehicle = classicVehicleByID(id);
  return (
    <div className="racing-preparation">
      <label>
        车型
        <select
          aria-label="车型"
          value={id}
          disabled={busy}
          onChange={(e) =>
            prepare({ type: "vehicle", vehicle: e.target.value })
          }
        >
          {classicVehicles.map((v) => (
            <option value={v.id} key={v.id}>
              {v.name}
            </option>
          ))}
        </select>
      </label>
      <span aria-label="车型参数">
        {vehicle.mass} kg · {vehicle.maxSpeedKmh} km/h
      </span>
    </div>
  );
}
export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  assetLoading: "board",
  resources: (room, media) => {
    const data = room.game?.data as View | undefined;
    if (
      data?.physics === 3 ||
      (data?.track.id && data.track.id in classicTrackNames)
    )
      return classicLegacySounds;
    return Object.keys(media).filter((path) => path.startsWith("/racing/v1/"));
  },
  defaultOptions: { track: "france", laps: 3, timeOfDay: "day" },
  Board,
  Cover,
  Options: OptionsForm,
  Preparation,
  PlayerSummary: ({ room, data: d, seat }) => {
    const c = d.cars.find((c) => c.seat === seat);
    if (!c) return null;
    return (
      <div className="racing-player-summary" data-racing-seat={seat}>
        <strong>
          <i aria-hidden="true" style={{ background: seatColors[seat] }} />第{" "}
          {c.rank} 名{room.you === seat && !room.spectating ? " · 你" : ""}
        </strong>
        <span>
          {c.vehicle && `${classicVehicleByID(c.vehicle).name} · `}
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
        圈比赛，按最终过线先后排名。五款经典车型具有各自的动力、传动、制动和抓地特性，可在等待房间选择。
      </p>
      <p>
        必须沿赛道正向依次通过检查点；逆穿终点和抄近路不会增加圈数。草地、砂地和道路外区域会明显减速。车身可以相撞，没有攻击或防御道具。
      </p>
      <p>
        首车冲线后，其他车辆有 30 秒完赛；比赛总时限为 20
        分钟。未完赛者按有效赛道进度排序，完全没有进度时为和局。计时与圈数由服务器判定。
      </p>
      <p>
        默认键位：W／↑ 加速，S／↓ 刹车，停稳后继续按住可倒车；A／D 或 ←／→
        转向，空格手刹，R 重置。 C 循环切换近追车、远追车、引擎盖和驾驶舱；按住
        B 向后看，松开恢复所选视角；按住 H 鸣笛。 F3 切换最大绘距，F5
        显示或隐藏仪表，F7 开关后视镜，F11／F12 缩小或放大游戏视区，M
        打开赛道地图。 可在平台「控制器设置」改键。
      </p>
      <p>
        标准映射手柄：左摇杆或十字键转向，右扳机油门、左扳机制动，下方面键手刹、上方面键后视、左方面键重置；L1
        切换视角，R1 鸣笛，Start 打开地图。按钮和轴均可在「控制器设置」调整。
        手机使用画面内左侧的两个转向箭头、右侧的刹车和油门，手刹作为小按钮放在右下边缘；视角、后视和喇叭另有按钮，可多指同时操作。离开页面会释放操作；空闲不会自动托管，可通过平台入口手动开启或取消托管。
      </p>
      <p>
        重置须先减速，回到最近有效检查点，并等待 2 秒重新起步；每次重置后冷却 5
        秒。点开赛道图后可拖动、滚轮或双指缩放，小地图支持拖动定位。
      </p>
      <details>
        <summary>素材与许可</summary>
        <p>
          新版场景、车辆与发动机音频参考《极品飞车 4》PC
          原始资源，格式转换保留来源及摘要；道路与驾驶由项目独立实现。旧版地图总览使用{" "}
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
          <a href="/racing/v3/manifest.json" target="_blank" rel="noreferrer">
            完整素材来源与署名
          </a>
        </p>
      </details>
    </div>
  ),
});
