import type { GameAudioPort } from "../../../web/src/game-api/module";
import type { SceneLoadProgress } from "./scene-renderer";
import type { PalAudioState } from "./audio-types";
import release from "../assets/audio-release.json";

type Clip = { path: string; samples: number; sampleRate: number };
type File = { size: number; sha256: string; mime: string };
type Index = { version: number; sounds: Record<string, Clip>; music: Record<string, Clip>; cd: Record<string, Clip>; files: Record<string, File> };
export type PreparedAudio = { url: string; duration: number };
type Entry = { promise: Promise<PreparedAudio>; url?: string; bytes: number; used: number };
type Publication = { assetID: string; assetsBaseURL: string; cdnVerified: boolean; files?: Record<string, { url: string; bytes: number; sha256: string; mime: string; encoding: string }> };

const digest = async (data: ArrayBuffer) => [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map(value => value.toString(16).padStart(2, "0")).join("");

export class PalAudioAssets {
  private base: string;
  private publication: Publication;
  private expected: string;
  private audio: GameAudioPort;
  private progress?: (value: SceneLoadProgress) => void;
  private abort = new AbortController();
  private signal: AbortSignal;
  private index?: Promise<Index>;
  private indexBytes = 0;
  private files = new Map<string, File>();
  private entries = new Map<string, Entry>();
  private counts = new Map<string, number>();
  private planned = new Set<string>();
  private key = "";
  private preparation?: Promise<void>;
  private serial = 0;
  private jobs: (() => Promise<void>)[] = [];
  private active = 0;

  constructor(expected: string, audio: GameAudioPort, options: { signal: AbortSignal; progress: (value: SceneLoadProgress) => void }) {
    const publication = (release as { version: number; catalogs: Record<string, Publication> }).catalogs[expected];
    if (!publication?.cdnVerified || publication.assetID !== expected || !publication.assetsBaseURL.startsWith("https://cdn.ichenj.com/bga/")) throw Error("此存档的音频版本尚未发布");
    this.publication = publication;
    this.base = publication.assetsBaseURL.replace(/\/$/, "");
    this.expected = expected;
    this.audio = audio;
    this.progress = options.progress;
    this.signal = AbortSignal.any([options.signal, this.abort.signal]);
  }

  private url(path: string) {
    const published = this.publication.files?.[path], file = this.files.get(path);
    if (this.publication.files && !published) throw Error("仙剑音频发布记录不完整");
    if (published && (path === "index.json" ? published.sha256 !== this.expected : file && (published.sha256 !== file.sha256 || published.bytes !== file.size || published.mime !== file.mime || published.encoding !== ""))) throw Error("仙剑音频发布记录不匹配");
    const url = published?.url ?? `${this.base}/${path}`;
    if (!url.startsWith("https://cdn.ichenj.com/bga/") || /[?#\\]/.test(url) || url.includes("/../")) throw Error("仙剑音频发布地址无效");
    return url;
  }

  private report(label = "加载原版音频") {
    const total = this.indexBytes + [...this.planned].reduce((sum, path) => sum + (this.files.get(path)?.size ?? 0), 0);
    const loaded = this.indexBytes + [...this.planned].reduce((sum, path) => sum + (this.counts.get(path) ?? 0), 0);
    this.progress?.({ percent: total ? loaded / total * 100 : 0, loaded, total, label });
  }

  private async download(path: string, maximum: number, update: (bytes: number) => void): Promise<ArrayBuffer> {
    const response = await fetch(this.url(path), { signal: AbortSignal.any([this.signal, AbortSignal.timeout(45000)]) });
    if (!response.ok) throw Error(`仙剑音频加载失败：${path}`);
    const reader = response.body?.getReader();
    if (!reader) {
      const data = await response.arrayBuffer();
      if (data.byteLength > maximum) throw Error("仙剑音频超过资源上限");
      update(data.byteLength);
      return data;
    }
    const chunks: Uint8Array[] = [];
    let size = 0;
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        size += value.byteLength;
        if (size > maximum) { await reader.cancel(); throw Error("仙剑音频超过资源上限"); }
        chunks.push(value);
        update(size);
      }
    } finally {
      reader.releaseLock();
    }
    const data = new Uint8Array(size);
    let at = 0;
    for (const chunk of chunks) { data.set(chunk, at); at += chunk.length; }
    return data.buffer;
  }

  private catalog(): Promise<Index> {
    this.index ??= (async () => {
      if (!/^[a-f0-9]{64}$/.test(this.expected)) throw Error("仙剑音频版本无效");
      const data = await this.download("index.json", 2097152, () => {});
      if (await digest(data) !== this.expected) throw Error("仙剑音频清单校验失败");
      const index: Index = JSON.parse(new TextDecoder().decode(data));
      if (index.version !== 1 || !index.files || !index.sounds || !index.music || !index.cd) throw Error("仙剑音频清单不兼容");
      this.indexBytes = data.byteLength;
      this.report();
      return index;
    })().catch(error => { this.index = undefined; throw error; });
    return this.index;
  }

  private clip(index: Index, kind: number, id: number): Clip {
    if (!Number.isSafeInteger(id) || id < 0) throw Error("仙剑音频编号无效");
    const clip = (kind === 1 ? index.music : kind === 2 ? index.sounds : kind === 3 ? index.cd : {})[String(id)];
    const allowed = kind === 1 ? [`music/${id}.flac`] : kind === 2 ? [`sounds/${id}.flac`, `sounds/${id}.wav`] : [`cd/${String(id).padStart(2, "0")}.ogg`];
    if (!clip || !allowed.includes(clip.path) || !Number.isSafeInteger(clip.samples) || clip.samples <= 0 || !Number.isSafeInteger(clip.sampleRate) || clip.sampleRate <= 0) throw Error("仙剑音频信息无效");
    const file = index.files[clip.path];
    if (!file || !Number.isSafeInteger(file.size) || file.size <= 0 || file.size > 67108864 || !/^[a-f0-9]{64}$/.test(file.sha256) || !["audio/flac", "audio/wav", "audio/ogg"].includes(file.mime)) throw Error("仙剑音频资源信息无效");
    this.files.set(clip.path, file);
    return clip;
  }

  private work<T>(job: () => Promise<T>): Promise<T> {
    return new Promise((resolve, reject) => {
      this.jobs.push(async () => { try { this.signal.throwIfAborted(); resolve(await job()); } catch (error) { reject(error); } });
      this.pump();
    });
  }

  private pump() {
    while (this.active < 6 && this.jobs.length) {
      const job = this.jobs.shift()!;
      this.active++;
      void job().finally(() => { this.active--; this.pump(); });
    }
  }

  private trim() {
    let bytes = [...this.entries.values()].reduce((sum, entry) => sum + entry.bytes, 0);
    for (const [path, entry] of [...this.entries].sort((a, b) => a[1].used - b[1].used)) {
      if (bytes <= 67108864 && this.entries.size <= 32) break;
      if (this.planned.has(path) || !entry.url) continue;
      bytes -= entry.bytes;
      this.entries.delete(path);
      this.counts.delete(path);
    }
  }

  async source(kind: number, id: number): Promise<PreparedAudio> {
    const index = await this.catalog(), clip = this.clip(index, kind, id), file = this.files.get(clip.path)!;
    let entry = this.entries.get(clip.path);
    if (!entry) {
      const created: Entry = { promise: Promise.resolve({ url: "", duration: 0 }), bytes: 0, used: ++this.serial };
      created.promise = this.work(async () => {
        const data = await this.download(clip.path, file.size, count => { this.counts.set(clip.path, count); this.report(); });
        if (data.byteLength !== file.size || await digest(data) !== file.sha256) throw Error(`仙剑音频校验失败：${clip.path}`);
        this.signal.throwIfAborted();
        const url = this.url(clip.path);
        this.report("初始化原版音频");
        if (!await this.audio.prepare([url], { signal: this.signal })) throw Error(`仙剑音频无法解码：${clip.path}`);
        this.signal.throwIfAborted();
        created.url = url;
        created.bytes = file.size;
        this.trim();
        this.report();
        return { url, duration: clip.samples / clip.sampleRate };
      }).catch(error => {
        if (this.entries.get(clip.path) === created) this.entries.delete(clip.path);
        this.counts.delete(clip.path);
        this.report();
        throw error;
      });
      this.entries.set(clip.path, created);
      entry = created;
    }
    entry.used = ++this.serial;
    return entry.promise;
  }

  prepare(view?: PalAudioState): Promise<void> {
    if (!view) return Promise.resolve();
    if (view.assetID !== this.expected) return Promise.reject(Error("仙剑音频版本已变化"));
    const key = JSON.stringify([view.music?.seq, view.sounds.map(sound => sound.seq)]);
    if (key === this.key && this.preparation) return this.preparation;
    this.key = key;
    this.preparation = (async () => {
      const index = await this.catalog(), required = view.sounds.map(sound => [2, sound.id]);
      if (view.music) required.push([view.music.kind, view.music.id]);
      this.planned = new Set(required.map(([kind, id]) => this.clip(index, kind, id).path));
      this.report();
      await Promise.all(required.map(([kind, id]) => this.source(kind, id)));
      this.trim();
      this.report();
    })();
    return this.preparation;
  }

  retry() { this.key = ""; this.preparation = undefined; }

  dispose() {
    this.abort.abort();
    this.entries.clear();
  }
}
