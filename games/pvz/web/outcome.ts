import type { RoundResult } from "./types";

const team = (id: number) => `${id === 0 ? "A" : "B"}队`;
export function formatElapsed(milliseconds: number) {
  const ms = Math.max(0, Math.round(milliseconds));
  return `${Math.floor(ms / 60000)}:${String(Math.floor(ms / 1000) % 60).padStart(2, "0")}.${String(Math.floor((ms % 1000) / 10)).padStart(2, "0")}`;
}
export function roundRow(round: RoundResult, index: number) {
  return {
    round: `第${index + 1}回合`,
    attack: team(round.attackTeam),
    defense: team(1 - round.attackTeam),
    result: round.breached ? "已攻破" : "未攻破",
    time: formatElapsed(round.elapsedMS),
  };
}
export function describeOutcome(
  rounds: readonly RoundResult[],
  winnerTeam?: number,
): string {
  if (rounds.length !== 2 || winnerTeam === undefined) return "";
  if (rounds.every((r) => !r.breached)) return "和局：双方进攻均未突破防线。";
  if (
    rounds.every((r) => r.breached) &&
    rounds[0].elapsedMS === rounds[1].elapsedMS
  )
    return `和局：双方均攻破，突破用时同为 ${formatElapsed(rounds[0].elapsedMS)}。`;
  if (winnerTeam !== 0 && winnerTeam !== 1) return "";
  const winner = rounds.find((r) => r.attackTeam === winnerTeam),
    other = rounds.find((r) => r.attackTeam !== winnerTeam);
  if (!winner || !other) return "";
  if (winner.breached && !other.breached)
    return `${team(winnerTeam)}获胜：${team(winnerTeam)}成功攻破，${team(1 - winnerTeam)}未能攻破。`;
  if (winner.breached && other.breached && winner.elapsedMS < other.elapsedMS) {
    const difference = ((other.elapsedMS - winner.elapsedMS) / 1000)
      .toFixed(2)
      .replace(/\.?0+$/, "");
    return `${team(winnerTeam)}获胜：双方均攻破，${team(winnerTeam)}用时 ${formatElapsed(winner.elapsedMS)}，比${team(1 - winnerTeam)}快 ${difference} 秒。`;
  }
  return `${team(winnerTeam)}获胜，判定以服务器记录为准。`;
}
