import { dropY, fits, fullBoard, tryMove, tryRotate } from "./shapes.ts";
import type { InputEvent } from "./input";
import type { Kind, PieceView } from "./types";

export type PredictedPiece = {
  id: number;
  kind: Kind;
  rot: number;
  x: number;
  y: number;
};

// Display-only prediction: movement and drop position of the viewer's current
// piece. It never mutates the authoritative board, clears lines, spawns pieces,
// applies garbage or produces a score/result. A hold or hard drop ends the
// prediction window until the next server snapshot.
export function predict(
  board: string[],
  active: PieceView | undefined,
  events: InputEvent[],
): PredictedPiece | undefined {
  if (!active) return undefined;
  const full = fullBoard(board);
  const piece: PredictedPiece = {
    id: active.id,
    kind: active.kind,
    rot: active.rot,
    x: active.x,
    y: active.y,
  };
  for (const event of events) {
    switch (event.kind) {
      case "left":
        tryMove(full, piece, -1);
        break;
      case "right":
        tryMove(full, piece, 1);
        break;
      case "rotateCW":
        tryRotate(full, piece, 1);
        break;
      case "rotateCCW":
        tryRotate(full, piece, -1);
        break;
      case "softDrop":
        if (fits(full, piece.kind, piece.rot, piece.x, piece.y + 1))
          piece.y++;
        break;
      case "hardDrop":
        piece.y = dropY(
          full,
          piece.kind,
          piece.rot,
          piece.x,
          piece.y,
        );
        return piece;
      case "hold":
        return piece;
    }
  }
  return piece;
}

export function ghostY(
  board: string[],
  piece: { kind: Kind; rot: number; x: number; y: number },
): number {
  return dropY(fullBoard(board), piece.kind, piece.rot, piece.x, piece.y);
}
