import { useEffect, useRef, useState, type ReactNode } from "react";
import type { Room } from "../types";
import { gameModule } from "../game-api/registry";

type SettingsPatch = { capacity?: number; roomPassword?: string };

export function RoomSettings({
  room,
  host,
  busy,
  command,
  optionsForCapacity,
  playerCounts,
  children,
}: {
  room: Room;
  host: boolean;
  busy: boolean;
  command: (
    type: string,
    extra?: Record<string, unknown>,
  ) => void | Promise<unknown>;
  optionsForCapacity?: (capacity: number) => unknown;
  playerCounts?: readonly number[];
  children?: ReactNode;
}) {
  const game = gameModule(room.kind)!;
  const [capacity, setCapacity] = useState(room.capacity);
  const [password, setPassword] = useState("");
  const [passwordDirty, setPasswordDirty] = useState(false);
  const [, setRevision] = useState(0);
  const latest = useRef({ room, password, capacity });
  latest.current = { room, password, capacity };
  const pending = useRef(new Map<object, SettingsPatch>());
  useEffect(() => {
    pending.current.clear();
    setCapacity(room.capacity);
    setPassword("");
    setPasswordDirty(false);
  }, [host, room.id]);
  useEffect(() => {
    const requested = [...pending.current.values()]
      .reverse()
      .find((patch) => patch.capacity !== undefined)?.capacity;
    setCapacity(requested ?? room.capacity);
  }, [room.capacity]);
  const enqueue = (patch: SettingsPatch) => {
    if (
      !host ||
      [...pending.current.values()].some(
        (entry) =>
          (patch.roomPassword !== undefined &&
            entry.roomPassword === patch.roomPassword) ||
          (patch.capacity !== undefined && entry.capacity === patch.capacity),
      )
    )
      return;
    const submission = {};
    const roomId = room.id;
    pending.current.set(submission, patch);
    setRevision((value) => value + 1);
    Promise.resolve(
      command("room_settings", {
        capacity: patch.capacity ?? room.capacity,
        ...patch,
        ...(patch.capacity === undefined || !optionsForCapacity
          ? {}
          : { options: optionsForCapacity(patch.capacity) }),
      }),
    )
      .then((success) => {
        if (
          !pending.current.has(submission) ||
          latest.current.room.id !== roomId
        )
          return;
        if (success === false) {
          if (
            patch.capacity !== undefined &&
            patch.capacity === latest.current.capacity
          )
            setCapacity(latest.current.room.capacity);
          return;
        }
        if (
          patch.roomPassword !== undefined &&
          latest.current.password === patch.roomPassword
        ) {
          setPassword("");
          setPasswordDirty(false);
        }
      })
      .catch(() => {
        if (
          pending.current.has(submission) &&
          latest.current.room.id === roomId
        )
          setCapacity(latest.current.room.capacity);
      })
      .finally(() => {
        if (!pending.current.delete(submission)) return;
        setRevision((value) => value + 1);
      });
  };
  const finishPassword = () => {
    if (passwordDirty) enqueue({ roomPassword: password });
  };
  const counts =
    playerCounts ??
    game.playerCounts ??
    Array.from(
      { length: game.maxPlayers - game.minPlayers + 1 },
      (value, index) => game.minPlayers + index,
    );
  return (
    <form
      className="room-settings"
      aria-busy={busy}
      onSubmit={(event) => {
        event.preventDefault();
        finishPassword();
      }}
    >
      <div className="form-grid">
        <label>
          房间人数
          <select
            aria-label="房间人数"
            value={capacity}
            disabled={
              !host ||
              busy ||
              counts.length === 1 ||
              [...pending.current.values()].some(
                (patch) => patch.capacity !== undefined,
              )
            }
            onChange={(event) => {
              const value = Number(event.target.value);
              if (!Number.isSafeInteger(value)) return;
              if (value === room.capacity && pending.current.size === 0) return;
              setCapacity(value);
              enqueue({ capacity: value });
            }}
          >
            {counts.map((count) => (
              <option
                key={count}
                value={count}
                disabled={count < room.seats.length}
              >
                {count} 人
              </option>
            ))}
          </select>
        </label>
        {host && (
          <label>
            房间密码
            <input
              aria-label="房间密码"
              type="password"
              autoComplete="new-password"
              maxLength={72}
              disabled={busy || pending.current.size > 0}
              value={password}
              placeholder={room.locked ? "输入新密码" : "不设密码"}
              onChange={(event) => {
                setPassword(event.target.value);
                setPasswordDirty(true);
              }}
              onBlur={finishPassword}
            />
          </label>
        )}
      </div>
      {host && room.locked && (
        <button
          type="button"
          className="subtle"
          data-room-password-remove
          disabled={busy || pending.current.size > 0}
          onClick={() => {
            setPassword("");
            setPasswordDirty(false);
            enqueue({ roomPassword: "" });
          }}
        >
          移除密码
        </button>
      )}
      {children}
    </form>
  );
}
