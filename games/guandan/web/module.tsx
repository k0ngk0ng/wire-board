import { pokerResources } from "../../../web/src/poker/loading";
import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { rankName } from "../../../web/src/poker/ui";
import { Board, OptionsForm, type View, type Options } from "./Board";
import { Cover } from "./Cover";
export default defineGame<View, Options>({
  ...catalog,
  resources: pokerResources,
  defaultOptions: { rounds: 0, startLevel: 2, turnSeconds: 30 },
  Board,
  Options: OptionsForm,
  Cover,
  PlayerSummary: ({ data, seat }) => (
    <div className="poker-public-summary">
      <strong>
        队伍 {(seat % 2) + 1} · {rankName(data.players[seat].level)}
      </strong>
      <span>{data.players[seat].count} 张</span>
      {data.players[seat].place > 0 && (
        <span>第 {data.players[seat].place} 位</span>
      )}
    </div>
  ),
  Rules: () => (
    <div className="poker-rules">
      <p>
        按江苏 2017 规则基线：四人两副 108 张牌，每人 27
        张，对家搭档。红心级牌为配牌，可代替普通牌，不能代王。单张级牌大于
        A、小于小王，配牌单出按级牌计算。
      </p>
      <p>
        支持单张、对子、三张、三带对、五张顺子、三连对、二连三。同花顺大于五张炸弹、小于六张炸弹；炸弹先比张数，最多可用配牌组成十张炸弹，四王王炸最大。顺子按自然顺序，A
        可作头或尾，不能跨 K、A、2 相连。
      </p>
      <p>
        配牌有多种解释时，选牌后点击「查看牌型」选择实际牌型，再出牌。提示仅列举部分推荐组合，手动选牌可使用任意合法组合。
      </p>
      <p>
        头游与同伴第二／第三／最后分别升 3／2／1 级；级数到 A 必须打一副
        A，同伴第二或第三才能过
        A。对家出完后，其他玩家均不出，由仍有牌的搭档接风。
      </p>
      <p>
        下游向上游进贡最大非配牌；双下时两人分别进贡，大贡给上游，小贡给二游；同点按座次顺时针交接。应贡方合持两个大王可抗贡并公开证明，抗贡由上游先出；否则给上游进贡者先出。
      </p>
      <p>
        还贡须为当前牌序不大于 10
        的非级牌，不能还刚收到的贡牌，且仅交接双方可见。线上补充：无满足条件的还牌时，还最小非配牌。默认不启用三次不过
        A 退级。
      </p>
      <p>
        休闲局按约定副数比较双方级数；平级加打一副决胜。所有规则和结束条件在开局前展示。
      </p>
    </div>
  ),
});
