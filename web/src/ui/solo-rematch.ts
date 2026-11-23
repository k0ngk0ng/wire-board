import type { Room } from "../types";

const onlyHumanHost = (room: Room, user: string) => !room.spectating && room.host === user && room.seats.filter(seat => !seat.bot && !seat.left).length === 1;

export function canQuickRematch(room: Room, user: string) {
  return onlyHumanHost(room, user) && ["finished", "closed"].includes(room.status);
}

export async function quickRematch(
  id: string,
  user: string,
  read: () => Promise<Room | undefined>,
  send: (room: Room, type: string) => Promise<unknown>,
) {
  let room = await read();
  if (!room || room.id !== id || !canQuickRematch(room, user)) throw Error("当前牌桌不能直接重开");
  for (const type of ["rematch", "ready", "start"]) {
    if (room.id !== id) throw Error("牌桌已变化，请返回当前房间");
    if (!onlyHumanHost(room, user)) throw Error("玩家已变化，请在房间中确认准备");
    if (type !== "rematch" && room.status !== "waiting") throw Error("牌桌状态已变化，请重试");
    await send(room, type);
    const next = await read();
    if (!next) throw Error("牌桌已关闭");
    room = next;
  }
  return room;
}
