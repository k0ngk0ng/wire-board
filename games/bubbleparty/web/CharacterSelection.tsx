import { useEffect, useRef, useState } from "react";
import { Modal } from "../../../web/src/ui/board-host";
import {
  characters,
  characterName,
  normalizeCharacter,
  selectableCharacters,
} from "./character-definitions";
import {
  drawCharacterArt,
  drawCharacterFallback,
  loadCharacter,
} from "./character-art";
import "./characters.css";

function Portrait({
  id,
  team,
  assets,
}: {
  id: number;
  team: number;
  assets: string;
}) {
  const canvas = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    let cancelled = false;
    const draw = () => {
      if (cancelled) return;
      const ctx = canvas.current?.getContext("2d");
      if (!ctx) return;
      ctx.clearRect(0, 0, 112, 116);
      ctx.imageSmoothingEnabled = false;
      if (
        !drawCharacterArt(
          ctx,
          assets,
          {
            character: id,
            team,
            dir: "down",
            moving: false,
            trapped: false,
            dead: false,
            deathAge: 0,
            now: 0,
            reduced: true,
          },
          56,
          106,
          1.35,
        )
      )
        drawCharacterFallback(ctx, id, team, 56, 106, 2);
    };
    draw();
    void loadCharacter(assets, id, team).then(draw);
    return () => {
      cancelled = true;
    };
  }, [id, team, assets]);
  return <canvas ref={canvas} width={112} height={116} aria-hidden="true" />;
}

function Attributes({ id }: { id: number }) {
  const c = characters[normalizeCharacter(id)];
  return (
    <dl className="bubble-character-stats">
      <div>
        <dt>速度</dt>
        <dd>
          {c.initialSpeed}
          <span> / {c.maximumSpeed}</span>
        </dd>
      </div>
      <div>
        <dt>水泡</dt>
        <dd>
          {c.initialBombs}
          <span> / {c.maximumBombs}</span>
        </dd>
      </div>
      <div>
        <dt>水柱</dt>
        <dd>
          {c.initialRange}
          <span> / {c.maximumRange}</span>
        </dd>
      </div>
    </dl>
  );
}

export function CharacterSelection({
  epoch,
  phase,
  character,
  confirmed,
  team,
  assets,
  selectUntil,
  now,
  selectedCount,
  playerCount,
  canChoose,
  open,
  onOpenChange,
  onChoose,
}: {
  epoch: string;
  phase: string;
  character: number;
  confirmed: boolean;
  team: number;
  assets: string;
  selectUntil: number;
  now: number;
  selectedCount: number;
  playerCount: number;
  canChoose: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onChoose: (character: number, confirm: boolean) => Promise<unknown>;
}) {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const opened = useRef("");
  const currentEpoch = useRef(epoch);
  currentEpoch.current = epoch;
  const selecting = phase === "select";
  const remaining = Math.max(0, Math.ceil((selectUntil - now) / 1000));
  useEffect(() => {
    if (selecting && canChoose && !confirmed && opened.current !== epoch) {
      opened.current = epoch;
      onOpenChange(true);
    }
    if (!selecting) onOpenChange(false);
  }, [epoch, selecting, canChoose, confirmed, onOpenChange]);
  const choose = async (id: number, confirm: boolean) => {
    if (!selecting || !canChoose || saving) return;
    setError("");
    setSaving(true);
    try {
      await onChoose(id, confirm);
      if (currentEpoch.current === epoch && confirm) onOpenChange(false);
    } catch (e) {
      if (currentEpoch.current === epoch)
        setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };
  return (
    <section
      className={`bubble-character-selection team-${team}`}
      data-bubbleparty-character-phase={phase}
    >
      <div className="bubble-character-selection-copy">
        <strong>
          {selecting ? "开战前选择角色" : `角色 · ${characterName(character)}`}
        </strong>
        {selecting && (
          <span>
            {confirmed
              ? "你已确认"
              : canChoose
                ? "各角色的初始能力与成长上限不同"
                : "等待玩家选择角色"}{" "}
            · {selectedCount}/{playerCount} 人已确认
          </span>
        )}
        {selecting && (
          <span role="timer">
            选角剩余 {remaining} 秒 · 全员确认后提前开战，到期自动开战
          </span>
        )}
      </div>
      <button
        type="button"
        className="bubble-character-open"
        data-bubbleparty-character-open
        onClick={() => onOpenChange(true)}
      >
        {selecting && canChoose
          ? confirmed
            ? "更换角色"
            : "选择角色"
          : "角色资料"}
      </button>
      {open && (
        <Modal
          title={selecting && canChoose ? "选择角色" : "角色资料"}
          onClose={() => onOpenChange(false)}
          className="bubble-character-modal"
        >
          <p className="bubble-character-note">
            {team === 0 ? "红队" : "蓝队"}基础色保持不变。能力按「初始 /
            上限」显示；吃到道具后提升。
          </p>
          {selecting && (
            <p className="bubble-character-deadline" role="status">
              选角剩余 {remaining} 秒，到期使用当前角色自动开战。
            </p>
          )}
          {selecting && canChoose ? (
            <>
              <div
                className="bubble-character-grid"
                role="group"
                aria-label="角色及初始能力"
              >
                {selectableCharacters.map((c) => (
                  <button
                    type="button"
                    key={c.id}
                    data-bubbleparty-character={c.id}
                    aria-pressed={c.id === character}
                    disabled={saving}
                    onClick={() => void choose(c.id, false)}
                  >
                    <Portrait id={c.id} team={team} assets={assets} />
                    <strong>{c.name}</strong>
                    <Attributes id={c.id} />
                    <span className="bubble-character-selected">
                      {c.id === character ? "当前角色" : "选择此角色"}
                    </span>
                  </button>
                ))}
              </div>
              <div className="bubble-character-confirm-row">
                <button
                  type="button"
                  className="bubble-character-confirm"
                  data-bubbleparty-character-confirm
                  disabled={saving || confirmed}
                  onClick={() => void choose(character, true)}
                >
                  {saving
                    ? "正在保存…"
                    : confirmed
                      ? "已确认，等待其他玩家"
                      : `确认使用 · ${characterName(character)}`}
                </button>
              </div>
            </>
          ) : (
            <div className="bubble-character-details">
              <Portrait id={character} team={team} assets={assets} />
              <strong>{characterName(character)}</strong>
              <Attributes id={character} />
              <p>比赛开始后不能更换角色。</p>
            </div>
          )}
          {error && (
            <p role="alert" className="bubble-character-error">
              {error}
            </p>
          )}
        </Modal>
      )}
    </section>
  );
}
