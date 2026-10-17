// Component acceptance fixture. Server authorization is tested separately.
import { useState } from "react";
import { createRoot } from "react-dom/client";
import { Preparation } from "../../games/basketball/web/Preparation";
import type { GameRoom } from "../../web/src/game-api/module";
import type { BasketballOptions } from "../../games/basketball/web/module";
import { OptionsForm } from "../../games/basketball/web/module";
const actions: Record<string, unknown>[] = [];
const initial: BasketballOptions = {
  mode: "versus",
  court: "new-york",
  teams: [0, 1],
  characters: [0, 1, 0, 1],
};
function Harness() {
  const [options, setOptions] = useState(initial);
  const [you, setYou] = useState(0);
  const [busy, setBusy] = useState(false);
  const [creation, setCreation] = useState(false);
  const room: GameRoom = {
    id: "fixture",
    name: "准备阶段",
    kind: "basketball",
    host: "p0",
    capacity: options.mode === "solo" ? 1 : 2,
    status: "waiting",
    version: 1,
    you,
    spectating: you < 0,
    seats: [
      { id: "p0", name: "房主", ready: false },
      ...(options.mode === "solo"
        ? []
        : [{ id: "p1", name: "客人", ready: false }]),
    ],
  };
  Object.assign(window, {
    preparationFixture: {
      actions,
      options,
      setOptions,
      setYou,
      setBusy,
      setCreation,
    },
  });
  return (
    <main>
      <h1>热血篮球 · 准备阶段</h1>
      {creation ? (
        <OptionsForm
          value={options}
          onChange={setOptions}
          capacity={room.capacity}
          disabled={false}
        />
      ) : (
        <Preparation
          room={room}
          options={options}
          host={you === 0}
          busy={busy}
          assets="/broken-cdn"
          prepare={(action) => {
            actions.push(action);
            setOptions((old) =>
              action.type === "court"
                ? { ...old, court: action.court as BasketballOptions["court"] }
                : {
                    ...old,
                    characters: old.characters!.map((v, i) =>
                      i === action.actor
                        ? Number(action.character)
                        : i === (Number(action.actor) ^ 1) &&
                            v === action.character
                          ? old.characters![Number(action.actor)]
                          : v,
                    ) as [number, number, number, number],
                  },
            );
          }}
        />
      )}
    </main>
  );
}
createRoot(document.getElementById("root")!).render(<Harness />);
