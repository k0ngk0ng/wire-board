export type DownloadProgress = {
  loaded: number;
  total: number;
  percent: number;
};

/** Counts decoded resource bytes like the platform's normal preload path.
 * The immutable build URL owns caching; response bodies are not retained here. */
export class ResourceDownloads {
  private sizes: Readonly<Record<string, number>>;
  private counts = new Map<string, number>();
  private files = new Map<string, number>();
  private scopes: Set<string>;
  private report: (progress: DownloadProgress) => void;
  constructor(
    sizes: Readonly<Record<string, number>>,
    report: (progress: DownloadProgress) => void,
    scopes = ["art", "runtime"],
  ) {
    this.sizes = sizes;
    this.report = report;
    this.scopes = new Set(scopes);
  }
  plan(paths: string[]) {
    for (const path of paths)
      if (!this.files.has(path)) this.files.set(path, this.sizes[path] ?? 0);
    this.emit();
  }
  planned(scope: string) {
    this.scopes.delete(scope);
    this.emit();
  }
  complete(path: string) {
    this.counts.set(path, this.files.get(path) ?? this.sizes[path] ?? 0);
    this.emit();
  }
  private emit() {
    const total = [...this.files.values()].reduce((a, b) => a + b, 0);
    const loaded = [...this.counts.entries()].reduce(
      (sum, [path, n]) => sum + Math.min(n, this.files.get(path) ?? n),
      0,
    );
    this.report({
      loaded,
      total,
      percent: this.scopes.size
        ? 0
        : total
          ? Math.min(100, (loaded / total) * 100)
          : 100,
    });
  }
  async bytes(
    url: string,
    path: string,
    signal?: AbortSignal,
    fetcher: typeof fetch = fetch,
  ): Promise<Uint8Array<ArrayBuffer>> {
    this.plan([path]);
    try {
      const response = await fetcher(url, { signal, cache: "force-cache" });
      if (!response.ok)
        throw Error(`资源下载失败（${response.status}）：${path}`);
      const chunks: Uint8Array[] = [];
      let count = 0;
      const reader = response.body?.getReader();
      if (reader) {
        try {
          for (;;) {
            const next = await reader.read();
            if (next.done) break;
            chunks.push(next.value);
            count += next.value.byteLength;
            this.counts.set(path, count);
            this.emit();
          }
        } finally {
          reader.releaseLock();
        }
      } else {
        const data = new Uint8Array(await response.arrayBuffer());
        chunks.push(data);
        count = data.byteLength;
      }
      const expected = this.sizes[path];
      if (expected !== undefined && count !== expected)
        throw Error(`资源大小不匹配：${path}`);
      this.files.set(path, count);
      this.counts.set(path, count);
      this.emit();
      const bytes = new Uint8Array(count);
      let offset = 0;
      for (const chunk of chunks) {
        bytes.set(chunk, offset);
        offset += chunk.byteLength;
      }
      return bytes;
    } catch (error) {
      this.counts.set(path, 0);
      this.emit();
      throw error;
    }
  }
}

export async function runDownloads<T>(
  items: readonly T[],
  work: (item: T) => Promise<unknown>,
  concurrency = 6,
) {
  let cursor = 0;
  await Promise.all(
    Array.from({ length: Math.min(concurrency, items.length) }, async () => {
      while (cursor < items.length) await work(items[cursor++]);
    }),
  );
}
