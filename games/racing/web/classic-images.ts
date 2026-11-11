import type { Texture } from "three";
import type { GLTFLoaderPlugin, GLTFParser } from "three/examples/jsm/loaders/GLTFLoader.js";

type Options = {
  decode?: (source: Blob, options: ImageBitmapOptions) => Promise<ImageBitmap>;
  own?: (texture: Texture) => void;
  decoded?: () => void;
};

/** Decode verified GLB bufferViews without object URLs or URL-based loaders. */
export function classicInlineImagePlugin(
  parser: GLTFParser,
  createTexture: (image: ImageBitmap) => Texture,
  options: Options = {},
): GLTFLoaderPlugin {
  const sources = new Map<number, Promise<Texture>>();
  const decode = options.decode ?? ((blob, settings) => createImageBitmap(blob, settings));
  const own = (texture: Texture) => {
    options.own?.(texture);
    return texture;
  };
  parser.loadImageSource = (sourceIndex) => {
    const known = sources.get(sourceIndex);
    if (known) return known.then((texture) => own(texture.clone()));
    const pending = (async () => {
      const source = parser.json.images?.[sourceIndex];
      if (!source || source.uri !== undefined || !Number.isSafeInteger(source.bufferView) || source.bufferView < 0)
        throw Error("赛车模型只允许已校验的内嵌贴图");
      if (!["image/png", "image/webp"].includes(source.mimeType))
        throw Error("赛车内嵌贴图格式无效");
      const bytes = await parser.getDependency("bufferView", source.bufferView) as ArrayBuffer;
      if (!(bytes instanceof ArrayBuffer) || !bytes.byteLength)
        throw Error("赛车内嵌贴图数据为空");
      const image = await decode(new Blob([bytes], { type: source.mimeType }), {
        imageOrientation: "none",
        premultiplyAlpha: "none",
        colorSpaceConversion: "none",
      });
      const dimensions = source.extras?.sourceDimensions;
      if (!image.width || !image.height || (Array.isArray(dimensions) &&
          (dimensions[0] !== image.width || dimensions[1] !== image.height))) {
        image.close();
        throw Error("赛车原贴图尺寸不匹配");
      }
      const texture = createTexture(image);
      texture.needsUpdate = true;
      Object.assign(texture.userData, source.extras ?? {}, { mimeType: source.mimeType });
      options.decoded?.();
      return own(texture);
    })();
    sources.set(sourceIndex, pending);
    return pending;
  };
  return {
    // Replace only this parser's WebP plugin. Its default support test creates
    // a data URI; decoding the real payload already proves browser support.
    name: "EXT_texture_webp",
    loadTexture(textureIndex) {
      const source = parser.json.textures?.[textureIndex]?.extensions?.EXT_texture_webp?.source;
      return source === undefined ? null : parser.loadTextureImage(textureIndex, source, parser.textureLoader);
    },
    async beforeRoot() {
      const images = parser.json.images;
      if (!Array.isArray(images) || !images.length) throw Error("赛车模型缺少原始贴图");
      // GLTFParser.loadTextureImage catches image errors and returns null.
      // Predecode makes failures reject parseAsync before that permissive path.
      await Promise.all(images.map((_, index) => parser.loadImageSource(index, parser.textureLoader)));
    },
    async afterRoot() {
      const textures = await parser.getDependencies("texture") as Array<Texture | null>;
      if (!textures.length || textures.length !== parser.json.textures?.length ||
          textures.some((texture) => !texture || !texture.image?.width || !texture.image?.height))
        throw Error("赛车原始贴图未完整解码，不能进入对局");
    },
  };
}
