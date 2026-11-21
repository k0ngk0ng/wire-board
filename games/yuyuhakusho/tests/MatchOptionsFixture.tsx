import { useState } from "react";
import { createRoot } from "react-dom/client";
import type { GameRoom } from "../../../web/src/game-api/module";
import { ModulePlayers } from "../../../web/src/game-api/host";
import "../../../web/src/style.css";
import { OptionsForm } from "../web/Options";
import { Preparation } from "../web/Preparation";
import type { GameOptions } from "../web/types";

function App() {
  const [options, setOptions] = useState<GameOptions>({
    arena: 0,
    teamBattle: false,
    characters: [3, 1, 0, 10],
  });
  const [capacity, setCapacity] = useState(4);
  const [host, setHost] = useState(true);
  const [creating, setCreating] = useState(true);
  const room: GameRoom = {
    id: "fixture",
    name: "比赛准备",
    kind: "yuyuhakusho",
    host: "0",
    capacity,
    status: "waiting",
    version: 1,
    you: host ? 0 : 1,
    options,
    seats: ["小明", "小红", "阿绿", "电脑"]
      .slice(0, capacity)
      .map((name, seat) => ({
        id: String(seat),
        name,
        ready: false,
        bot: seat >= 2,
      })),
    game: {
      data: {
        options,
        players: options.characters.map((characterID, seat) => ({
          seat,
          characterID,
          health: 223,
          energy: 223,
        })),
      },
      finished: false,
      winners: [],
      log: [],
    },
  };
  const prepare = (action: Record<string, unknown>) =>
    setOptions((previous) => {
      if (action.type === "arena")
        return { ...previous, arena: Number(action.arena) };
      if (action.type === "mode")
        return {
          ...previous,
          teamBattle: !!action.teamBattle,
          oneVsThree: !!action.oneVsThree,
        };
      const characters = [...previous.characters];
      characters[Number(action.seat)] = Number(action.character);
      return { ...previous, characters };
    });
  return (
    <main style={{ maxWidth: 760, margin: "auto", padding: 12 }}>
      <nav style={{ display: "flex", gap: 8, marginBottom: 16 }}>
        <button onClick={() => setCreating((value) => !value)}>
          {creating ? "进入准备" : "创建选项"}
        </button>
        <button onClick={() => setHost((value) => !value)}>切换房主</button>
        <button onClick={() => setCapacity((value) => (value === 4 ? 2 : 4))}>
          切换人数
        </button>
      </nav>
      {creating ? (
        <OptionsForm
          value={options}
          onChange={setOptions}
          capacity={capacity}
          disabled={false}
        />
      ) : (
        <Preparation
          room={room}
          options={options}
          host={host}
          busy={false}
          prepare={prepare}
          assets=""
        />
      )}
      {!creating && (
        <ModulePlayers
          room={room as Parameters<typeof ModulePlayers>[0]["room"]}
        />
      )}
      <output id="selection" style={{ display: "none" }}>
        {JSON.stringify(options)}
      </output>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
