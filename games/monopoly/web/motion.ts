export function motionPath(event: {
  kind: string;
  from: number;
  to: number;
  steps: number;
}): number[] {
  if (
    ![event.from, event.to, event.steps].every(Number.isInteger) ||
    event.from < 0 ||
    event.from > 39 ||
    event.to < 0 ||
    event.to > 39
  )
    return [];
  if (event.kind === "jump") return [event.to];
  if (
    event.kind !== "move" ||
    Math.abs(event.steps) > 40 ||
    (((event.from + event.steps) % 40) + 40) % 40 !== event.to
  )
    return [];
  return Array.from(
    { length: Math.abs(event.steps) },
    (_, i) => (event.from + Math.sign(event.steps) * (i + 1) + 40) % 40,
  );
}
export function squarePoint(id: number) {
  const column = id <= 10 ? 10 - id : id <= 20 ? 0 : id <= 30 ? id - 20 : 10;
  const row = id <= 10 ? 10 : id <= 20 ? 20 - id : id <= 30 ? 0 : id - 30;
  const center = (n: number) =>
    ((n === 0 ? 0.8 : n === 10 ? 11.4 : n + 1.1) / 12.2) * 100;
  return { x: center(column), y: center(row) };
}
