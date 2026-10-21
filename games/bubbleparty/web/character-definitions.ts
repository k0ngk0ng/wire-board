import definition from "../assets/characters-v4.json" with { type: "json" };

export const characters = definition.characters;
export const selectableCharacters = characters.filter((c) => !c.hidden);
export const normalizeCharacter = (value?: number) =>
  Number.isInteger(value) && value! >= 0 && value! < characters.length
    ? value!
    : 0;
export const characterName = (value?: number) =>
  characters[normalizeCharacter(value)].name;
export const characterSourceCandidates = (base: string, file: string) => {
  const local = `/bubbleparty/v4/characters/${file}`;
  return base ? [`${base.replace(/\/$/, "")}${local}`, local] : [local];
};
