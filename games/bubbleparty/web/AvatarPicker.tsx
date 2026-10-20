import { useEffect, useRef } from "react";
import { Modal } from "../../../web/src/ui/board-host";
import {
  avatarCatalog,
  drawAvatarArt,
  drawAvatarFallback,
  loadAvatar,
} from "./avatar-art";

function Preview({
  avatar,
  team,
  assets,
}: {
  avatar: number;
  team: number;
  assets: string;
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    void loadAvatar(assets, avatar, team);
    let frame = 0;
    const render = () => {
      const ctx = ref.current?.getContext("2d");
      if (ctx) {
        ctx.clearRect(0, 0, 112, 116);
        ctx.imageSmoothingEnabled = false;
        if (
          !drawAvatarArt(
            ctx,
            assets,
            {
              avatar,
              team,
              dir: "down",
              moving: false,
              trapped: false,
              dead: false,
              deathAge: 0,
              now: performance.now(),
              reduced: false,
            },
            56,
            100,
            1.35,
          )
        )
          drawAvatarFallback(ctx, avatar, team, 56, 100, 2);
      }
      frame = requestAnimationFrame(render);
    };
    render();
    return () => cancelAnimationFrame(frame);
  }, [avatar, team, assets]);
  return <canvas ref={ref} width={112} height={116} aria-hidden="true" />;
}
export function AvatarPicker({
  team,
  selected,
  assets,
  saving,
  error,
  onChoose,
  onClose,
}: {
  team: number;
  selected: number;
  assets: string;
  saving: boolean;
  error: string;
  onChoose: (id: number) => void;
  onClose: () => void;
}) {
  return (
    <Modal title="选择人物" onClose={onClose} className="bubble-avatar-modal">
      <p className={`bubble-avatar-note team-${team}`}>
        {team === 0 ? "红队" : "蓝队"}
        的基础色保持不变，造型不影响能力。对局会继续进行。
      </p>
      <div className="bubble-avatar-grid" role="group" aria-label="人物造型">
        {avatarCatalog.avatars.map((a) => (
          <button
            type="button"
            key={a.id}
            data-bubbleparty-avatar={a.id}
            aria-pressed={a.id === selected}
            disabled={saving || a.id === selected}
            onClick={() => onChoose(a.id)}
          >
            <Preview avatar={a.id} team={team} assets={assets} />
            <strong>{a.name}</strong>
            <span>{a.id === selected ? "已选择" : "选择"}</span>
          </button>
        ))}
      </div>
      {error && (
        <p className="bubble-avatar-error" role="alert">
          {error}
        </p>
      )}
    </Modal>
  );
}
