type ImageSource = { bufferView?: number; mimeType?: string; uri?: string };
type Parser = { json: { textures?: { source?: number; sampler?: number; name?: string }[]; images?: ImageSource[]; samplers?: Record<string, number>[] }; getDependency: (kind: string, index: number) => Promise<ArrayBuffer> };
type ImageSize = { width: number; height: number };

export async function decodeEmbeddedTexture<TImage extends ImageSize, TTexture>(parser: Parser, index: number, decode: (blob: Blob) => Promise<TImage>, texture: (image: TImage, sampler: Record<string, number>, name: string) => TTexture): Promise<TTexture> {
  const selected = parser.json.textures?.[index];
  const image = selected?.source === undefined ? undefined : parser.json.images?.[selected.source];
  if (!image || image.bufferView === undefined || image.uri || !['image/png', 'image/jpeg'].includes(image.mimeType ?? '')) throw Error('原版模型缺少内嵌纹理');
  const bytes = await parser.getDependency('bufferView', image.bufferView);
  const decoded = await decode(new Blob([new Uint8Array(bytes)], { type: image.mimeType }));
  if (!decoded.width || !decoded.height) throw Error('原版模型纹理未成功解码');
  return texture(decoded, parser.json.samplers?.[selected!.sampler ?? -1] ?? {}, selected?.name ?? '');
}

type Material = { name?: string; map?: { image?: ImageSize } | null };
export function assertModelTextures(scene: { traverse: (visit: (object: unknown) => void) => void }) {
  let meshes = 0;
  let materials = 0;
  scene.traverse(object => {
    const mesh = object as { isMesh?: boolean; material?: Material | Material[] };
    if (!mesh.isMesh) return;
    meshes++;
    for (const material of Array.isArray(mesh.material) ? mesh.material : [mesh.material]) {
      materials++;
      if (!material?.map?.image?.width || !material.map.image.height) throw Error(`原版模型纹理加载失败：${material?.name ?? '材质'}`);
    }
  });
  if (!meshes) throw Error('原版模型没有可绘制网格');
  return { meshes, materials, texturedMaterials: materials };
}
