export type PokerEvent = {
  id: number;
  type: string;
  seat: number;
  cards?: number[];
  at: number;
  kind?: string;
  value?: number;
  label?: string;
};
export type PokerSoundMapping =
  Record<string, string> | ((event: PokerEvent) => string | undefined);
// Feedback policy only. Each game owns its event-to-clip vocabulary.
export function selectPokerSound(
  events: readonly PokerEvent[],
  mapping: PokerSoundMapping,
): string | undefined {
  const audible = events
    .map((event) => ({
      event,
      sound:
        typeof mapping === "function" ? mapping(event) : mapping[event.type],
    }))
    .filter((v) => !!v.sound);
  const plays = audible.filter((v) => v.event.type === "play");
  const choices = plays.length ? plays : audible;
  return choices.reduce<
    { event: PokerEvent; sound: string | undefined } | undefined
  >(
    (latest, next) =>
      !latest || next.event.id > latest.event.id ? next : latest,
    undefined,
  )?.sound;
}
