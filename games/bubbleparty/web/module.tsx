import { resources } from "./loading";
import catalog from "./catalog";
import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import { Board } from "./Board";
import { Cover } from "./Cover";
import { MapPreview } from "./MapPreview";
import { classicMapGroups } from "./classic-map-options";
import { characterName } from "./character-definitions";
import { mountName } from "./mount-art";
import { mapNames, teamNames, type Options, type View } from "./types";

function OptionsForm({ value, onChange, disabled }: OptionsProps<Options>) {
  return (
    <div className="bubble-options">
      <label>
        地图
        <select
          aria-label="水泡战场地图"
          disabled={disabled}
          value={value.map}
          onChange={(event) => onChange({ ...value, map: event.target.value })}
        >
          <optgroup label="现有场地">
            {["community10", "waterpark", "forest", "desert", "random"].map(
              (id) => (
                <option key={id} value={id}>
                  {mapNames[id]}
                </option>
              ),
            )}
          </optgroup>
          {classicMapGroups.map((group) => (
            <optgroup key={group.family} label={group.label}>
              {group.maps.map((map) => (
                <option key={map.id} value={map.id}>
                  {map.name}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </label>
      <MapPreview map={value.map} />
      <label>
        单局时限
        <select
          aria-label="单局时限"
          disabled={disabled}
          value={value.roundSeconds}
          onChange={(event) =>
            onChange({ ...value, roundSeconds: Number(event.target.value) })
          }
        >
          <option value={180}>3 分钟 · 快速局</option>
          <option value={240}>4 分钟 · 标准局</option>
          <option value={300}>5 分钟 · 完整局</option>
        </select>
      </label>
      <p>
        2／4／6／8 人分成红蓝两队，最多 4v4。水泡会连爆、破坏木箱并困住对手；
        触碰被困队友可以救援，触碰被困对手可将其击破。
      </p>
    </div>
  );
}

export default defineGame<View, Options>({
  ...catalog,
  resources,
  asyncBoard: true,
  defaultOptions: { map: "community10", roundSeconds: 300 },
  Board,
  Cover,
  Options: OptionsForm,
  Rules: () => (
    <div className="bubble-rules">
      <p>
        2／4／6／8
        人实时对战，分成红、蓝两队。进入对局先选择人物，全部选好或选择时间结束后，
        经过 3 秒倒数开战。一方全员淘汰即获胜；时间到双方仍有人存活则和局。
      </p>
      <p>
        人物有不同的初始速度、水泡数量、威力和成长上限；红蓝队的基础配色不变。
        WASD／方向键移动，空格或 J
        放水泡。手机使用左侧摇杆沿格线四向移动，右侧按钮放水泡。
      </p>
      <p>
        水泡约 2.2
        秒后沿十字方向爆开，并能引发连爆。木箱和固定砖都会挡住爆炸并被破坏；
        可推动木箱需正面接触，前方有障碍、人物、水泡或道具时不能推动。
        树木、房屋等固定障碍不能穿过。放下水泡后可以离开，但不能返回穿过水泡。
      </p>
      <p>
        水柱命中会把人物困在水泡里；队友触碰可救援，对手触碰会击破淘汰。
        困泡倒计时结束仍未获救则淘汰，救援后有短暂保护。未消退的水柱仍会困住人物，
        被困时无法击破或救援。
      </p>
      <p>
        箱子会掉落速度、水泡容量、威力道具，也可能出现乌龟、猫头鹰、飞碟、章鱼、矿车等坐骑。
        坐骑各有移动特性，受到水柱攻击会先失去坐骑。捡道具的音效仅由捡到的人听见。
      </p>
      <p>
        经典场地收录可核验的 2002 客户端全部 112
        张地图，保留来源编号，不能与后续国服编号直接对应。 小区10
        按历史参考重建，另有三张自定义场地。少出生点地图按平台人数适配，不改变原始地形；
        方向通道、传送点和传送带按各地图规则通行。
      </p>
      <p>
        新对局没有缩圈，可选择 3、4 或 5
        分钟。电脑和托管使用相同规则；刷新或重连恢复同一局，
        不重播已发生的爆炸。本项目为围桌内的玩法还原，与官方运营服务无关联。
      </p>
    </div>
  ),
  PlayerSummary: ({ room, data, seat }) => {
    const player = data.players.find((p) => p.seat === seat);
    if (!player) return null;
    return (
      <div
        className={`bubble-player-summary team-${player.team}`}
        data-bubbleparty-seat={seat}
        data-bubbleparty-seq={player.seq}
        data-bubbleparty-team={player.team}
        data-bubbleparty-alive={player.alive ? "1" : "0"}
      >
        <strong>
          <i aria-hidden="true" />
          {teamNames[player.team]} ·{" "}
          {player.trappedUntil ? "困泡" : player.alive ? "存活" : "淘汰"}
          {room.you === seat && !room.spectating ? " · 你" : ""}
        </strong>
        {(data.rulesVersion ?? 0) >= 3 && (
          <span>
            {characterName(player.character)}
            {player.mount && mountName(player.mount)
              ? ` · 骑乘${mountName(player.mount)}`
              : ""}
          </span>
        )}
        <span>
          {player.alive
            ? `水泡 ${player.maxBombs} · 射程 ${player.range}${player.shield ? " · 护盾" : ""}`
            : "观战中"}
          {player.eliminations ? ` · 淘汰 ${player.eliminations}` : ""}
        </span>
      </div>
    );
  },
});
