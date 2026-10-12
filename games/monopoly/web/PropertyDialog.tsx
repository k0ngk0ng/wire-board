import { Modal } from "../../../web/src/ui/board-host";
import type { GameRoom } from "../../../web/src/game-api/module";
import type { Send, View } from "./types";
import { GameButton, playerName } from "./ui";

export function PropertyDialog({
  id,
  data,
  room,
  busy,
  send,
  onClose,
}: {
  id: number;
  data: View;
  room: GameRoom;
  busy: boolean;
  send: Send;
  onClose: () => void;
}) {
  const space = data.board[id],
    property = data.properties[id],
    controls = data.controls.properties[id];
  const owned = property.owner === room.you && room.you >= 0;
  const execute = (type: string) => void send(type, { property: id });
  const notes: Record<string, string> = {
    go: "每次向前经过或停在起点，领取 200。直接入狱不领取奖励。",
    visiting: "停在这里仅为探监，不会入狱。",
    parking: "免费停车不收取费用，也不发放奖金。",
    jail: "直接进入监狱，本回合结束。",
    chance: "抽取并执行一张机会卡。",
    chest: "抽取并执行一张公共基金卡。",
    tax: `向银行支付 ${space.tax}。`,
  };
  return (
    <Modal title={space.name} onClose={onClose} className="mono-modal">
      <div data-mono-property-dialog className="mono-deed">
        <div
          className="mono-deed-heading"
          style={{ borderTopColor: space.color }}
        >
          <small>{space.price ? "所有权证 · TITLE DEED" : "棋盘地点"}</small>
          <h3>{space.name}</h3>
        </div>
        {space.price > 0 ? (
          <>
            <div className="mono-deed-status">
              <strong>持有者：{playerName(room, property.owner)}</strong>
              <span>
                {property.mortgaged
                  ? "已抵押 · 不收租"
                  : property.buildings === 5
                    ? "已建旅馆"
                    : property.buildings > 0
                      ? `${property.buildings} 座房屋`
                      : "未建设"}
              </span>
            </div>
            <table>
              <tbody>
                <tr>
                  <th>购买价格</th>
                  <td>{space.price}</td>
                </tr>
                {space.kind === "street" && (
                  <>
                    <tr>
                      <th>空地租金</th>
                      <td>{space.rents[0]}</td>
                    </tr>
                    <tr>
                      <th>集齐色组的空地租金</th>
                      <td>{space.rents[0] * 2}</td>
                    </tr>
                    {[1, 2, 3, 4, 5].map((n) => (
                      <tr
                        key={n}
                        className={property.buildings === n ? "current" : ""}
                      >
                        <th>{n === 5 ? "1 座旅馆" : `${n} 座房屋`}</th>
                        <td>{space.rents[n]}</td>
                      </tr>
                    ))}
                  </>
                )}
                {space.kind === "station" &&
                  [25, 50, 100, 200].map((rent, i) => (
                    <tr key={i}>
                      <th>持有 {i + 1} 座车站</th>
                      <td>{rent}</td>
                    </tr>
                  ))}
                {space.kind === "utility" && (
                  <>
                    <tr>
                      <th>持有 1 家公用事业</th>
                      <td>新骰子点数 × 4</td>
                    </tr>
                    <tr>
                      <th>持有 2 家公用事业</th>
                      <td>新骰子点数 × 10</td>
                    </tr>
                  </>
                )}
                <tr>
                  <th>抵押收入</th>
                  <td>{space.price / 2}</td>
                </tr>
                {space.kind === "street" && (
                  <tr>
                    <th>每座建筑价格</th>
                    <td>{space.buildingCost}</td>
                  </tr>
                )}
              </tbody>
            </table>
            {owned && controls && (
              <div className="mono-property-actions">
                {space.kind === "street" && (
                  <GameButton
                    disabled={busy || !controls.build}
                    onClick={() => execute("build")}
                  >
                    {controls.buildingAuction
                      ? "竞购最后一座建筑"
                      : `${property.buildings === 4 ? "升级旅馆" : "建房"} · ${space.buildingCost}`}
                  </GameButton>
                )}
                {property.buildings > 0 && (
                  <>
                    <GameButton
                      disabled={busy || !controls.sell}
                      onClick={() => execute("sell_building")}
                    >
                      出售一座 · {controls.sellValue}
                    </GameButton>
                    <GameButton
                      disabled={busy || !controls.sellGroup}
                      onClick={() => execute("sell_group")}
                    >
                      出售整组建筑 · {controls.groupSellValue}
                    </GameButton>
                  </>
                )}
                {property.mortgaged ? (
                  <GameButton
                    disabled={busy || !controls.redeem}
                    onClick={() => execute("unmortgage")}
                  >
                    赎回 · 支付 {controls.redeemCost}
                  </GameButton>
                ) : (
                  <GameButton
                    disabled={busy || !controls.mortgage}
                    onClick={() => execute("mortgage")}
                  >
                    抵押 · 收入 {controls.mortgageValue}
                  </GameButton>
                )}
                <p className="mono-help">
                  须集齐未抵押色组并均衡建设。抵押或交易街道前，先出售同组全部建筑。
                </p>
              </div>
            )}
          </>
        ) : (
          <p>{notes[space.kind]}</p>
        )}
      </div>
    </Modal>
  );
}
