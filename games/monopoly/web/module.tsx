import catalog from "./catalog";
import { defineGame } from "../../../web/src/game-api/module";
import { Cover } from "./Cover";
import { Board } from "./Board";
import type { Options, View } from "./types";
import { Art } from "./ui";
import "./style.css";

export default defineGame<View, Options>({
  ...catalog,
  defaultOptions: { auctionSeconds: 20, turnSeconds: 120, tradeSeconds: 90 },
  Board,
  Cover,
  PlayerSummary: ({ room, data, seat, assets }) => (
    <div className="mono-seat-summary">
      <small className="mono-cash-label">
        {!room.spectating && room.you === seat ? "你的现金" : "现金"}
      </small>
      <strong data-mono-cash={seat}>M {data.players[seat].cash}</strong>
      <span className="mono-seat-holdings">
        <Art
          base={assets}
          name={`token-${seat + 1}`}
          fallback={<span aria-hidden="true">♟</span>}
        />
        <small>
          {data.properties.filter((p) => p.owner === seat).length} 处地产
        </small>
      </span>
      <span className="sr-only">
        当前位置：{data.board[data.players[seat].position].name}
      </span>
    </div>
  ),
  Options: ({ value, onChange, disabled }) => (
    <div className="mono-options">
      <label>
        行动时限（秒）
        <input
          aria-label="行动时限（秒）"
          type="number"
          min="30"
          max="300"
          step="10"
          value={value.turnSeconds}
          disabled={disabled}
          onChange={(e) =>
            onChange({ ...value, turnSeconds: Number(e.target.value) })
          }
        />
      </label>
      <label>
        拍卖初始时限（秒）
        <input
          aria-label="拍卖初始时限（秒）"
          type="number"
          min="5"
          max="60"
          value={value.auctionSeconds}
          disabled={disabled}
          onChange={(e) =>
            onChange({ ...value, auctionSeconds: Number(e.target.value) })
          }
        />
      </label>
      <label>
        交易报价保留（秒）
        <input
          aria-label="交易报价保留（秒）"
          type="number"
          min="15"
          max="180"
          step="15"
          value={value.tradeSeconds}
          disabled={disabled}
          onChange={(e) =>
            onChange({ ...value, tradeSeconds: Number(e.target.value) })
          }
        />
      </label>
      <p>拍卖截止和交易报价过期不会开启托管；必须处理的行动超时会交给电脑。</p>
    </div>
  ),
  Rules: () => (
    <div className="mono-rules">
      <p>
        2～6 人，每人初始资金
        1500。掷两枚骰子决定先手，之后顺时针行动。最后未破产的玩家获胜。
      </p>
      <ol>
        <li>
          经过或停在起点领取 200。双骰相同可再次行动；连续第三次相同直接入狱。
        </li>
        <li>
          落在无主地产上，选择购买或公开拍卖。拍卖从 10 起价，每次至少加
          1；任何合法竞买者都可报价，无人出价则归银行保留。
        </li>
        <li>
          租金自动收付。拥有整个色组时，未建设街道的租金翻倍；抵押地产不收租。同组其余未抵押地产仍保留加成。
        </li>
        <li>
          集齐且未抵押的色组可均衡建造。每块最多 4
          座房屋，再升级为旅馆。银行只有 32 座房屋、12
          座旅馆，最后库存有多人竞购时拍卖。
        </li>
        <li>
          建筑按半价出售，拆除也须均衡。单独降级旅馆需要银行有 4
          座房屋，否则可整组出售。抵押或交易街道前，先卖掉同组全部建筑。
        </li>
        <li>
          可交易现金、地产和免狱卡，不允许借款或免租承诺。抵押地产受让人需立即付
          10% 利息，或支付本金加 10% 赎回；单笔利息向上取整。
        </li>
        <li>
          出狱可支付 50、使用免狱卡或尝试双骰相同。第三次尝试失败须支付 50
          后按原骰子移动；双骰出狱没有额外回合。在狱中仍可收租和管理资产。
        </li>
        <li>
          现金不足先筹资；仍无法偿还时破产。欠玩家与欠银行按不同规则移交资产。出局后可以离桌观战，原对局保留名次。
        </li>
      </ol>
      <p>
        免费停车没有奖金。卡牌的逐人收付款按抽牌者之后的座次顺序处理。仅剩一人时，转移破产者资产后直接结算，不再开启赢家的受让收费或拍卖。首版使用英国版地名与
        2021 年后公共基金卡。
      </p>
    </div>
  ),
});
