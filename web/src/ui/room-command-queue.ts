type RoomIdentity = { id: string; status: string; version: number };

export class RoomCommandQueue<Current extends RoomIdentity> {
  private current: () => Current | undefined;
  private tail: Promise<void> = Promise.resolve();
  private roomId = "";
  private advances = new Map<number, number>();
  private generation = 0;

  constructor(current: () => Current | undefined) {
    this.current = current;
  }

  enqueue(
    visible: RoomIdentity,
    operation: (room: Current) => Promise<boolean>,
    passwordOnly = false,
  ): Promise<boolean> {
    const { id, version: requestedVersion, status } = visible;
    const generation = this.generation;
    const result = this.tail.then(async () => {
      const room = this.current();
      if (!room || room.id !== this.roomId || room.status !== "waiting") {
        this.advances.clear();
        this.roomId = room?.id ?? "";
      }
      if (
        generation !== this.generation ||
        !room ||
        status !== "waiting" ||
        room.id !== id ||
        room.status !== "waiting"
      )
        return false;
      if (
        !Number.isSafeInteger(requestedVersion) ||
        !Number.isSafeInteger(room.version)
      )
        return false;
      let version = requestedVersion;
      for (let depth = 0; depth < 64 && version !== room.version; depth++) {
        const next = this.advances.get(version);
        if (next === undefined) return false;
        version = next;
      }
      if (version !== room.version) return false;
      let succeeded = false;
      try {
        succeeded = await operation(room);
      } catch (error) {
        this.generation++;
        throw error;
      }
      if (!succeeded) {
        this.generation++;
        return false;
      }
      const after = this.current();
      if (
        passwordOnly &&
        after?.id === id &&
        after.status === "waiting" &&
        Number.isSafeInteger(after.version) &&
        after.version === room.version + 1
      ) {
        this.advances.set(room.version, after.version);
        if (this.advances.size > 64)
          this.advances.delete(this.advances.keys().next().value!);
      }
      return true;
    });
    this.tail = result.then(
      () => undefined,
      () => undefined,
    );
    return result;
  }
}
