import { useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { ModuleWaiting, ModulePlayers } from "../web/src/game-api/host";
import { LegacyWaiting, LegacyPlayers } from "../web/src/legacy/room-ui";
import { loadGame } from "../web/src/game-api/registry";
import type { Room, Seat } from "../web/src/types";
import "../web/src/style.css";
import "../web/src/autoplay.css";

// Production waiting/player components with a local command sink. No engine
// rules or server room state are replaced by this presentation-only fixture.
const longNames = [
  "ComputerOpponentWithAnExtremelyLongUnbrokenName12345",
  "用于验证窄屏布局的超长电脑玩家名称二号",
];
function App() {
  const nextBot = useRef(1);
  const [mode, setMode] = useState("module-waiting");
  const [long, setLong] = useState(false);
  const [seats, setSeats] = useState<Seat[]>([
    { id: "human", name: "jason", ready: true, left: false },
  ]);
  const users = seats.map((seat, i) => ({
    ...seat,
    name: long
      ? i === 0
        ? "HumanWithALongUnbrokenName123456"
        : longNames[(i - 1) % 2]
      : seat.name,
  }));
  const legacy = mode.startsWith("legacy"),
    playing = mode.endsWith("playing");
  const room = {
    id: "fixture-room",
    name: "电脑名称布局验收",
    kind: legacy ? "carcassonne" : "monopoly",
    host: "human",
    capacity: 4,
    seats: users,
    status: playing ? "playing" : "waiting",
    locked: false,
    version: 1,
    you: 0,
    updated: 0,
    options: { auctionSeconds: 20, turnSeconds: 120, tradeSeconds: 90 },
    game: {
      turn: 0,
      finished: false,
      actors: [0],
      attention: [0],
      carcassonne: {
        players: users.map((_, i) => ({
          meeples: 7,
          score: i === 1 ? 0 : 12 + i,
        })),
      },
      data: {
        players: users.map((_, i) => ({
          cash: i === 1 ? 0 : 1500,
          position: 0,
        })),
        properties: [],
        board: [{ name: "起点" }],
      },
    },
  } as unknown as Room;
  const command = (type: string, extra?: Record<string, unknown>) => {
    if (type === "add_bot") {
      const number = nextBot.current++;
      setSeats((old) => [
        ...old,
        {
          id: "bot-" + number,
          name: "电脑 " + number,
          bot: true,
          ready: true,
          left: false,
        },
      ]);
    }
    if (type === "remove_bot")
      setSeats((old) => old.filter((s) => s.id !== extra?.target));
  };
  return (
    <div className={`room-page${playing ? " playing" : ""}`}>
      <div style={{ marginBottom: 16 }}>
        <label>
          界面
          <select
            aria-label="验收界面"
            value={mode}
            onChange={(e) => setMode(e.target.value)}
          >
            <option value="module-waiting">模块等待房间</option>
            <option value="legacy-waiting">旧版等待房间</option>
            <option value="module-playing">模块玩家栏</option>
            <option value="legacy-playing">旧版玩家栏</option>
          </select>
        </label>
        <button onClick={() => setLong((v) => !v)}>
          {long ? "普通名称" : "长名称"}
        </button>
      </div>
      {playing ? (
        legacy ? (
          <LegacyPlayers room={room} />
        ) : (
          <ModulePlayers room={room} />
        )
      ) : legacy ? (
        <LegacyWaiting room={room} host busy={false} command={command} />
      ) : (
        <ModuleWaiting room={room} host busy={false} command={command} />
      )}
    </div>
  );
}
void loadGame("monopoly").then(() =>
  createRoot(document.getElementById("root")!).render(<App />),
);
