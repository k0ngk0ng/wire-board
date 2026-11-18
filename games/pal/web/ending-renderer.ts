export type EndingFrame = { clip: number; frame: number };
export type EndingPublication = {
  assetID: string;
  files: Record<string, { url: string; bytes: number; sha256: string; mime: string; encoding: string }>;
};
export type EndingOptions = {
  signal?: AbortSignal;
  progress?: (value: { percent: number; loaded: number; total: number; label: string }) => void;
  publication?: EndingPublication;
  prepareAudio?: (url: string) => Promise<boolean>;
};

type EndingMetadata = {
  version: number;
  sourceSHA256: string;
  width: number;
  height: number;
  rate: number[];
  frames: number;
  chunks: { first: number; frames: number; path: string }[];
  audio: {
    path: string;
    sampleRate: number;
    channels: number;
    bits: number;
    samples: number;
    pcmSHA256: string;
  };
};

type EndingIndex = {
  version: number;
  sourceCommit: string;
  files: Record<string, { size: number; sha256: string; mime: string }>;
};

const sources: Record<number, { frames: number; sha256: string }> = {
  4: { frames: 200, sha256: "600616fa3aa551ae3b783715325066051a4c46151ee8daeb4e0e4243b869aa73" },
  5: { frames: 2294, sha256: "30c6a574b2625140ca5ad1b456e8fb30ae9026b1684f90381630caff4e24f069" },
  6: { frames: 2196, sha256: "53a8c472d50263d512bae327e7064e9b88ff459788dc5bbee8fd7f8a55300217" },
};

export class EndingRenderer {
  private index?: Promise<EndingIndex>;
  private metadata = new Map<number, Promise<EndingMetadata>>();
  private chunks = new Map<string, Promise<Uint8Array>>();
  private audioReady = new Map<number, Promise<void>>();
  private sizes = new Map<string, number>();
  private counts = new Map<string, number>();
  private manifestBytes = 0;

  constructor(
    private base: string,
    private options: EndingOptions = {},
  ) {
    this.base = base.replace(/\/$/, "");
  }

  private url(name: string) {
    if (!this.options.publication) return `${this.base}/${name}`;
    const file = this.options.publication.files[name];
    if (!file || !file.url.startsWith("https://cdn.ichenj.com/bga/") || /[?#\\]/.test(file.url) || file.url.includes("/../") || !Number.isSafeInteger(file.bytes) || file.bytes <= 0 || file.bytes > 16777216 || !/^[a-f0-9]{64}$/.test(file.sha256)) throw Error("仙剑结局发布记录无效");
    return file.url;
  }

  private async download(name: string) {
    const file = this.options.publication?.files[name];
    const response = await fetch(this.url(name), { signal: this.options.signal ? AbortSignal.any([this.options.signal, AbortSignal.timeout(45000)]) : AbortSignal.timeout(45000) });
    if (!response.ok) throw Error("仙剑结局资源加载失败");
    if (file && response.headers.get("Content-Type")?.split(";")[0] !== file.mime) throw Error("仙剑结局媒体格式不匹配");
    const maximum = file?.bytes ?? 16777216, reader = response.body?.getReader();
    if (!reader) {
      const data = await response.arrayBuffer();
      if (data.byteLength > maximum) throw Error("仙剑结局资源超过大小限制");
      return data;
    }
    const chunks: Uint8Array[] = [];
    let size = 0;
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        size += value.byteLength;
        if (size > maximum) { await reader.cancel(); throw Error("仙剑结局资源超过大小限制"); }
        chunks.push(value);
        if (name === "index.json") this.manifestBytes = size;
        else this.counts.set(name, size);
        this.report();
      }
    } finally { reader.releaseLock(); }
    const data = new Uint8Array(size);
    let offset = 0;
    for (const chunk of chunks) { data.set(chunk, offset); offset += chunk.byteLength; }
    return data.buffer;
  }

  private report() {
    const total = this.manifestBytes + [...this.sizes.values()].reduce((a, b) => a + b, 0);
    const loaded = this.manifestBytes + [...this.counts.values()].reduce((a, b) => a + b, 0);
    this.options.progress?.({ percent: total ? loaded / total * 100 : 0, loaded, total, label: "加载原版结局" });
  }

