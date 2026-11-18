import { effectiveResourceSpecies, pokemonSprites } from "./resource-species";
import { mapArtwork } from "./map-presentation";
import type { Catalog, Route, Sprite, View } from "./types";
import type { NativeProgress, NativeResourceExpectation } from "./native-runtime";

export type Resource = { path: string; bytes: number; sha256: string; mime: string; url?: string };
export type Progress = { loaded: number; total: number; label: string };
const cache = new Map<string, ArrayBuffer>();
const fonts = new Map<string, Promise<FontFace>>();

export class AssetStore {
  readonly images = new Map<string, HTMLImageElement>();
  readonly urls = new Map<string, string>();
  readonly files = new Map<string, Resource>();
  readonly controller = new AbortController();
  private needed = new Set<string>();
  private received = new Map<string, number>();
  private promises = new Map<string, Promise<ArrayBuffer>>();
  private encoded = new Map<string, Promise<string>>();
  private imagePromises = new Map<string, Promise<HTMLImageElement>>();
  private aliases = new Map<string, string>();

  constructor(readonly base: URL, private progress: (value: Progress) => void) {}

  path(value: string): string {
    let clean = value.replace(/^\.\//, "");
    if (clean.startsWith("/pokepath/v1/")) clean = clean.slice(13);
    if (/^https?:/.test(clean)) {
      const mapped = [...this.files].find(([, file]) => file.url === clean)?.[0];
      const url = new URL(clean);
      if (mapped) clean = mapped;
      else if (url.origin === this.base.origin && url.pathname.startsWith(this.base.pathname)) clean = decodeURIComponent(url.pathname.slice(this.base.pathname.length));
      else throw Error("素材地址不属于当前游戏");
    }
    const seen = new Set<string>();
    while (this.aliases.has(clean)) {
      if (seen.has(clean) || seen.size >= 32) throw Error("素材别名存在循环");
      seen.add(clean);
      clean = this.aliases.get(clean)!.replace(/^\.\//, "");
    }
    return clean;
  }

  url(value: string): string {
    const path = this.path(value);
    if (path.split("/").some(part => part === "..") || path.startsWith("/") || path.includes("\\")) throw Error("素材路径无效");
    return this.urls.get(path) ?? this.files.get(path)?.url ?? new URL(path, this.base).href;
  }

  async initialize() {
    const response = await fetch(this.files.get("resources.json")?.url ?? new URL("resources.json", this.base), { signal: this.controller.signal });
    if (!response.ok) throw Error("无法读取巡径守卫素材清单");
    const body = await response.arrayBuffer();
    const metadataHash = [...new Uint8Array(await crypto.subtle.digest("SHA-256", body))].map(byte => byte.toString(16).padStart(2, "0")).join("");
    const expected = this.files.get("resources.json");
    if (expected && (body.byteLength !== expected.bytes || metadataHash !== expected.sha256)) throw Error("当前素材清单校验失败");
    const manifest = JSON.parse(new TextDecoder().decode(body));
    if (!Array.isArray(manifest.files)) throw Error("素材清单格式无效");
    for (const [key, value] of Object.entries(manifest.aliases ?? {})) {
      if (typeof value !== "string" || key.length > 512 || value.length > 512) throw Error("素材别名格式无效");
      this.aliases.set(key, value);
    }
    for (const file of manifest.files as Resource[]) if (!this.files.has(file.path)) this.files.set(file.path, file);
    this.files.set("resources.json", { path: "resources.json", bytes: body.byteLength, sha256: metadataHash, mime: "application/json", url: expected?.url });
    this.promises.set("resources.json", Promise.resolve(body));
    this.received.set("resources.json", body.byteLength);
    this.needed.add("resources.json");
  }

  require(paths: readonly string[]) {
    for (const path of paths) if (path) this.needed.add(this.path(path));
    this.notify();
  }

  private notify() {
    const total = [...this.needed].reduce((sum, path) => sum + (this.files.get(path)?.bytes ?? this.received.get(path) ?? 0), 0);
    const loaded = [...this.needed].reduce((sum, path) => sum + (this.received.get(path) ?? 0), 0);
    this.progress({ loaded, total, label: "读取当前路线与伙伴素材" });
  }

  async readExternal(logical: string, expected?: NativeResourceExpectation): Promise<ArrayBuffer | undefined> {
    const path = this.path(logical);
    if (expected) this.files.set(path, { ...this.files.get(path), ...expected, path });
    if (!this.files.get(path)?.sha256) return undefined;
    return this.read(path);
  }

  external(progress: NativeProgress) {
    for (const file of progress.resources ?? []) {
      const path = this.path(file.path);
      if (!this.files.has(path)) this.files.set(path, { path, bytes: file.bytes, sha256: "", mime: "application/octet-stream" });
      this.needed.add(path);
      if (progress.cached) this.received.set(path, file.bytes);
    }
    if (progress.path && progress.fileTotal !== undefined) {
      const path = this.path(progress.path);
      if (!this.files.has(path)) this.files.set(path, { path, bytes: progress.fileTotal, sha256: "", mime: "application/octet-stream" });
      this.needed.add(path);
      this.received.set(path, Math.max(this.received.get(path) ?? 0, progress.fileLoaded ?? 0));
    }
    this.notify();
  }

  async read(value: string): Promise<ArrayBuffer> {
    const path = this.path(value);
    if (this.promises.has(path)) return this.promises.get(path)!;
    this.require([path]);
    const task = (async () => {
      const file = this.files.get(path);
      if (!file || !Number.isSafeInteger(file.bytes) || !/^[a-f0-9]{64}$/.test(file.sha256)) throw Error(`缺少素材校验信息：${path}`);
      const key = `${this.url(path)}:${file.sha256}`;
      const retained = cache.get(key);
      if (retained) {
        this.received.set(path, retained.byteLength);
        this.notify();
        return retained;
      }
      const response = await fetch(this.url(path), { signal: this.controller.signal });
      if (!response.ok) throw Error(`素材读取失败：${path}`);
      const reader = response.body?.getReader();
      const chunks: Uint8Array[] = [];
      let size = 0;
      if (reader) {
        while (true) {
          const next = await reader.read();
          if (next.done) break;
          size += next.value.byteLength;
          if (size > file.bytes) throw Error(`素材大小不符：${path}`);
          chunks.push(next.value);
          this.received.set(path, size);
          this.notify();
        }
      } else {
        const bytes = new Uint8Array(await response.arrayBuffer());
        size = bytes.byteLength;
        chunks.push(bytes);
      }
      const bytes = new Uint8Array(size);
      let offset = 0;
      for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
      const hash = [...new Uint8Array(await crypto.subtle.digest("SHA-256", bytes))].map(byte => byte.toString(16).padStart(2, "0")).join("");
      if (size !== file.bytes || hash !== file.sha256) throw Error(`素材完整性校验失败：${path}`);
      this.received.set(path, size);
      if (cache.size > 256) cache.delete(cache.keys().next().value!);
      cache.set(key, bytes.buffer);
      this.notify();
      return bytes.buffer;
    })();
    this.promises.set(path, task);
    return task;
  }

  async json<T>(path: string): Promise<T> { return JSON.parse(new TextDecoder().decode(await this.read(path))) as T; }

  private dataURL(value: string): Promise<string> {
    const path = this.path(value);
    let task = this.encoded.get(path);
    if (!task) {
      task = this.read(path).then(bytes => new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => typeof reader.result === "string" ? resolve(reader.result) : reject(Error(`素材编码失败：${path}`));
        reader.onerror = () => reject(Error(`素材编码失败：${path}`));
        reader.readAsDataURL(new Blob([bytes], { type: this.files.get(path)?.mime ?? "application/octet-stream" }));
      })).then(url => { this.urls.set(path, url); return url; });
      this.encoded.set(path, task);
    }
    return task;
  }

