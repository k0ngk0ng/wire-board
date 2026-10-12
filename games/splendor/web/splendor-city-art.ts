// Original illustrations, without the printed conditions. Local tile IDs do
// not follow the atlas order. Crops omit the white gutters between cities.
const cityArtTop: Record<number, number> = {
  1: 570, // Madrid
  2: 8, // Amboise
  3: 194, // Timbuktu
  4: 1130, // Delhi
  5: 756, // Samarkand
  6: 944, // Seoul
  7: 381, // Krakow
};

export function cityAtlasStyle(assets: string, tile: number) {
  const top = cityArtTop[tile];
  if (!assets || !Number.isInteger(tile) || top === undefined) return undefined;
  return {
    backgroundImage: `url("${assets}/splendor/expansions/cities-art-v1.webp")`,
    backgroundSize: `${(360 / 350) * 100}% ${(1310 / 172) * 100}%`,
    backgroundPosition: `50% ${(top / (1310 - 172)) * 100}%`,
    backgroundRepeat: "no-repeat",
  };
}
