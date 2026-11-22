export type HudSprite = {
  x: number;
  y: number;
  width: number;
  height: number;
  sourceSprite: string;
  sourceRect: readonly [number, number, number, number];
  sourceSHA256: string;
  blend: 'opaque' | 'additive' | 'alpha' | 'alpha-test';
};

export type HudAssets = {
  schema: 1;
  atlas: { file: string; bytes: number; sha256: string; mime: string };
  width: number;
  height: number;
  defaultResolution: 640;
  sprites: Record<string, Record<string, HudSprite>>;
  weapons: Record<string, Record<string, string>>;
  killIcons: Record<string, string>;
  source: {
    repository: string;
    commit: string;
    catalogSHA256: string;
    hudTextSHA256: string;
    declaredDefinitions: number;
    actualDefinitions: number;
    spriteBanks: number;
    skippedDefinitions: { name: string; resolution: number; sourceSprite: string; sourceRect: readonly number[]; reason: string }[];
  };
};