  async media(value: string): Promise<string> {
    const path = this.path(value);
    if (!this.files.get(path)?.mime.startsWith("audio/")) throw Error(`素材不是音频：${path}`);
    return this.dataURL(path);
  }

  async image(value: string): Promise<HTMLImageElement> {
    const path = this.path(value);
    if (this.imagePromises.has(path)) return this.imagePromises.get(path)!;
    const task = (async () => {
      const url = await this.dataURL(path);
      const image = new Image();
      image.src = url;
      try { await image.decode(); }
      catch { throw Error(`素材无法解码：${path}（${this.files.get(path)?.mime ?? "image/png"}）`); }
      this.images.set(path, image);
      this.urls.set(path, url);
      return image;
    })();
    this.imagePromises.set(path, task);
    return task;
  }

  async font(name = "PressStart2P") {
    const path = `src/assets/fonts/${name}.ttf`;
    if (!this.files.has(path)) throw Error("所选字体尚不可用");
    this.require([path]);
    const bytes = await this.read(path);
    if (!fonts.has(name)) fonts.set(name, (async () => {
      const sizeAdjust = name === "Unown" ? "135%" : ["NGC", "Lexend", "PixelMPlus"].includes(name) ? "120%" : "100%";
      const descriptors = { style: "normal", ...(sizeAdjust === "100%" ? {} : { sizeAdjust }) };
      const face = new FontFace(name, bytes, descriptors);
      await face.load();
      document.fonts.add(face);
      return face;
    })());
    await fonts.get(name);
  }

