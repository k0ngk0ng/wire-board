type ScopeRoom = {
  id: string;
  matchId?: string;
  setupVersion?: number;
  status: string;
  you: number;
  spectating?: boolean;
  loading?: { token: string } | null;
  seats: readonly { id: string; left?: boolean; autoPlay?: boolean }[];
  game?: { data?: unknown };
};
export type ActionScopeState = {
  serverInstance?: string;
  user?: { id: string };
  room?: ScopeRoom;
};
export type ActionScopeToken = Readonly<{ identity: string }>;

function gameEpoch(
  state: ActionScopeState | undefined,
): string | number | undefined {
  const data = state?.room?.game?.data;
  if (!data || typeof data !== "object" || Array.isArray(data)) return;
  const epoch = (data as { epoch?: unknown }).epoch;
  if (
    typeof epoch === "string" ||
    (typeof epoch === "number" && Number.isFinite(epoch))
  )
    return epoch;
}

function identity(state: ActionScopeState | undefined): string {
  const room = state?.room;
  if (!room) return "";
  const seat = room.seats[room.you];
  return JSON.stringify([
    state?.serverInstance,
    state?.user?.id,
    room.id,
    room.matchId,
    room.setupVersion,
    room.status,
    room.you,
    seat?.id,
    !!room.spectating,
    !!seat?.left,
    !!seat?.autoPlay,
    room.loading?.token,
    gameEpoch(state),
  ]);
}

export class ActionScopes {
  private token: ActionScopeToken = { identity: "" };
  private paused = false;
  private ownedError?: { token: ActionScopeToken; message: string };

  update(state: ActionScopeState | undefined): string | undefined {
    const next = identity(state);
    if (next === this.token.identity) return;
    const cleared = this.ownedError?.message;
    this.token = { identity: next };
    this.paused = false;
    this.ownedError = undefined;
    return cleared;
  }

  capture(
    state: ActionScopeState | undefined,
    requestedEpoch?: unknown,
  ): ActionScopeToken | undefined {
    const epoch = gameEpoch(state);
    if (
      (typeof requestedEpoch === "string" ||
        (typeof requestedEpoch === "number" &&
          Number.isFinite(requestedEpoch))) &&
      epoch !== undefined &&
      requestedEpoch !== epoch
    )
      return;
    if (
      !this.paused &&
      this.token.identity &&
      identity(state) === this.token.identity
    )
      return this.token;
  }

  isCurrent(token: ActionScopeToken | undefined): token is ActionScopeToken {
    return !!token && !!token.identity && token === this.token && !this.paused;
  }

  pause(): { token: ActionScopeToken; cleared?: string } {
    const cleared = this.ownedError?.message;
    this.token = { identity: this.token.identity };
    this.paused = true;
    this.ownedError = undefined;
    return { token: this.token, cleared };
  }

  resume(token: ActionScopeToken, state: ActionScopeState | undefined) {
    if (token === this.token && identity(state) === token.identity)
      this.paused = false;
  }

  report(token: ActionScopeToken | undefined, message: string): boolean {
    if (!this.isCurrent(token)) return false;
    this.ownedError = { token, message };
    return true;
  }

  forgetError(message?: string) {
    if (message === undefined || this.ownedError?.message === message)
      this.ownedError = undefined;
  }

  async run<T>(
    token: ActionScopeToken | undefined,
    operation: () => Promise<T>,
    failure: (error: unknown, scope: ActionScopeToken) => void,
  ): Promise<T> {
    if (!this.isCurrent(token))
      throw Object.assign(new Error(""), {
        name: "InactiveActionScopeError",
        transientSync: true,
      });
    try {
      return await operation();
    } catch (error) {
      if (this.isCurrent(token)) failure(error, token);
      throw error;
    }
  }
}