  private catalog(): Promise<EndingIndex> {
    this.index ??= this.download("index.json").then(async (data) => {
      this.manifestBytes = data.byteLength;
      if (this.options.publication) {
        const file = this.options.publication.files["index.json"], digest = [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map(value => value.toString(16).padStart(2, "0")).join("");
        if (file.sha256 !== this.options.publication.assetID || digest !== file.sha256 || data.byteLength !== file.bytes) throw Error("仙剑结局清单校验失败");
      }
      const index: EndingIndex = JSON.parse(new TextDecoder().decode(data));
      if (index.version !== 1 || index.sourceCommit !== "ec8dfcb15eb78ce2e851332650b88c723e8eb760" || !index.files)
        throw Error("仙剑结局资源版本不兼容");
      return index;
    }).catch((error) => {
      this.index = undefined;
      throw error;
    });
    return this.index;
  }

  private async bytes(name: string): Promise<ArrayBuffer> {
    if (!/^ending\/[4-6](?:\/\d+\.rgb\.gz|\.json|\.flac)$/.test(name))
      throw Error("仙剑结局资源路径无效");
    const index = await this.catalog(), file = index.files[name];
    if (!file || !Number.isSafeInteger(file.size) || file.size <= 0 || file.size > 16777216)
      throw Error("仙剑结局资源清单无效");
    const published = this.options.publication?.files[name];
    if (this.options.publication && (!published || published.sha256 !== file.sha256 || published.bytes !== file.size || published.mime !== file.mime)) throw Error("仙剑结局发布记录不匹配");
    this.sizes.set(name, file.size);
    this.report();
    const data = await this.download(name);
    const digest = [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map((value) => value.toString(16).padStart(2, "0")).join("");
    if (data.byteLength !== file.size || digest !== file.sha256)
      throw Error("仙剑结局资源校验失败");
    this.counts.set(name, data.byteLength);
    this.report();
    return data;
  }

  info(clip: number): Promise<EndingMetadata> {
    if (!sources[clip]) return Promise.reject(Error("仙剑结局影片编号无效"));
    if (!this.metadata.has(clip)) this.metadata.set(clip, (async () => {
      const value: EndingMetadata = JSON.parse(new TextDecoder().decode(await this.bytes(`ending/${clip}.json`)));
      if (value.version !== 1 || value.width !== 288 || value.height !== 180 || value.frames !== sources[clip].frames || value.sourceSHA256 !== sources[clip].sha256 || value.rate?.length !== 2 || value.rate[0] !== 15 || value.rate[1] !== 1 || !Array.isArray(value.chunks))
        throw Error("仙剑结局影片信息无效");
      let first = 0;
      for (const chunk of value.chunks) {
        if (chunk.first !== first || !Number.isInteger(chunk.frames) || chunk.frames < 1 || chunk.frames > 15 || chunk.path !== `ending/${clip}/${first}.rgb.gz`)
          throw Error("仙剑结局影片分块无效");
        first += chunk.frames;
      }
      if (first !== value.frames || value.audio?.path !== `ending/${clip}.flac` || value.audio.sampleRate !== 22050 || value.audio.bits !== 8 || ![1, 2].includes(value.audio.channels) || !Number.isSafeInteger(value.audio.samples) || value.audio.samples <= 0)
        throw Error("仙剑结局音画长度无效");
      return value;
    })().catch((error) => {
      this.metadata.delete(clip);
      throw error;
    }));
    return this.metadata.get(clip)!;
  }

  async prepare(frame: EndingFrame) {
    const metadata = await this.info(frame.clip);
    if (!this.audioReady.has(frame.clip)) this.audioReady.set(frame.clip, this.bytes(metadata.audio.path).then(async () => {
      if (this.options.prepareAudio && !await this.options.prepareAudio(this.url(metadata.audio.path))) throw Error("仙剑结局音轨无法解码");
    }).catch((error) => {
      this.audioReady.delete(frame.clip);
      throw error;
    }));
    await Promise.all([this.rgba(frame), this.audioReady.get(frame.clip)]);
  }

  async audioSource(clip: number) {
    const metadata = await this.info(clip);
    return this.url(metadata.audio.path);
  }

  async rgba(frame: EndingFrame): Promise<Uint8ClampedArray> {
    const metadata = await this.info(frame.clip);
    if (!Number.isInteger(frame.frame) || frame.frame < 0 || frame.frame >= metadata.frames)
      throw Error("仙剑结局播放位置无效");
    const chunk = metadata.chunks.find((part) => frame.frame >= part.first && frame.frame < part.first + part.frames)!;
    let pending = this.chunks.get(chunk.path);
    if (pending) this.chunks.delete(chunk.path);
    else pending = (async () => {
      const compressed = await this.bytes(chunk.path);
      const stream = new Blob([compressed]).stream().pipeThrough(new DecompressionStream("gzip"));
      const pixels = new Uint8Array(await new Response(stream).arrayBuffer());
      if (pixels.length !== chunk.frames * 288 * 180 * 3)
        throw Error("仙剑结局画面分块长度无效");
      return pixels;
    })().catch((error) => {
      this.chunks.delete(chunk.path);
      throw error;
    });
    this.chunks.set(chunk.path, pending);
    while (this.chunks.size > 2) this.chunks.delete(this.chunks.keys().next().value!);
    const pixels = await pending, offset = (frame.frame - chunk.first) * 288 * 180 * 3;
    const rgba = new Uint8ClampedArray(320 * 200 * 4);
    for (let y = 0; y < 200; y++) for (let x = 0; x < 320; x++) {
      const source = offset + (Math.floor(y * 0.9) * 288 + Math.floor(x * 0.9)) * 3;
      const target = (y * 320 + x) * 4;
      rgba[target] = pixels[source];
      rgba[target + 1] = pixels[source + 1];
      rgba[target + 2] = pixels[source + 2];
      rgba[target + 3] = 255;
    }
    return rgba;
  }

  async paint(canvas: HTMLCanvasElement, frame: EndingFrame, valid: () => boolean = () => true) {
    const pixels = await this.rgba(frame);
    if (!valid()) return;
    const context = canvas.getContext("2d");
    if (!context || canvas.width !== 320 || canvas.height !== 200)
      throw Error("仙剑结局画布不可用");
    const image = context.createImageData(320, 200);
    image.data.set(pixels);
    context.putImageData(image, 0, 0);
  }
}
