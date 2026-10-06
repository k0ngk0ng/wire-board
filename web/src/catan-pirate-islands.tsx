import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { CatanState, Room } from "./types";
import {
  catanColorIndex,
  catanPieceColors,
  catanSeatColor,
} from "./catan-player-colors";
import "./catan-pirate-islands.css";

export function catanFortressReady(game: CatanState, player: number) {
  const f = game.seafarers?.pirateIslands?.fortresses[player];
  if (!f || f.strength <= 0 || !f.route.length) return false;
  const last = game.edges[f.route[f.route.length - 1]];
  return last?.a === f.vertex || last?.b === f.vertex;
}

export function CatanEndAction({
  room,
  busy,
  end,
}: {
  room: Room;
  busy: boolean;
  end: () => Promise<void>;
}) {
  const [confirm, setConfirm] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const dialog = useRef<HTMLElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const game = room.game!.catan!,
    ready = catanFortressReady(game, room.you);
  const fort = game.seafarers?.pirateIslands?.fortresses[room.you];
  const warships =
    fort?.route.filter((id) => game.edges[id].warship).length ?? 0;
  useEffect(() => {
    if (!ready) setConfirm(false);
    else if (confirm) dialog.current?.focus({ preventScroll: true });
  }, [ready, confirm]);
  const cancel = () => {
    setConfirm(false);
    trigger.current?.focus({ preventScroll: true });
  };
  return (
    <>
      <button
        ref={trigger}
        className="catan-end"
        disabled={busy}
        onClick={() => (ready ? setConfirm(true) : void end())}
      >
        {ready
          ? "攻打要塞并结束行动 →"
          : game.transport
            ? "结束建设，开始运输 →"
            : "结束回合 →"}
      </button>
      {confirm &&
        ready &&
        createPortal(
          <section
            ref={dialog}
            tabIndex={-1}
            className="catan-gold-choice catan-fortress-confirm"
            role="dialog"
            aria-label="确认攻打要塞"
            onKeyDown={(event) => {
              if (event.key === "Escape" && !busy) {
                event.stopPropagation();
                cancel();
              }
            }}
          >
            <header>
              <strong>攻打你的海盗要塞</strong>
              <button
                aria-expanded={!collapsed}
                onClick={() => setCollapsed(!collapsed)}
              >
                {collapsed ? "展开" : "收起"}
              </button>
            </header>
            {!collapsed && (
              <div className="catan-gold-body">
                <p>
                  远征航线有 <b>{warships} 艘战舰</b>，要塞剩余{" "}
                  <b>{fort!.strength} 层防御</b>
                  。结束行动时掷一枚战斗骰，点数为海盗力量。
                </p>
                <ul>
                  <li>战舰数大于点数：移除一层防御。</li>
                  <li>战舰数等于点数：退回末端一艘船。</li>
                  <li>战舰数小于点数：退回末端两艘船。</li>
                </ul>
                <p>战斗结束后交接下一位玩家。你也可以先继续建设或交易。</p>
                <button
                  className="primary wide"
                  disabled={busy}
                  onClick={() => void end().then(() => setConfirm(false))}
                >
                  掷战斗骰并结束行动
                </button>
                <button className="wide" disabled={busy} onClick={cancel}>
                  返回地图，继续行动
                </button>
              </div>
            )}
          </section>,
          document.body,
        )}
    </>
  );
}

export function CatanFleetPath({ game }: { game: CatanState }) {
  const pirates = game.seafarers?.pirateIslands;
  if (!pirates || game.seafarers!.pirate < 0) return null;
  return (
    <g className="catan-fleet-path" pointerEvents="none">
      <title>海盗舰队沿箭头巡航；每次前进两枚骰子中较小的点数</title>
      {pirates.fleetPath.map((id, i) => {
        const a = game.tiles[id],
          b = game.tiles[pirates.fleetPath[(i + 1) % pirates.fleetPath.length]];
        const angle = (Math.atan2(b.y - a.y, b.x - a.x) * 180) / Math.PI;
        return (
          <g
            key={id}
            transform={`translate(${(a.x + b.x) / 2},${(a.y + b.y) / 2}) rotate(${angle})`}
          >
            <path d="M-9 0H7M1-5L8 0L1 5" />
          </g>
        );
      })}
      {pirates.safeTile >= 0 && (
        <g
          transform={`translate(${game.tiles[pirates.safeTile].x},${game.tiles[pirates.safeTile].y - 20})`}
        >
          <title>安全海域：海盗舰队停在这里时不发动攻击</title>
          <circle r="11" fill="#ffedac" stroke="#483824" />
          <text
            textAnchor="middle"
            y="5"
            fill="#483824"
            fontSize="17"
            fontWeight="800"
          >
            !
          </text>
        </g>
      )}
    </g>
  );
}

