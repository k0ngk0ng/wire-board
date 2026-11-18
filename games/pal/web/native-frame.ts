import type { NativeRuntime, NativeSnapshot } from "./native-runtime";
import type { LocalMeta } from "./local-types";
import type { PalView } from "./view";

export function nativePhase(snapshot: NativeSnapshot): PalView["phase"] {
  const v = snapshot.view,
    event = snapshot.event.kind;
  if (v.gameOver?.stage === 2) return "gameover";
  if (v.ending?.finished) return "finished";
  if (v.ending) return "ending";
  if (event === 6) return "cooperation";
  if (event === 2) return "dialogue";
  if (event === 5) return "system";
  if (v.mode === 3) return "menu";
  if (v.mode === 0 && !v.entering && event === 4) return "field";
  if (v.mode === 2) return "battle";
  return "script";
}

export function nativeFrame(runtime: NativeRuntime, meta: LocalMeta): PalView {
  const v = runtime.snapshot.view,
    phase = nativePhase(runtime.snapshot),
    dialogueID = v.messages * 65536 + v.pc + 1;
  const { audio, battle, party, cooperation, ...scene } = v;
  void audio;
  void cooperation;
  return {
    ...scene,
    phase,
    epoch: meta.generation,
    leader: 0,
    seq: meta.seq,
    appliedSeq: meta.appliedSeq,
    controlAt: meta.serverAt,
    discardedSeq: meta.discardedSeq,
    local: {
      ...meta,
      boundary: runtime.inputBoundary(),
      records: undefined,
      checkpoint: undefined,
    },
    party: party.map((role, i) => {
      const player = battle.players[i];
      return {
        role,
        name: runtime.checkpoint.roles[role],
        controller: 0,
        hp: player.hp,
        maxHP: player.maxHP,
        mp: player.mp,
        maxMP: player.maxMP,
      };
    }),
    ...(battle.active ? { battle } : {}),
    ...(phase === "dialogue"
      ? {
          dialogue: {
            id: dialogueID,
            text: v.dialogPage
              .map((id) => runtime.checkpoint.messages[id])
              .join("\n"),
            face: v.face,
            position: v.dialogPosition,
            read: [false],
          },
        }
      : {}),
    ...(v.raster ? { overlay: [], draws: [] } : {}),
  };
}
