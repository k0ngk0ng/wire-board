export type VerifiedFile = { url: string; bytes: number };

/** Only completed byte/version checks are retained, never response bodies.
 * Every new match still initializes its scene and waits for onReady.
 */
export class VerifiedAssetCache {
  private readonly completed = new Set<string>();
  private key(file: VerifiedFile, build: string, origin: string) {
    return JSON.stringify([build, origin, file.url, file.bytes]);
  }
  has(file: VerifiedFile, build: string, origin: string): boolean {
    return this.completed.has(this.key(file, build, origin));
  }
  async load(
    file: VerifiedFile,
    build: string,
    origin: string,
    signal: AbortSignal,
    progress: (bytes: number) => void,
    verify: () => Promise<void>,
  ): Promise<"cached" | "loaded"> {
    signal.throwIfAborted();
    if (this.has(file, build, origin)) {
      progress(file.bytes);
      return "cached";
    }
    await verify();
    signal.throwIfAborted();
    this.completed.add(this.key(file, build, origin));
    return "loaded";
  }
}
