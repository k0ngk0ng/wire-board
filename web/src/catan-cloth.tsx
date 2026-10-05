import { useEffect, useState } from "react";
import type { Act, Room } from "./types";
import "./catan-cloth.css";

export function ClothPicture({ assets }: { assets: string }) {
  return assets ? (
    <img src={`${assets}/catan/seafarers/cloth-v1.webp`} alt="" />
  ) : (
    <span aria-hidden="true">▱</span>
  );
}
export function CatanClothVillages({
  room,
  assets,
  inspect,
}: {
  room: Room;
  assets: string;
  inspect: (index: number) => void;
}) {
  const g = room.game!.catan!,
    c = g.seafarers?.cloth;
  if (!c) return null;
  const scale = Math.max(0.64, (g.hexSize || 62) / 62);
  return (
    <g className="catan-cloth-villages">
      {c.villages.map((v, i) => {
        const p = g.vertices[v.vertex];
        const mine = !room.spectating && (v.traders || []).includes(room.you);
        const label = `布匹村落 ${i + 1}，数字 ${v.number}，剩余 ${v.stock} 枚，${mine ? "已与你建立贸易" : "查看贸易关系"}`;
        return (
          <g
            key={v.vertex}
            role="button"
            tabIndex={0}
            aria-label={label}
            transform={`translate(${p.x},${p.y}) scale(${scale})`}
            onClick={() => inspect(i)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                inspect(i);
              }
            }}
            className={`catan-cloth-village ${mine ? "connected" : ""} ${v.stock === 0 ? "empty" : ""}`}
          >
            <circle className="cloth-village-hit" r="23" fill="transparent" />
            <circle className="cloth-village-disc" r="17" />
            <text
              className={`cloth-village-number ${v.number === 6 || v.number === 8 ? "red" : ""}`}
              textAnchor="middle"
              dominantBaseline="central"
            >
              {v.number}
            </text>
            <g transform="translate(19 1)">
              <rect
                className="cloth-village-stock"
                x="0"
                y="-12"
                width="43"
                height="24"
                rx="5"
              />
              {assets && (
                <image
                  href={`${assets}/catan/seafarers/cloth-v1.webp`}
                  x="2"
                  y="-9"
                  width="24"
                  height="18"
                />
              )}
              <text
                x="34"
                textAnchor="middle"
                dominantBaseline="central"
                className="cloth-village-count"
              >
                {v.stock}
              </text>
            </g>
            <title>{label}</title>
          </g>
        );
      })}
    </g>
  );
}
export function CatanClothStock({
  room,
  assets,
}: {
  room: Room;
  assets: string;
}) {
  const c = room.game?.catan?.seafarers?.cloth;
  if (!c) return null;
  const empty = c.villages.filter((v) => v.stock === 0).length;
  const mine = !room.spectating && room.you >= 0;
  const held = mine ? c.held[room.you] : 0;
  return (
    <section className="catan-cloth-stock" aria-label="布匹库存">
      <h3>
        <ClothPicture assets={assets} />
        卡坦布匹
      </h3>
      <div className="catan-cloth-counts">
        <span>
          公共库存 <b>{c.stock}</b>
        </span>
        <span>
          耗尽村落{" "}
          <b>
            {empty} / {c.emptyLimit}
          </b>
        </span>
        {mine && (
          <span>
            你的布匹 <b>{held}</b> 枚 · <b>{Math.floor(held / 2)}</b> 分
          </span>
        )}
      </div>
      <p>每两枚布匹得1分。点击村落查看贸易关系。没有最长路线奖励。</p>
      {room.game?.catan?.fishing && (
        <p>
          两座大岛各有三处渔场，不使用湖泊。仅第三座起始村庄领取资源和鱼。
        </p>
      )}
      {room.game?.catan?.citiesKnights && (
        <p>布匹是计分筹码，与城市建设使用的布料商品牌不同。</p>
      )}
      {empty >= c.emptyLimit ? (
        <p className="cloth-ending">本回合结束时比较总分，同分比较布匹数。</p>
      ) : (
        <p>回合结束时，{c.emptyLimit}座村落耗尽也会结算比赛。</p>
      )}
    </section>
  );
}
export function CatanClothChoice({
  room,
  assets,
  busy,
  act,
  selectedTile,
  clearTile,
  village,
  closeVillage,
  colors,
}: {
  room: Room;
  assets: string;
  busy: boolean;
  act: Act;
  selectedTile: number | null;
  clearTile: () => void;
  village: number | null;
  closeVillage: () => void;
  colors: string[];
}) {
  const game = room.game!,
    g = game.catan!,
    c = g.seafarers?.cloth;
  const phase = game.phase;
  const start = phase === "catan_cloth_start",
    steal = phase === "catan_cloth_steal";
  const active =
    (start || steal) && room.status === "playing" && !game.finished;
  const mine =
    active &&
    !room.spectating &&
    room.you === game.turn &&
    !g.players[room.you]?.eliminated;
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => setCollapsed(false), [room.id, phase, selectedTile, village]);
  if (!c || (!active && village === null)) return null;
  const v = village === null ? undefined : c.villages[village];
  return (
    <section
      className="catan-gold-choice catan-cloth-choice"
      aria-label={
        v ? "村落贸易详情" : start ? "选择初始强盗位置" : "海盗偷取选择"
      }
    >
      <header>
        <strong>
          {v
            ? `村落 #${village! + 1} · 数字 ${v.number}`
            : active
              ? mine
                ? start
                  ? "选择初始强盗位置"
                  : "海盗：选择偷取物品"
                : `${room.seats[game.turn].name} 正在${start ? "选择强盗起点" : "选择偷取物品"}`
              : "村落贸易详情"}
        </strong>
        <button
          aria-expanded={!collapsed}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "展开" : "收起"}
        </button>
        {v && (
          <button
            onClick={closeVillage}
            aria-label={active ? "返回当前选择" : "关闭村落详情"}
          >
            {active ? "返回选择" : "关闭"}
          </button>
        )}
      </header>
      {!collapsed && (
        <div className="catan-gold-body">
          {active && <p>限时120秒，超时自动选择。可收起面板查看地图。</p>}
          {start && !v && (
            <>
              <p>
                由先手选择大岛上一块12号地块作为强盗起点。
                {g.citiesKnights
                  ? "首次蛮族进攻后强盗才入场。之后依次放置村庄、城市、村庄及配套路线，仅第三座领取普通起始资源。"
                  : g.fishing
                    ? "之后每人放置三组起始村庄与路线，只从第三座村庄领取资源和鱼。"
                    : "之后每人放置三组起始村庄与路线，只从第三座村庄领取资源。"}
              </p>
              {mine && (
                <>
                  <p>
                    {selectedTile === null
                      ? "点击地图上亮起的12号地块。"
                      : `已选择地块 #${selectedTile + 1}。`}
                  </p>
                  <div className="cloth-confirm-actions">
                    {selectedTile !== null && (
                      <button disabled={busy} onClick={clearTile}>
                        重选位置
                      </button>
                    )}
                    <button
                      className="primary"
                      disabled={
                        busy ||
                        selectedTile === null ||
                        !g.legal.robber?.includes(selectedTile)
                      }
                      onClick={async () => {
                        await act({
                          type: "catan_cloth_start",
                          tile: selectedTile,
                        });
                        clearTile();
                      }}
                    >
                      确认起点
                    </button>
                  </div>
                </>
              )}
            </>
          )}
          {steal && mine && !v && (
            <div className="cloth-theft-targets">
              {g.victims.map((i) => (
                <div className="cloth-theft-target" key={i}>
                  <strong>{room.seats[i].name}</strong>
                  <div>
                    <button
                      disabled={busy || g.players[i].resourceCount === 0}
                      onClick={() =>
                        void act({
                          type: "catan_cloth_steal",
                          target: i,
                          choice: "resource",
                        })
                      }
                    >
                      <span>偷1张资源</span>
                      <small>持有{g.players[i].resourceCount}张</small>
                    </button>
                    <button
                      disabled={busy || c.held[i] === 0}
                      onClick={() =>
                        void act({
                          type: "catan_cloth_steal",
                          target: i,
                          choice: "cloth",
                        })
                      }
                    >
                      <span>
                        <ClothPicture assets={assets} /> 偷1枚布匹
                      </span>
                      <small>持有{c.held[i]}枚</small>
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
          {v && (
            <>
              <p className="cloth-detail-stock">
                <ClothPicture assets={assets} />
                剩余 <b>{v.stock}</b> 枚
                {v.stock === 0
                  ? " · 已停止生产"
                  : ` · 掷出${v.number}时，各贸易玩家领取1枚`}
              </p>
              <div className="cloth-traders" aria-label="贸易玩家">
                {(v.traders || []).length ? (
                  (v.traders || []).map((i) => (
                    <span key={i}>
                      <i style={{ background: colors[i] }} />
                      {room.seats[i].name}
                      {g.players[i].eliminated ? "（已离场）" : ""}
                    </span>
                  ))
                ) : (
                  <span>尚无贸易玩家</span>
                )}
              </div>
              <p>
                船线从己方建筑连接到村落即可建立贸易，首次连接领取1枚；空村落不发放布匹。
              </p>
            </>
          )}
        </div>
      )}
    </section>
  );
}
