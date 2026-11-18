export class NativeAudioPreparation {
  private tasks = new Map<string, Set<AbortController>>();
  private factory: () => HTMLAudioElement;

  constructor(factory: () => HTMLAudioElement) { this.factory = factory; }

  async prepare(owner: string, sources: readonly string[], signal?: AbortSignal): Promise<boolean> {
    const choices = [...new Set(sources.filter(Boolean))];
    if (!choices.length || choices.length > 8 || signal?.aborted) return false;
    const abort = new AbortController();
    const cancel = () => abort.abort();
    signal?.addEventListener("abort", cancel, { once: true });
    let tasks = this.tasks.get(owner);
    if (!tasks) this.tasks.set(owner, tasks = new Set());
    tasks.add(abort);
    try {
      for (const source of choices) {
        if (abort.signal.aborted) return false;
        if (await this.decode(owner, source, abort.signal)) return true;
      }
      return false;
    } finally {
      signal?.removeEventListener("abort", cancel);
      tasks.delete(abort);
      if (!tasks.size) this.tasks.delete(owner);
    }
  }

  private decode(owner: string, source: string, signal: AbortSignal): Promise<boolean> {
    return new Promise(resolve => {
      if (signal.aborted) { resolve(false); return; }
      const media = this.factory();
      let finished = false;
      const done = (success: boolean) => {
        if (finished) return;
        finished = true;
        clearTimeout(timeout);
        signal.removeEventListener("abort", cancelled);
        for (const event of ["loadedmetadata", "loadeddata", "canplay", "canplaythrough"]) media.removeEventListener(event, ready);
        media.removeEventListener("error", failed);
        media.pause();
        media.removeAttribute("src");
        media.load();
        media.remove();
        resolve(success);
      };
      const ready = () => {
        if (media.readyState >= 3 && Number.isFinite(media.duration) && media.duration > 0) done(true);
      };
      const failed = () => done(false);
      const cancelled = () => done(false);
      const timeout = setTimeout(failed, 20000);
      signal.addEventListener("abort", cancelled, { once: true });
      for (const event of ["loadedmetadata", "loadeddata", "canplay", "canplaythrough"]) media.addEventListener(event, ready);
      media.addEventListener("error", failed, { once: true });
      media.dataset.platformAudio = "";
      media.dataset.platformPreparation = owner;
      media.hidden = true;
      media.preload = "auto";
      media.muted = true;
      media.volume = 0;
      if (typeof document !== "undefined") document.body.append(media);
      try {
        media.src = source;
        media.load();
        ready();
      } catch {
        failed();
      }
    });
  }

  cancel(owner?: string) {
    for (const [key, tasks] of this.tasks) {
      if (owner !== undefined && owner !== key) continue;
      for (const task of tasks) task.abort();
    }
  }
}
