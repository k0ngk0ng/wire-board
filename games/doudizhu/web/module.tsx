import { pokerResources } from "../../../web/src/poker/loading";
import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { Board, OptionsForm, roleName, type Options, type View } from "./Board";
import { Cover } from "./Cover";
export default defineGame<View, Options>({
  ...catalog,
  resources: pokerResources,
  defaultOptions: {
    mode: "shanghai4",
    rounds: 1,
    turnSeconds: 30,
    allowStraight: true,
    personalDoubling: false,
  },
  Board,
  Options: OptionsForm,
  Cover,
  PlayerSummary: ({ data: d, seat }) => {
    const p = d.players[seat],
      huanle = d.ruleset === "doudizhu-shanghai-huanle-v2";
    return (
      <div className="poker-public-summary">
        <strong>{roleName(p.role)}</strong>
        <span>{p.count} 张</span>
        <span>积分 {p.score}</span>
        {huanle && d.phase !== "bid" && (
          <span>
            炸弹{" "}
            {p.bombLimit < 0
              ? "不限"
              : `剩 ${Math.max(0, p.bombLimit - p.bombUsed)} 次`}
          </span>
        )}
      </div>
    );
  },
  Rules: () => (
    <div className="poker-rules">
      <p>
        默认且仅提供「上海斗地主」，采用弈棋耍大牌官网的四人欢乐三打一规则。两副
        108 张，每人 25 张，地主取 8
        张底牌；底牌仅地主可见。地主对三位农民，任一农民先出完则农民共同赢牌。
      </p>
      <p>
        标记牌所在玩家先叫档，可选一档、二档、三档或不叫，后叫须更高。叫三档立即确定地主；四家不叫重发，下一副为荒番。标记牌为王时为明王；荒番、明王各
        ×2。
      </p>
      <p>
        单张大小：大王、小王、2、A、K、Q、J、10 至
        3。允许单牌、对子、三张、三带二、单顺、双顺、三顺、飞机带对子。顺子按自然顺序，2
        小于 3，A 可在两端，不能跨 K、A、2
        连成环；飞机带的对子也必须连续。不允许三带一、飞机带单、四带二。
      </p>
      <p>
        4～8
        张同点牌为炸弹，先比张数再比点数。四王称「王炸」，最大，压过八张炸弹。地主和未轮到叫档的农民不限炸弹次数；叫一档或选择不叫的农民最多炸一次，叫二档的农民最多炸两次，王炸也计一次。不额外套用春天或炸弹翻倍。
      </p>
      <p>
        首位直接叫三档为头撩，地主赢牌时结算
        ×4，输时不加此倍数。仅头撩可选择摊打；摊打后地主手牌公开，输赢 ×2。
      </p>
      <p>
        地主拥有四王或七张／八张同点炸弹可选报到，至少两组可双报到。报到不打直接结算；报到后继续打则将报到奖励并入结算。官网简述与公式有冲突，本平台采用下方详细公式。
      </p>
      <p>
        底注固定为 1，D 为档数，R 为报到组数（单报 1、双报 2），M
        为荒番、明王、摊打的倍率乘积。每位农民与地主结算：普通地主赢为
        D×M，地主输为 −D×M；报到不打为 D×R；报到继续打且地主赢为
        D×(M+R)，地主输为 D×(R−M)。头撩只在地主赢时将相应结果再
        ×4。地主积分为三位农民转移之和，四人总和始终为 0。
      </p>
      <p>
        报到奖励可能抵扣或超过输牌扣分，赢牌阵营与积分金额分别显示。单副按阵营判胜负，多副按累计积分排名。更新前已开始的对局保留原存档规则；新房间不再开放三人模式。
      </p>
      <p>
        <a
          href="https://www.17dp.com/down/gamelist/id/201"
          target="_blank"
          rel="noreferrer"
        >
          来源：弈棋耍大牌官网《上海斗地主》
        </a>
        。本平台不使用运营方的金豆、收费或逃跑扣币制度。
      </p>
    </div>
  ),
});
