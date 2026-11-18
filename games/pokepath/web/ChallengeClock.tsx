import { useFact } from "./I18n";

export function ChallengeClock({ elapsedMS }: { elapsedMS: number }) {
  const fact = useFact();
  const centiseconds = Math.floor(Math.max(0, elapsedMS) / 10);
  const seconds = Math.floor(centiseconds / 100);
  const two = (part: number) => String(part).padStart(2, "0");
  const time = `${two(Math.floor(seconds / 3600))}:${two(Math.floor(seconds / 60) % 60)}:${two(seconds % 60)}.${two(centiseconds % 100)}`;
  const title = fact("text.challenge.label");
  return <>
    <span className="pokepath-challenge-status" data-challenge-status>{title.toUpperCase()}</span>
    <output className="pokepath-challenge-clock" data-challenge-clock role="timer" aria-live="off" aria-label={title}>{time}</output>
  </>;
}
