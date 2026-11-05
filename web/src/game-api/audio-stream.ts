// Public contract only. The platform owns HTMLAudio/MSE, buffering and cleanup;
// games supply trusted encoder output and never construct their own player.
export type GameAudioStreamConfig = {
  playbackID: string;
  mimeType: string;
  sampleRate: number; // Currently validated as44100; counters are sample frames.
  initSegment: Uint8Array;
  maxForwardSamples?: number; // Default66150; positive and at most441000.
  retainBackSamples?: number; // Default88200; inclusive11025..441000.
};

export type GameAudioStreamSegment = {
  sequence: number; // uint32, starts1 and advances only after accepted updateend.
  startSample: number; // Safe integer, starts0 and equals acceptedEndSample.
  sampleFrames: number; // Positive safe integer; end addition must stay safe.
  data: Uint8Array; // Copied before async use; at most1MiB.
};

export type GameAudioStreamPhase =
  | "opening"
  | "ready"
  | "playing"
  | "paused"
  | "draining"
  | "ended"
  | "stopped"
  | "failed";

export type GameAudioStreamStatus = {
  phase: GameAudioStreamPhase;
  playbackID: string;
  sampleRate: number;
  maxForwardSamples: number;
  retainBackSamples: number;
  nextSeq: number;
  acceptedEndSample: number;
  updating: boolean;
  pendingBytes: number;
  currentTime: number;
  buffered: readonly (readonly [number, number])[];
  error?: string;
};

export type GameAudioStreamHandle = {
  // True after sourceopen + SourceBuffer creation + init updateend. Does not
  // wait for native play(), which may require media segments still to arrive.
  readonly ready: Promise<boolean>;
  // One in-flight segment. An identical pending call returns the same Promise;
  // busy/backpressure/invalid calls return false without reserving a sequence.
  // True commits after updateend; it does not mean the samples are audible yet.
  append(segment: GameAudioStreamSegment): Promise<boolean>;
  // Locks new appends, waits at most the current append, then commits EOS.
  // True is not playback completion; the accepted tail must still drain.
  end(): Promise<boolean>;
  status(): GameAudioStreamStatus;
  // Bound to this handle's epoch. An old handle cannot stop its successor.
  stop(): void;
};
