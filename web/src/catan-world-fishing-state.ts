import type { Room } from "./types";

// Legal vertex IDs come from the server's joint port/ground capacity check.
// Derive only the preview shape from public coast geometry, never new legality.
export function worldFishPreview(room: Room, vertex: number | null) {
  const game = room.game,
    g = game?.catan,
    setup = g?.fishing?.worldSetup;
  if (
    !g ||
    !setup ||
    setup.current === undefined ||
    vertex === null ||
    room.status !== "playing" ||
    game!.finished ||
    room.spectating ||
    room.you < 0 ||
    !g.players[room.you] ||
    g.players[room.you].eliminated ||
    game!.turn !== room.you ||
    game!.phase !== "catan_world_fish" ||
    !g.legal.fishGrounds?.includes(vertex)
  )
    return null;
  const coast = g.edges.filter((e) => {
    if (e.a !== vertex && e.b !== vertex) return false;
    const tiles =
      e.tiles?.map((id) => g.tiles[id]) ??
      g.tiles.filter(
        (t) => t.vertices.includes(e.a) && t.vertices.includes(e.b),
      );
    return (
      tiles.length > 0 &&
      tiles.length <= 2 &&
      tiles.some((t) => t && t.resource !== 6 && t.resource !== 8) &&
      (tiles.length === 1 || tiles.some((t) => t?.resource === 6))
    );
  });
  if (coast.length !== 2) return null;
  const edges = coast.sort((a, b) => a.id - b.id);
  return {
    number: setup.current,
    edges: edges.map((e) => e.id),
    vertices: [
      edges[0].a === vertex ? edges[0].b : edges[0].a,
      vertex,
      edges[1].a === vertex ? edges[1].b : edges[1].a,
    ],
  };
}
