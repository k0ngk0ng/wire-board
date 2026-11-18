import type { Catalog, Pokemon, Species, Sprite } from "./types";

export function effectiveResourceSpecies(pokemon: Pokemon, catalog: Catalog, dna?: Record<string, string>): Species | undefined {
  const candidate = catalog.species[dna?.[String(pokemon.id)] ?? pokemon.species];
  const texture = pokemon.sprite?.image?.replace("/shiny/", "/normal/");
  if (!texture || candidate?.sprite.image === texture) return candidate;
  return Object.values(catalog.species).find(species => species.sprite.image === texture) ?? candidate;
}

export function pokemonSprites(pokemon: Pokemon, catalog: Catalog, dna?: Record<string, string>): Sprite[] {
  const species = effectiveResourceSpecies(pokemon, catalog, dna);
  const values: Sprite[] = [];
  const own = pokemon.sprite ?? species?.sprite;
  if (own) values.push(own);
  const keys = [String(species?.projectileId ?? "")];
  if (species?.abilityId === "trickOrTreat") keys.push("candy", "candy1", "spinarak", "apple");
  for (const key of keys) { const sprite = catalog.projectiles[key]?.sprite; if (typeof sprite === "object") values.push(sprite); }
  return values;
}
