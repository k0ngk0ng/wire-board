import "./game-mobile-play-notice.css";

export function GameMobilePlayNotice({ enabled }: { enabled?: boolean }) {
  if (!enabled) return null;
  return (
    <aside
      className="game-mobile-play-notice"
      data-mobile-play-notice
      role="note"
    >
      建议使用电脑游玩。手机可能出现卡顿，组合按键需要多指操作。
    </aside>
  );
}
