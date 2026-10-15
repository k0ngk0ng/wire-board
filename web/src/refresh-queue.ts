// One in-flight request, plus one coalesced batch of refresh callers. Every
// completed snapshot may be applied; a busy realtime stream cannot starve UI.
export function refreshQueue(load: () => Promise<void>) {
  type Waiting = { resolve: () => void; reject: (error: unknown) => void };
  let active = false;
  let waiting: Waiting[] = [];
  const drain = async () => {
    active = true;
    while (waiting.length) {
      const batch = waiting;
      waiting = [];
      try {
        await load();
        batch.forEach(({ resolve }) => resolve());
      } catch (error) {
        batch.forEach(({ reject }) => reject(error));
      }
    }
    active = false;
  };
  return () =>
    new Promise<void>((resolve, reject) => {
      waiting.push({ resolve, reject });
      if (!active) void drain();
    });
}
