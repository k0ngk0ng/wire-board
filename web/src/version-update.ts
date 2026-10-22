export function isEntryAsset(path: string) {
  return (
    /^\/assets\/[^/?#]+\.(js|css)$/.test(path) ||
    /^https:\/\/cdn\.ichenj\.com\/bga\/web\/[a-f0-9]{40}\/assets\/[^/?#]+\.(js|css)$/.test(
      path,
    )
  );
}
export type ReleaseInfo = { version: string; assets: string[] };
const assetKey = (assets: string[]) => [...new Set(assets)].sort().join("\n");
function validInfo(value: unknown): value is ReleaseInfo {
  if (!value || typeof value !== "object") return false;
  const data = value as ReleaseInfo;
  return (
    typeof data.version === "string" &&
    /^[\w.+-]{1,64}$/.test(data.version) &&
    Array.isArray(data.assets) &&
    data.assets.length <= 100 &&
    data.assets.every(
      (path) =>
        typeof path === "string" && isEntryAsset(path) && path.length < 500,
    )
  );
}

export class VersionMonitor {
  private currentVersion: string | undefined;
  private noticeKey = "";
  private active: AbortController | null = null;
  private stopped = false;
  private readonly loadedAssets: string;
  private read: (signal: AbortSignal) => Promise<unknown>;
  private changed: (info: ReleaseInfo | null) => void;
  constructor(
    assets: string[],
    read: (signal: AbortSignal) => Promise<unknown>,
    changed: (info: ReleaseInfo | null) => void,
  ) {
    this.read = read;
    this.changed = changed;
    this.loadedAssets = assetKey(assets);
  }
  async check() {
    if (this.stopped || this.active) return;
    const request = new AbortController();
    this.active = request;
    const timeout = setTimeout(() => request.abort(), 10000);
    try {
      const info = await this.read(request.signal);
      if (this.stopped || request.signal.aborted || !validInfo(info)) return;
      const remoteAssets = assetKey(info.assets);
      if (this.loadedAssets && !remoteAssets) return;
      const differentAssets = remoteAssets !== this.loadedAssets;
      if (!differentAssets && this.currentVersion === undefined)
        this.currentVersion = info.version;
      const outdated =
        differentAssets ||
        (this.currentVersion !== undefined &&
          info.version !== this.currentVersion);
      const key = outdated ? info.version + "\n" + remoteAssets : "";
      if (key !== this.noticeKey) {
        this.noticeKey = key;
        this.changed(outdated ? info : null);
      }
    } catch {
      // Offline, timeout and temporarily unavailable metadata must not interrupt
      // gameplay, erase a known update or change the loaded-version baseline.
    } finally {
      clearTimeout(timeout);
      if (this.active === request) this.active = null;
    }
  }
  stop() {
    this.stopped = true;
    this.active?.abort();
  }
}
