import type { Room } from "./types";
import { catanSeatColor } from "./catan-player-colors";
import { explorerFarmAbilities } from "./catan-explorer-state";

export function ExplorerSpiceMission({
  room,
  assets,
}: {
  room: Room;
  assets: string;
}) {
  const g = room.game!.catan!,
    x = g.explorer!,
    mission = x.spice!;
  return (
    <section
      className="explorer-mission explorer-spice-mission"
      aria-label="香料任务进度"
    >
      <strong>
        {assets && (
          <img src={`${assets}/catan/explorer/spice-v1.webp`} alt="" />
        )}
        香料任务
      </strong>
      {g.players.map((p, id) => {
        const abilities = explorerFarmAbilities(g, id);
        return (
          <div key={id} className={p.eliminated ? "retired" : ""}>
            <span style={{ borderColor: catanSeatColor(g, id) }}>
              {room.seats[id]?.name}
            </span>
            <b>进度 {mission.progress[id]} / 6</b>
            <span>
              任务 {mission.scores[id]}分
              {mission.leader === id ? " · 领先" : ""}
            </span>
            <small className="explorer-spice-abilities">
              已派驻 {abilities.count}/6
              {abilities.swift > 0 && <span>航速＋{abilities.swift}</span>}
              {abilities.gold > 0 && (
                <span>换金币 {abilities.gold}次/回合</span>
              )}
              {abilities.pirate.length > 0 && (
                <span>驱赶 {abilities.pirate.join("、")} 也成功</span>
              )}
            </small>
          </div>
        );
      })}
      <p>
        每座农场永久留驻1名船员，领取1袋香料。运到议会岛交付；即使交付或失去香料，能力仍保留。
      </p>
      {room.you >= 0 && (
        <p>
          本回合农场换金币{" "}
          {mission.goldUse?.sequence === x.sequence &&
          mission.goldUse.player === room.you
            ? mission.goldUse.count
            : 0}
          /{explorerFarmAbilities(g, room.you).gold} 次
        </p>
      )}
      {x.pirate?.lastChase && (
        <p>
          最近驱赶：{room.seats[x.pirate.lastChase.player]?.name}掷出
          {x.pirate.lastChase.die}，
          {x.pirate.lastChase.success ? "成功" : "未成功"}。
        </p>
      )}
    </section>
  );
}
