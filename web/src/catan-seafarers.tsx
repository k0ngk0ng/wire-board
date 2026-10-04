import type { CatanState } from "./types";

const colors = ["blue", "red", "white", "orange", "purple", "green"];
const paints = [
  "#3078be",
  "#c94737",
  "#f5eee1",
  "#e5902e",
  "#794287",
  "#388146",
];
export function CatanShip({
  assets,
  player,
  warship = false,
}: {
  assets: string;
  player: number;
  warship?: boolean;
}) {
  const ship = assets ? (
    <image
      href={`${assets}/catan/seafarers/ship-${colors[player]}-v1.webp`}
      x="-23"
      y="-18"
      width="46"
      height="36"
      pointerEvents="none"
    />
  ) : (
    <path
      d="M-22 5L-15 14H15L22 5H3V-17H-1V5Z"
      fill={paints[player]}
      stroke="#453b32"
      strokeWidth="1.5"
    />
  );
  return (
    <g>
      {ship}
      {warship && (
        <g pointerEvents="none">
          <title>战舰</title>
          <path
            d="M0-15L9-12V-5Q9 2 0 6Q-9 2-9-5V-12Z"
            fill="#f9e9ac"
            stroke="#49341d"
            strokeWidth="1.5"
          />
          <path d="M-4-8L4 0M4-8L-4 0" stroke="#49341d" strokeWidth="2" />
        </g>
      )}
    </g>
  );
}
export function CatanPirate({ assets }: { assets: string }) {
  return assets ? (
    <image
      href={`${assets}/catan/seafarers/pirate-v1.webp`}
      x="-21"
      y="-22"
      width="42"
      height="44"
      pointerEvents="none"
    />
  ) : (
    <path
      d="M-22 8L-14 18H16L22 8H2V-21L-15 2H-2V8Z"
      fill="#595754"
      stroke="#ddd9cb"
      strokeWidth="1.5"
    />
  );
}

export function CatanDesertRegions({ game }: { game: CatanState }) {
  const sea = game.seafarers;
  if (sea?.scenario !== "desert" || !sea.islands || !sea.startIslands)
    return null;
  const regions = sea.islands;
  const boundaries = game.edges.flatMap((edge) => {
    const tiles =
      edge.tiles ||
      game.tiles
        .filter(
          (tile) =>
            tile.vertices.includes(edge.a) && tile.vertices.includes(edge.b),
        )
        .map((tile) => tile.id);
    const ids = [...new Set(tiles.map((id) => regions[id]))];
    const region = ids.find((id) => id >= 0 && !sea.startIslands!.includes(id));
    if (region === undefined || (tiles.length > 1 && ids.length === 1))
      return [];
    return [{ edge, region }];
  });
  const acrossDesert = new Set(
    boundaries
      .filter(({ edge }) =>
        edge.tiles?.some((id) => game.tiles[id].resource === 5),
      )
      .map(({ region }) => region),
  );
  return (
    <g className="catan-exploration-borders" pointerEvents="none">
      <title>边框标示探索区域；每位玩家首次在各区域定居额外获得2分</title>
      {boundaries.map(({ edge, region }) => {
        const a = game.vertices[edge.a],
          b = game.vertices[edge.b];
        return (
          <line
            key={edge.id}
            x1={a.x}
            y1={a.y}
            x2={b.x}
            y2={b.y}
            stroke={
              game.players.length <= 4 && acrossDesert.has(region)
                ? "#34713d"
                : "#243e73"
            }
            strokeWidth="3"
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        );
      })}
    </g>
  );
}
