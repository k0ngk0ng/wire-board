import { useEffect, useState } from "react";
import type { Act, CatanState, Room } from "./types";
import "./catan-new-world.css";

const keys = ["wood", "brick", "wool", "grain", "ore"];
const names = ["木材", "砖块", "羊毛", "粮食", "矿石"];

export function CatanNewWorldRules({ game }: { game: CatanState }) {
  return (
    <>
      <p>
        航海家 · 新世界，{game.players.length}
        人。地图在开局前共同确认，在自己的行动阶段达到12分获胜。
      </p>
      <ol>
        <li>
          从先手开始轮流放置当前抽出的港口，放在陆地与海洋或外框之间的边上，与其他港口至少隔开一条边。放完全部港口后，由先手开始起始建设。每次放港口限时120秒，超时自动安放。
        </li>
        <li>
          顺序放第一组村庄与道路或船只，再逆序放第二组。只从第二座村庄领取相邻资源；金矿资源可自选。两座起始村庄所在的一座或两座岛是自己的家乡岛。
        </li>
        <li>
          每位玩家首次在自己的非家乡岛建村，额外获得1分，每岛一次。其他玩家已经定居不影响自己的奖励。
        </li>
        <li>
          回合开始掷两颗骰子，按数字生产资源，村庄一张、城市两张。掷出7时资源超过七张者同时弃一半（向下取整），随后移动强盗或海盗并偷取一位相邻对手的资源。
        </li>
        <li>
          强盗和海盗开局都在外框。强盗阻止地块生产，海盗封锁所在海格旁的船只。金矿产出选择一种资源，每份分别选择；红色6/8不相邻，也不放在金矿上。
        </li>
        <li>
          船只费用为一木一羊，沿海洋或海岸建造；道路与船只只在己方建筑处接续。每个行动阶段可移动一艘开放航线末端的旧船，本阶段刚造的船不能移动。村庄与所有建筑至少相隔两条边，城市升级已有村庄。
        </li>
        <li>
          掷骰后可交易、建设及购买发展卡。银行4:1、通用港口3:1、专用港口2:1；自由交易需双方同意。每行动阶段最多使用一张发展卡，刚买的本阶段不能使用；胜利点自动计入私人总分。
        </li>
        <li>
          最长路线至少5段、最大骑士军队至少3名，各奖励2分。并列由原持有者保留，原持有者不在并列中则暂时无人持有。
        </li>
      </ol>
      {game.options?.fiveSix && (
        <p>
          五至六人采用配对行动：①号玩家正常回合后，左侧第三位②号玩家不掷骰、不自由交易，可以建设、银行/港口交易和使用发展卡，也可获胜；两人完成后标记移到下一位。
        </p>
      )}
      {game.options?.helpers && (
        <p>
          本局启用Helpers。完成自己的第二组起始建设后领取助手，使用后按提示翻面或更换。
        </p>
      )}
      <p>
        正常回合120秒；起始建设、弃牌及资源选择超时自动处理。资源和发展卡仅本人可见，结束后公开。
      </p>
    </>
  );
}

export function CatanNewWorldPortChoice({
  room,
  assets,
  busy,
  act,
  edge,
  clear,
}: {
  room: Room;
  assets: string;
  busy: boolean;
  act: Act;
  edge: number | null;
  clear: () => void;
}) {
  const game = room.game!,
    g = game.catan!,
    world = g.seafarers?.newWorld;
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => setCollapsed(false), [room.id, world?.index, edge]);
  if (
    !world ||
    world.current === undefined ||
    game.phase !== "catan_world_ports" ||
    game.finished ||
    room.status !== "playing"
  )
    return null;
  const mine =
    !room.spectating &&
    room.you >= 0 &&
    room.you === game.turn &&
    !g.players[room.you]?.eliminated;
  const resource = world.current,
    name = resource < 0 ? "通用港口 3:1" : `${names[resource]}港口 2:1`;
  return (
    <section
      className="catan-gold-choice catan-world-choice"
      aria-label="新世界港口布局"
    >
      <header>
        <strong>
          {mine ? "放置当前港口" : `${room.seats[game.turn].name} 正在放置港口`}{" "}
          · {world.index + 1}/{world.total}
        </strong>
        <button
          aria-expanded={!collapsed}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "展开" : "收起"}
        </button>
      </header>
      {!collapsed && (
        <div className="catan-gold-body">
          <div className="catan-world-current">
            {assets && (
              <img
                src={`${assets}/catan/port-${resource < 0 ? "any" : keys[resource]}-v1.webp`}
                alt=""
              />
            )}
            <div>
              <strong>{name}</strong>
              <p>剩余{world.remaining}个（含当前港口）</p>
            </div>
          </div>
          <p>
            港口放在陆地与海洋或外框之间，彼此至少隔开一条边。每人轮流放一个，全部放完后开始建村。
          </p>
          <p>限时120秒，超时自动安放。可收起面板查看地图。</p>
          {mine && (
            <>
              <p>
                {edge === null
                  ? "点击地图上亮起的海岸位置，再确认。"
                  : `已选择海岸 #${edge + 1}。`}
              </p>
              <div className="cloth-confirm-actions">
                {edge !== null && (
                  <button disabled={busy} onClick={clear}>
                    重选位置
                  </button>
                )}
                <button
                  className="primary"
                  disabled={
                    busy || edge === null || !g.legal.ports?.includes(edge)
                  }
                  onClick={async () => {
                    await act({ type: "catan_world_port", edge });
                    clear();
                  }}
                >
                  确认安放
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </section>
  );
}
