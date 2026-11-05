export type GameAudioPCMStreamConfig = {
  playbackID: string;
  sampleRate: 44100;
  channels: 2;
  format: "s16le";
  maxAheadSamples?: number; // Default and upper bound2048; may be tightened.
};

export type GameAudioPCMStreamPacket = {
  sequence: number; // uint32, starts1.
  startSample: number; // Safe sample-frame cursor, starts0.
  sampleFrames: number; // 1..1024; bytes must contain exactly4bytes/frame.
  bytes: Uint8Array;
};

export type GameAudioPCMStreamPhase =
  | "opening"
  | "ready"
  | "feeding"
  | "paused"
  | "ending-input"
  | "input-ended"
  | "stopped"
  | "failed";

export type GameAudioPCMStreamStatus = {
  phase: GameAudioPCMStreamPhase;
  playbackID: string;
  sampleRate: 44100;
  channels: 2;
  format: "s16le";
  maxAheadSamples: number;
  maxLateSamples: number; // Fixed4410 in v1, independent of native currentTime.
  acceptedEndSample: number;
  nextSeq: number;
  pendingSequence: number | null;
  pendingBytes: number;
  sourceClockSamples: number;
  sourceLateSamples: number;
  nativePlaying: boolean;
  nativeTrackState: "live" | "ended" | null;
  nativeCurrentTimeSeconds: number | null;
  nativePlayedSamples: null;
  nativeDroppedSamples: null;
  lastRejection?: string;
  error?: string;
};

export type GameAudioPCMStreamHandle = {
  // Track/writer/element established; never waits for media play or a packet.
  readonly ready: Promise<boolean>;
  // Resource cleanup completed. Does not mean the speaker has drained.
  readonly released: Promise<void>;
  append(packet: GameAudioPCMStreamPacket): Promise<boolean>;
  // Locks input, waits one pending packet and its nominal source-end time,
  // then closes the writer. Host must explicitly stop to release resources.
  endInput(): Promise<boolean>;
  status(): GameAudioPCMStreamStatus;
  stop(): void;
};
