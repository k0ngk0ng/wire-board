import { useEffect, useRef, type ReactNode } from "react";
import { gameModule } from "../game-api/registry";
import { gameDefaultPlayers } from "../game-api/default-players";
import { GameChoices } from "./game-choices";
import { Modal } from "./board-host";

export type RoomDraft = { name: string; password: string };
export type GameSetupFormProps = {
  kind: string;
  busy: boolean;
  draft: RoomDraft;
  onDraftChange: (patch: Partial<RoomDraft>) => void;
  onSubmit: (body: unknown) => void;
};

export function CreateRoom({
  availableGames,
  busy,
  onClose,
  onSubmit,
  renderCover,
}: {
  availableGames: string[];
  busy: boolean;
  onClose: () => void;
  onSubmit: (body: unknown) => void;
  renderCover: (kind: string) => ReactNode;
}) {
  const choices = useRef<HTMLDivElement>(null);
  const pending = useRef(false);
  useEffect(() => {
    if (!busy) pending.current = false;
  }, [busy]);
  useEffect(() => {
    const mobile = window.matchMedia(
      "(max-width: 600px), (pointer: coarse)",
    ).matches;
    const target = mobile
      ? choices.current
      : choices.current?.querySelector<HTMLInputElement>(
          'input[type="search"]',
        );
    target?.focus({ preventScroll: true });
  }, []);
  return (
    <Modal
      title="选择游戏"
      onClose={onClose}
      className="create-room-modal create-room-choosing"
    >
      <div
        ref={choices}
        tabIndex={-1}
        role="group"
        aria-label="选择游戏"
        data-create-step="choose"
      >
        <GameChoices
          games={availableGames}
          value=""
          disabled={busy}
          renderCover={renderCover}
          onChange={(kind) => {
            const game = gameModule(kind);
            if (
              busy ||
              pending.current ||
              !game ||
              !availableGames.includes(kind)
            )
              return;
            pending.current = true;
            const capacity = gameDefaultPlayers(game);
            onSubmit({
              kind,
              capacity,
              ...(game.legacy
                ? {
                    railMap: kind === "rail" ? "usa" : undefined,
                    catanOptions:
                      kind === "catan"
                        ? {
                            fiveSix:
                              Number.isSafeInteger(capacity) && capacity > 4,
                          }
                        : undefined,
                  }
                : { options: {} }),
            });
          }}
        />
      </div>
    </Modal>
  );
}
