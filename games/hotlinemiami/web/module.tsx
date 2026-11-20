import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import catalog from "./catalog";
import { Options, Preparation } from "./Preparation";
import { characterNames, playerColors, weaponNames, type GameOptions, type View } from "./types";

export default defineGame<View, GameOptions>({
  ...catalog,
  assetLoading: "board",
  asyncBoard: true,
  preparationOwnsOptions: true,
  fullscreenToolbarOverlay: true,
  playerLayout: { density: "compact" },
  defaultOptions: {
    map: "no-talk",
    roundSeconds: 180,
    characters: ["jacket", "biker"],
    weapons: ["pistol", "pistol"],
  },
  Board,
  Options,
  Preparation,
  isInputRelease: action => action.type === "input" && action.moveX === 0 && action.moveY === 0 && action.fire === false && action.throw === false && action.pickup === false && action.finish === false && action.interact === false,
  Rules: () => <div className="hm-rules">
    <p>2 至 4 人自由混战。房主选地图和比赛时长，每人选择自己的造型与起始武器，准备后开始。击杀加 1 分，自杀减 1 分；时间到得分最高者获胜，最高分并列则和局。造型属性相同。</p>
    <p>WASD 移动，鼠标瞄准，左键攻击，右键投掷，E 拾取武器，F 开关门，空格处决倒地对手。按住攻击可持续开火；枪械弹药有限，打空后可投掷或换枪。近战不消耗弹药。</p>
    <p>手机使用左摇杆移动，右摇杆瞄准，向外推动右摇杆攻击，松开停止。拾取、投掷、处决和开门使用中间按钮。手柄左摇杆移动，右摇杆瞄准，右扳机攻击；键位可在平台控制器设置中修改。</p>
    <p>有效枪击与近战可致命，普通武器投掷通常使对手倒地；靠近倒地对手可以处决，处决时仍会受到攻击。死亡约 2 秒后复活，出生保护持续约 0.8 秒，攻击或投掷立即解除。</p>
    <p>地图由原版关卡区域改编，出生点和武器点为混战调整。支持电脑与主动托管；断线不会自动接管，重连继续原对局。含电脑或托管的比赛不计竞技积分。</p>
  </div>,
  PlayerSummary: ({ room, data, seat }) => {
    const player = data.players.find(candidate => candidate.seat === seat);
    if (!player) return null;
    const melee = ["knife", "bat", "katana", ""].includes(player.weapon);
    return <div className="hm-player-summary" data-hotlinemiami-summary={seat} style={{ borderColor: playerColors[seat % 4] }}>
      <strong>{player.alive ? player.downUntil > data.frame ? "倒地" : "存活" : "等待复活"}{seat === room.you ? " · 你" : ""}</strong>
      <span>{characterNames[player.character]} · {player.score} 分</span>
      <span>{weaponNames[player.weapon] ?? player.weapon}{melee ? "" : ` · ${player.ammo} 发`}</span>
    </div>;
  },
  ResultDetails: ({ room, data }) => <div className="hm-results">
    {[...data.players].sort((a, b) => b.score - a.score || b.kills - a.kills || a.seat - b.seat).map(player => <div key={player.seat}>
      <strong>{room.seats[player.seat]?.name ?? "玩家"}</strong><span>{player.score} 分</span><span>击杀 {player.kills} · 死亡 {player.deaths}</span>
    </div>)}
  </div>,
});
