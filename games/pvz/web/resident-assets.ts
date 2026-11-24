import type { ArtManifest, SpriteDefinition } from "./art";
import type { ResidentResource } from "../../../web/src/game-api/resident-assets";

export function pvzResidentResources(scene: string, manifest: ArtManifest, media: Readonly<Record<string, number>>): ResidentResource[] {
  if (!manifest.maps?.[scene] || !manifest.byUnit || !manifest.music?.[scene]) throw Error("场景资源清单不完整");
  const paths = new Set<string>(["manifest.json", manifest.maps[scene].background, ...(manifest.byScene?.[scene] ?? []), ...Object.values(manifest.images ?? {}), ...Object.values(manifest.byUnit).flat(), ...Object.values(manifest.audio ?? {}).flat(), ...manifest.music[scene].tracks.map(track => track.src)]);
  const sprite = (definition: SpriteDefinition) => {
    paths.add(definition.src);
    if (definition.native) { paths.add(definition.native.src); for (const image of (definition.native.images ?? [])) paths.add(image); }
    if (definition.preview) sprite(definition.preview);
    for (const animation of Object.values(definition.animations ?? {})) sprite(animation);
  };
  for (const definition of Object.values(manifest.projectiles ?? {})) sprite(definition);
  const files = [...paths].filter(Boolean).map(path => "/pvz/classic-v2/" + path);
  files.push(...Object.keys(media).filter(path => path.startsWith("/pvz/v1/runtime/")));
  return [...new Set(files)].map(url => {
    const bytes = media[url];
    if (!Number.isSafeInteger(bytes) || bytes <= 0) throw Error("场景缺少必要资源");
    return { url, bytes };
  });
}
