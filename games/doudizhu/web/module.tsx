import { defineGame } from "../../../web/src/game-api/module";
import { Board, OptionsForm, roleName, type Options, type View } from "./Board";
import { Cover } from "./Cover";
export default defineGame<View, Options>({
  id: "doudizhu",
  name: "斗地主",
  description: "三人经典或四人上海，叫分争地主，组合出牌，与伙伴协作出完手牌。",
  category: "poker",
  minPlayers: 3,
  maxPlayers: 4,
  requiresFullTable: true,
  supportsBots: true,
  ownsSounds: true,
  defaultOptions: {
    mode: "classic3",
    rounds: 1,
    turnSeconds: 30,
    allowStraight: true,
    personalDoubling: false,
  },
  Board,
  Options: OptionsForm,
  Cover,
  PlayerSummary: ({ data, seat }) => (
    <div className="poker-public-summary">
      <strong>{roleName(data.players[seat].role)}</strong>
      <span>{data.players[seat].count} 张</span>
      <span>积分 {data.players[seat].score}</span>
    </div>
  ),
  Rules: () => (
    <div className="poker-rules">
      <p>
        三人经典使用一副 54 张牌，每人 17 张，地主取 3
        张公开底牌。四人上海使用两副 108 张牌，每人 25 张，地主取 8
        张底牌。地主独自对抗其余所有农民，任一农民先出完即农民共同获胜。
      </p>
      <p>
        叫 1、2、3 分，后叫须更高；叫 3
        分立即确定地主，全不叫重发。领出必须出牌，其余玩家可跟更大的同型同张数牌或不出，一圈不出后重新领出。
      </p>
      <p>
        三人允许三带一、三带对、单顺、连对、飞机带单／带对和四带两单／两对。四人不允许三带一、飞机单翅膀和四带二；允许同色王对，单顺按房间选项执行。连续主体不含
        2 和王。
      </p>
      <p>
        四人 4～8
        张同点炸弹按张数优先、再按点数比较。四个王组成「王炸」，压过所有炸弹，包括八张炸弹。三人王炸为大小王各一张。
      </p>
      <p>
        三人炸弹和王炸各翻倍；四人 4／5 张炸弹不翻倍，6
        张以上及王炸各翻倍。地主胜且农民都未出牌为春天；农民胜且地主只出过一手为反春，均再翻倍。个人加倍默认关闭，开启时农民分别决定，地主可再加倍。每位农民分别与地主结算，积分总和为
        0。
      </p>
      <p>
        单副按地主／农民阵营判胜负，个人加倍不改变同阵营胜负。多副按累计积分排名，最高分者获胜，可并列。地区差异以开局设置为准。手牌只对本人显示，超时由电脑接管，可通过平台收回托管。
      </p>
    </div>
  ),
});
