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
}: {
  assets: string;
  player: number;
}) {
  return assets ? (
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