export function CatanPirateMarkers({
  room,
  assets,
}: {
  room: Room;
  assets: string;
}) {
  const game = room.game!.catan!,
    pirates = game.seafarers?.pirateIslands;
  if (!pirates) return null;
  const scale = Math.max(0.64, (game.hexSize || 62) / 62);
  return (
    <g className="catan-pirate-markers" pointerEvents="none">
      {pirates.fortresses.map((fort, seat) => {
        const beach = game.vertices[fort.beachhead],
          v = game.vertices[fort.vertex];
        const name = room.seats[seat].name,
          color = catanSeatColor(game, seat);
        return (
          <g key={seat}>
            {beach.level === 0 && (
              <g transform={`translate(${beach.x},${beach.y}) scale(${scale})`}>
                <title>{name}的登陆点；船线抵达后可支付资源建村庄</title>
                <circle r="9" fill={color} stroke="#54442b" strokeWidth="1.5" />
                <circle r="12" fill="none" stroke={color} strokeWidth="2" />
              </g>
            )}
            {fort.strength > 0 && (
              <g transform={`translate(${v.x},${v.y}) scale(${scale})`}>
                <title>
                  {name}的要塞；剩余{fort.strength}
                  层防御；收复前不产资源、不计村庄分
                </title>
                {Array.from({ length: fort.strength }, (_, i) =>
                  assets ? (
                    <image
                      key={i}
                      href={`${assets}/catan/seafarers/pirate-lair-v1.webp`}
                      x="-16"
                      y={-9 - i * 4}
                      width="32"
                      height="32"
                    />
                  ) : (
                    <ellipse
                      key={i}
                      cy={8 - i * 4}
                      rx="16"
                      ry="9"
                      fill="#736943"
                      stroke="#dac18a"
                    />
                  ),
                )}
                {assets ? (
                  <image
                    href={`${assets}/catan/settlement-${catanPieceColors[catanColorIndex(game, seat)]}-v1.webp`}
                    x="-14"
                    y="-24"
                    width="28"
                    height="32"
                  />
                ) : (
                  <path
                    d="M-10 0V-13L0-22L10-13V0Z"
                    fill={color}
                    stroke="#523829"
                  />
                )}
                <circle cx="15" cy="12" r="9" fill="#fff3d7" stroke="#664022" />
                <text
                  x="15"
                  y="16"
                  textAnchor="middle"
                  fontSize="13"
                  fontWeight="800"
                  fill="#432d20"
                >
                  {fort.strength}
                </text>
              </g>
            )}
          </g>
        );
      })}
    </g>
  );
}

export function CatanPirateProgress({ room }: { room: Room }) {
  const g = room.game!.catan!,
    pirates = g.seafarers?.pirateIslands;
  if (!pirates) return null;
  const battle = pirates.battle;
  return (
    <section className="catan-pirate-progress" aria-label="海盗群岛进度">
      <details>
        <summary>远征与要塞 · 查看进度与规则</summary>
        <p>
          收复自己的要塞，并在自己的行动阶段达到 10
          分获胜。本剧本没有最长路线和最大骑士军队奖励。
        </p>
        {pirates.fortresses.map((fort, i) => {
          const defense = g.edges.filter(
            (e) => e.owner === i && e.ship && e.warship,
          ).length;
          const attack = fort.route.filter((id) => g.edges[id].warship).length;
          return (
            <div className="catan-pirate-progress-seat" key={i}>
              <strong>
                <i style={{ background: catanSeatColor(g, i) }} />
                {room.seats[i].name}
              </strong>
              <span>
                战舰 {defense} · 远征 {attack}/{fort.route.length}
              </span>
              <span>
                {fort.strength ? `要塞 ${fort.strength}/3 层` : "✓ 已收复"}
              </span>
            </div>
          );
        })}
        <small>战舰总数用于防守；攻打要塞只计算远征航线上的战舰。</small>
      </details>
      {battle && (
        <div key={battle.id} className="catan-pirate-battle" role="status">
          <strong>最近战斗 · {room.seats[battle.player].name}</strong>
          <div>
            <span
              className="catan-battle-die"
              aria-label={`海盗战斗骰 ${battle.die}`}
            >
              {battle.die}
            </span>
            <span>
              {battle.warships} 艘战舰 对 海盗 {battle.die}
            </span>
          </div>
          <p>
            {battle.warships > battle.die
              ? battle.remaining === 0
                ? "胜利，已收复要塞！"
                : `胜利，移除一层防御，剩余 ${battle.remaining} 层。`
              : `${battle.warships === battle.die ? "平局" : "战败"}，退回末端 ${battle.removed.length} 艘船。`}
          </p>
        </div>
      )}
    </section>
  );
}
