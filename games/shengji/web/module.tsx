import { pokerResources } from "../../../web/src/poker/loading";
import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { rankName } from "../../../web/src/poker/ui";
import { Board, OptionsForm, type View, type Options } from "./Board";
import { Cover } from "./Cover";
export default defineGame<View, Options>({
  ...catalog,
  resources: pokerResources,
  defaultOptions: {
    decks: 2,
    rounds: 0,
    startLevel: 2,
    turnSeconds: 60,
    throwPenalty: 0,
    bottomMode: "cards",
  },
  Board,
  Options: OptionsForm,
  Cover,
  PlayerSummary: ({ data, seat }) => (
    <div className="poker-public-summary">
      <strong>
        队伍 {(seat % 2) + 1} · {rankName(data.players[seat].level)}
      </strong>
      <span>{data.players[seat].count} 张</span>
      {seat === data.dealer && <span>庄家</span>}
    </div>
  ),
  Rules: () => (
    <div className="poker-rules">
      <p>
        四人固定对家，默认两副 108 张，每人 25 张，庄家取 8 张底牌再扣 8
        张。单副 54 张，每人 12 张，底牌 6 张。5 为 5 分，10 和 K 各 10
        分；40／80 是抓分方上台门槛。
      </p>
      <p>
        线上预设发齐后集中亮主：单张级牌→同花级牌对子→同色双王无主，只能更强反主，不能自反、同强度反主或自保。无主仍以王和级牌为主。双副首副最终亮主者坐庄；无人亮主重发。后续无人亮主逐张翻底，遇级牌定主，否则最高普通底牌花色定主，同点先翻者优先。
      </p>
      <p>
        主牌从大到小：大王、小王、主花色级牌、其他花色级牌、主花色 A 至
        2（跳过级牌）。副级牌彼此同强，先出者大。对子必须同花同点，两张副级牌花色不同不能当对子。级牌移出自然序列后，相邻对子可组成拖拉机。
      </p>
      <p>
        须先跟足同门牌；有对应完整拖拉机必须跟，无完整结构时依次保留最长部分拖拉机和对子。不足一门时跟完该门再垫牌；彻底缺门时可用相同结构主牌毙牌。散主不能毙对子，散对不能毙拖拉机。
      </p>
      <p>
        同门多子组可甩牌。包括对家在内，若其他玩家能用同门对应结构压过其中的子组，系统会拒绝本次提交，手牌、回合和分数均不变；不会自动拆牌领出、收回牌或罚分。混合结构毙牌比较最大结构的最高点，同强先出优先。
      </p>
      <p>
        抓分方拿到末墩才抠底。单副固定底分 ×2；双副默认底分 ×末墩领出张数
        ×2。可选指数方式：单张 ×2、对子 ×4、两连对 ×8、三连对
        ×16，混合甩牌按最大结构。
      </p>
      <p>
        单副计分：0 分庄升 2；5～35 庄升 1；40～75 攻方上台不升级；80～95 攻升
        1；100 以上攻升 2。双副：0 分庄升 3；5～35 庄升 2；40～75 庄升
        1；80～115 攻方上台；其后每多 40 分攻升 1。罚分在抠底后合并，抓分最低按
        0 计。
      </p>
      <p>
        庄家守住时对家接庄；上台时原庄下家坐庄。升级不能跳过 A，必须以 A
        坐庄并守住才能赢整场。休闲局按副数比较级数，平级加打一副决胜。
      </p>
      <p>
        单副上海特殊规则：同点四花色可同时领出，其余人有该花色须每花色各跟一张；只有更高四同可赢，级牌四同最大。单副无分牌可明示申请重发，庄家须连同底牌都无分；重发后双方按各自级牌抢庄。未启用常主
        2 或勾 J。以上具体约定随对局保存，不宣称覆盖所有上海家庭玩法。
      </p>
    </div>
  ),
});
