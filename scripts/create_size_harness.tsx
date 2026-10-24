import { useState } from "react";
import { createRoot } from "react-dom/client";
import { CreateRoom } from "../web/src/ui/create-room";
import { isIndependentGame } from "../web/src/game-api/registry";
import { ModuleCover, ModuleCreate } from "../web/src/game-api/host";
import { LegacyCover } from "../web/src/legacy/cover";
import "../web/src/style.css";

// Real catalogue, covers and shared dialog. Only the outer lobby and submit
// sink are fixtures: this layout check does not create production rooms.
const catalogs = import.meta.glob<{ default: { id: string } }>(
  "../games/*/web/catalog.tsx",
  { eager: true },
);
const games = Object.values(catalogs).map((entry) => entry.default.id);
function App() {
  const [open, setOpen] = useState(false);
  return (
    <main style={{ padding: 24 }}>
      <button className="primary" onClick={() => setOpen(true)}>
        创建房间
      </button>
      {open && (
        <CreateRoom
          availableGames={games}
          busy={false}
          onClose={() => setOpen(false)}
          onSubmit={() => {}}
          renderCover={(kind) =>
            isIndependentGame(kind) ? (
              <ModuleCover kind={kind} mini />
            ) : (
              <LegacyCover kind={kind} mini />
            )
          }
          renderForm={(props) => <ModuleCreate {...props} />}
        />
      )}
    </main>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