  async prepare(view: View, catalog: Catalog, route: Route) {
    const paths = new Set<string>([mapArtwork(view, route) ?? route.background, "src/assets/images/textures/texture1.png", `src/assets/images/portraits/${view.progression?.portrait ?? 0}.png`, ...["heart", "no-heart", "ribbon1", "pokeball-close", "pokeball-open", "profile", "box", "inventory", "shop", "map", "challenge", "dex", "damageDealt", "menu", "curse"].map(name => `src/assets/images/icons/${name}.png`)]);
    if (route.effect) paths.add(route.effect);
    const sprites: Sprite[] = [...view.team.flatMap(pokemon => pokemonSprites(pokemon, catalog, view.progression?.dna)), ...view.enemies.map(enemy => enemy.sprite ?? catalog.enemies[enemy.key]?.sprite), ...view.projectiles.map(projectile => projectile.sprite), ...(view.attackModes?.effects?.map(effect => effect.sprite) ?? [])].filter((sprite): sprite is Sprite => Boolean(sprite));
    for (const piece of route.chessPlacements ?? []) if (catalog.species[piece.species]?.sprite) sprites.push(catalog.species[piece.species].sprite);
    for (const key of route.waves[String(view.wave)]?.preview ?? []) if (catalog.enemies[key]?.sprite) sprites.push(catalog.enemies[key].sprite);
    for (const sprite of sprites) for (const path of [sprite.image, sprite.base]) if (path) paths.add(path);
    this.require([...paths]);
    await Promise.all([...paths].map(path => this.image(path)));
    await this.font(view.progression?.settings.font ?? "PressStart2P");
    const sounds = new Set(["src/assets/sounds/ui/click1.wav"]);
    for (const pokemon of view.team) {
      const key = effectiveResourceSpecies(pokemon, catalog, view.progression?.dna)?.facts?.projectileSound;
      if (typeof key === "string" && /^[a-zA-Z][A-Za-z0-9_-]*$/.test(key)) sounds.add(`src/assets/sounds/effect/${key}.wav`);
    }
    await Promise.all([...sounds].filter(path => this.files.has(this.path(path))).map(path => this.media(path)));
  }

  dispose() {
    this.controller.abort();
    this.images.clear();
    this.urls.clear();
  }
}
