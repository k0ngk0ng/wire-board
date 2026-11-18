export type PalAudioTrack = {
  seq: number;
  kind: number;
  id: number;
  loop: boolean;
  startedAt: number;
};

export type PalAudioSound = { seq: number; id: number; startedAt: number };

export type PalAudioState = {
  assetID: string;
  serial: number;
  music?: PalAudioTrack;
  sounds: PalAudioSound[];
};
