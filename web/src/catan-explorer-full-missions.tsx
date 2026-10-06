import type { Room } from "./types";
import { catanSeatColor } from "./catan-player-colors";
import {
  explorerFarmAbilities,
  explorerLairTotal,
} from "./catan-explorer-state";

export function ExplorerFullMissions({
  room,
  assets,
  fishRolling = false,
}: {
  room: Room;
  assets: string;
  fishRolling?: boolean;
}) {
  const g = room.game!.catan!,
    x = g.explorer!;
  const tracks = [
    { name: "巢穴", limit: 7, mission: x.lairs! },
    { name: "鱼群", limit: 7, mission: x.fish! },
    { name: "香料", limit: 6, mission: x.spice! },
  ];
  const you = room.you;
  const used =
    x.spice!.goldUse?.sequence === x.sequence && x.spice!.goldUse.player === you
      ? x.spice!.goldUse.count
      : 0;
  return (
    <section className="explorer-full-missions" aria-label="三任务进度">
      <div className="explorer-full-heading">
        <strong>三任务进度</strong>
        <span>己方回合 {x.board.target} 分获胜</span>
      </div>
      <table>
        <caption className="sr-only">
          各玩家的任务进度与得分，领先分已计入任务得分
        </caption>
        <thead>
          <tr>
            <th scope="col">玩家</th>
            {tracks.map((track) => (
              <th scope="col" key={track.name}>
                {track.name}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {g.players.map((p, id) => (
            <tr
              key={id}
              className={p.eliminated ? "retired" : id === you ? "is-you" : ""}
            >
              <th scope="row">
                <span
                  style={{ borderColor: catanSeatColor(g, id) }}
                  title={room.seats[id]?.name}
                >
                  {room.seats[id]?.name}
                </span>
                {p.eliminated && <small>已离场</small>}
              </th>
              {tracks.map(({ name, limit, mission }) => (
                <td
                  key={name}
                  className={mission.leader === id ? "is-leader" : ""}
                >
                  <b>
                    {mission.progress[id]}
                    <small>/{limit}</small>
                  </b>
                  <span>
                    {mission.scores[id]}分
                    {mission.leader === id && (
                      <em title="领先奖励已计入得分"> · 领先</em>
                    )}
                  </span>
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      <p className="explorer-full-summary">
        已解放 {x.lairs!.sites.filter((site) => site.resolved).length}/
        {explorerLairTotal(g)} 座巢穴 · 供应{" "}
        {x.cargo.fish?.filter((loc) => loc.kind === "supply").length ?? 0} 群鱼
      </p>
      {x.lairs!.battle && (
        <p role="status">
          巢穴{x.lairs!.battle.tile + 1}：
          {x
            .lairs!.battle.candidates.map((id) => room.seats[id]?.name)
            .join("、")}
          掷英雄骰。
        </p>
      )}
      <details>
        <summary>农场能力与任务说明</summary>
        <div className="explorer-full-abilities">
          {g.players.map((p, id) => {
            const a = explorerFarmAbilities(g, id);
            return (
              <div key={id} className={p.eliminated ? "retired" : ""}>
                <strong style={{ borderColor: catanSeatColor(g, id) }}>
                  {room.seats[id]?.name}
                </strong>
                <span>已派驻 {a.count}/6</span>
                {a.swift > 0 && <span>航速＋{a.swift}</span>}
                {a.gold > 0 && <span>换金币 {a.gold}次/回合</span>}
                {a.pirate.length > 0 && (
                  <span>驱赶 {a.pirate.join("、")} 也成功</span>
                )}
              </div>
            );
          })}
        </div>
        {you >= 0 && !room.spectating && (
          <p>
            我的农场换金币：本回合 {used}/{explorerFarmAbilities(g, you).gold}{" "}
            次。
          </p>
        )}
        <p>
          巢穴数字在解放前隐藏。鱼群与香料送到议会岛锚点交付，任务领先奖励已计入表中得分。
        </p>
        <p className="explorer-full-spice-note">
          {assets && (
            <img src={`${assets}/catan/explorer/spice-v1.webp`} alt="" />
          )}
          每座农场永久留驻1名船员、领取1袋香料；交付或失去香料后，能力仍保留。
        </p>
      </details>
      {x.fish!.lastRoll && (
        <p
          role="status"
          key={x.fish!.lastRoll.sequence}
          className={fishRolling ? "explorer-fish-roll" : undefined}
        >
          最近捕鱼骰：{room.seats[x.fish!.lastRoll.player]?.name}掷出{" "}
          {x.fish!.lastRoll.die}，
          {x.fish!.lastRoll.spawned >= 0 ? "出现一群鱼" : "未出现鱼群"}。
        </p>
      )}
      {x.pirate?.lastChase && (
        <p>
          最近驱赶：{room.seats[x.pirate.lastChase.player]?.name}掷出{" "}
          {x.pirate.lastChase.die}，
          {x.pirate.lastChase.success ? "成功" : "未成功"}。
        </p>
      )}
    </section>
  );
}
