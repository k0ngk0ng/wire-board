import type { GameRoom } from "../game-api/module";
export function pokerResources(_room: GameRoom, media: Record<string, number>) {
  let skin = "english";
  try {
    const saved = localStorage.getItem("wb_poker_skin");
    if (["english", "illustrated", "fourcolor"].includes(saved ?? ""))
      skin = saved!;
  } catch {}
  return Object.keys(media).filter(
    (path) =>
      path.startsWith(`/poker/v4/${skin}/`) ||
      path.startsWith("/poker/v1/backs/") ||
      path.startsWith("/poker/v2/"),
  );
}
