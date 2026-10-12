import type { Room, CatanProgressEvent } from "./types";

export type CityMotion = {
  moves: { from: number; to: number }[];
  flashes: { vertex: number; loss: boolean }[];
  merchant?: { from: number; to: number };
  ship?: { from: number; to: number; attack: boolean };
  cards: CatanProgressEvent[];
  players: number[];
};

// A visual transition never invents intermediate actions across missing snapshots,
// a different viewer, a rematch or initial setup. Only public map data is compared.
export function catanCityMotion(before: Room, after: Room): CityMotion | null {
  const a = before.game?.catan,
    b = after.game?.catan;
  const old = a?.citiesKnights,
    next = b?.citiesKnights;
  if (
    !a ||
    !b ||
    !old ||
    !next ||
    before.id !== after.id ||
    before.you !== after.you ||
    !!before.spectating !== !!after.spectating ||
    before.status !== "playing" ||
    after.version !== before.version + 1 ||
    b.rollId < a.rollId ||
    b.rollId > a.rollId + 1 ||
    a.setupStep < (a.setupLimit ?? a.players.length * 2) ||
    b.setupStep < (b.setupLimit ?? b.players.length * 2) ||
    a.tiles.length !== b.tiles.length ||
    a.players.length !== b.players.length
  )
    return null;
  const motion: CityMotion = { moves: [], flashes: [], cards: [], players: [] };
  const previous = [...old.knights];
  if (old.pending?.kind === "knight_retreat" && old.pending.knight)
    previous.push(old.pending.knight);
  for (const knight of next.knights) {
    const same = old.knights.find(
      (n) => n.owner === knight.owner && n.vertex === knight.vertex,
    );
    if (same) {
      if (same.strength !== knight.strength || same.active !== knight.active)
        motion.flashes.push({
          vertex: knight.vertex,
          loss: !knight.active && same.active,
        });
      continue;
    }
    const sources = previous.filter(
      (n) =>
        n.owner === knight.owner &&
        n.strength === knight.strength &&
        !next.knights.some((k) => k.owner === n.owner && k.vertex === n.vertex),
    );
    if (
      sources.length === 1 &&
      a.vertices[sources[0].vertex] &&
      b.vertices[knight.vertex]
    )
      motion.moves.push({ from: sources[0].vertex, to: knight.vertex });
    else motion.flashes.push({ vertex: knight.vertex, loss: false });
  }
  for (const vertex of b.vertices) {
    const prior = a.vertices[vertex.id];
    if (
      prior &&
      (prior.level !== vertex.level ||
        old.walls.includes(vertex.id) !== next.walls.includes(vertex.id) ||
        old.metropolises.indexOf(vertex.id) !==
          next.metropolises.indexOf(vertex.id))
    )
      motion.flashes.push({
        vertex: vertex.id,
        loss:
          prior.level > vertex.level ||
          (old.metropolises.includes(vertex.id) &&
            !next.metropolises.includes(vertex.id)),
      });
  }
  if (
    next.merchant &&
    (next.merchant.tile !== old.merchant?.tile ||
      next.merchant.owner !== old.merchant?.owner)
  )
    motion.merchant = {
      from: old.merchant?.tile ?? next.merchant.tile,
      to: next.merchant.tile,
    };
  if (
    (next.barbarianPosition === old.barbarianPosition + 1 &&
      b.rollId === a.rollId + 1) ||
    (next.invasions === old.invasions + 1 && next.barbarianPosition === 0)
  )
    motion.ship = {
      from: old.barbarianPosition,
      to: next.barbarianPosition,
      attack: next.invasions === old.invasions + 1,
    };
  const first = (old.progressEventId ?? 0) + 1;
  const events = (next.progressEvents ?? []).filter((e) => e.id >= first);
  if (
    events.length &&
    events.every((event, i) => event.id === first + i) &&
    events.at(-1)!.id === next.progressEventId
  )
    motion.cards = events;
  next.players.forEach((p, i) => {
    const prev = old.players[i];
    if (
      prev &&
      (p.defenderPoints !== prev.defenderPoints ||
        p.progressPoints !== prev.progressPoints ||
        p.improvements.some(
          (level, track) => level !== prev.improvements[track],
        ))
    )
      motion.players.push(i);
  });
  return motion;
}
