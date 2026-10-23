import { useEffect, useRef, useState, type ReactNode } from "react";
import { ArrowLeft } from "lucide-react";
import { gameModule, gameName } from "../game-api/registry";
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
  renderForm,
}: {
  availableGames: string[];
  busy: boolean;
  onClose: () => void;
  onSubmit: (body: unknown) => void;
  renderCover: (kind: string) => ReactNode;
  renderForm: (props: GameSetupFormProps) => ReactNode;
}) {
  const [kind, setKind] = useState("");
  const [choosing, setChoosing] = useState(true);
  const [draft, setDraft] = useState<RoomDraft>({
    name: "朋友们的牌桌",
    password: "",
  });
  const choices = useRef<HTMLDivElement>(null);
  const settings = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (kind && !availableGames.includes(kind)) {
      setKind("");
      setChoosing(true);
    }
  }, [kind, availableGames]);
  useEffect(() => {
    const panel = choosing ? choices.current : settings.current;
    panel?.closest(".modal")?.scrollTo({ top: 0 });
    // Keep focus inside the active step without summoning a mobile keyboard.
    const mobile = window.matchMedia(
      "(max-width: 600px), (pointer: coarse)",
    ).matches;
    const target = mobile
      ? panel
      : panel?.querySelector<HTMLInputElement>(
          choosing ? 'input[type="search"]' : 'input[name="name"]',
        );
    target?.focus({ preventScroll: true });
  }, [choosing, kind]);
  const game = gameModule(kind);
  return (
    <Modal
      title={choosing ? "选择游戏" : `${gameName(kind)} · 创建房间`}
      onClose={onClose}
      className={`create-room-modal${choosing ? " create-room-choosing" : ""}`}
    >
      <div
        ref={choices}
        tabIndex={-1}
        role="group"
        aria-label="选择游戏"
        hidden={!choosing}
        inert={!choosing}
        data-create-step="choose"
      >
        <GameChoices
          games={availableGames}
          value={kind}
          disabled={busy}
          renderCover={renderCover}
          onChange={(next) => {
            setKind(next);
            setChoosing(false);
          }}
        />
      </div>
      {kind && game && (
        <div
          key={kind}
          ref={settings}
          tabIndex={-1}
          role="group"
          aria-label="房间设置"
          hidden={choosing}
          inert={choosing}
          data-create-step="setup"
        >
          <button
            type="button"
            className="subtle create-back"
            disabled={busy}
            onClick={() => setChoosing(true)}
          >
            <ArrowLeft size={16} />
            返回选择游戏
          </button>
          <div className="create-selected-game">
            <div className="create-selected-cover">{renderCover(kind)}</div>
            <div>
              <strong>{game.name}</strong>
              <span>
                {game.playerCounts
                  ? game.playerCounts.join(" / ")
                  : game.minPlayers === game.maxPlayers
                    ? game.minPlayers
                    : `${game.minPlayers}～${game.maxPlayers}`}{" "}
                人
              </span>
              <p>{game.description}</p>
            </div>
          </div>
          {renderForm({
            kind,
            busy: busy || choosing || !availableGames.includes(kind),
            draft,
            onDraftChange: (patch) => setDraft((old) => ({ ...old, ...patch })),
            onSubmit: (body) => {
              if (!choosing && availableGames.includes(kind)) onSubmit(body);
            },
          })}
        </div>
      )}
    </Modal>
  );
}
