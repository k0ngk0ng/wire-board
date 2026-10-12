// Copied into a temporary game directory only by the browser acceptance runner.
import { defineGame } from "../../../web/src/game-api/module";

type View = { count: number; secret: string };
export default defineGame<View, { mode: string }>({
  id: "acceptance", name: "接入验收", description: "独立游戏模块的真实浏览器验收。",
  minPlayers: 2, maxPlayers: 2, supportsBots: true, defaultOptions: { mode: "normal" },
  Options: ({ value, onChange, disabled }) => <label>验收模式<select disabled={disabled} value={value.mode} onChange={e => onChange({ mode: e.target.value })}><option value="normal">标准</option></select></label>,
  Board: ({ data, room, act, busy }) => <section className="acceptance-board" style={{ padding: 24 }}>
    <h2>独立模块计数：{data.count}</h2><p data-testid="private-card">{data.secret}</p>
    <button className="primary" disabled={busy || room.spectating || room.game?.finished || !room.game?.actors?.includes(room.you)} onClick={() => void act({ increment: 1 })}>增加一次</button>
  </section>,
  Rules: () => <p>双方交替操作，计数达到 4 后结束。</p>,
});
