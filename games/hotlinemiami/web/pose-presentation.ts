type Pose = { x: number; y: number; life: number; alive: boolean };

export class PosePresentation {
  private x = 0;
  private y = 0;
  private at = 0;

  reconcile(before: Pose, after: Pose, at: number, force = false) {
    if (force || before.life !== after.life || !before.alive || !after.alive) {
      this.reset();
      return;
    }
    const dx = before.x - after.x, dy = before.y - after.y;
    if (!dx && !dy) return;
    const offset = this.offset(at);
    const x = offset.x + dx, y = offset.y + dy;
    if (Math.abs(dx) + Math.abs(dy) > 2048 || Math.abs(x) + Math.abs(y) > 2048) {
      this.reset();
      return;
    }
    this.x = x;
    this.y = y;
    this.at = Math.trunc(at);
  }

  sample<T extends Pose>(logical: T, at: number): T {
    if (!logical.alive) return logical;
    const offset = this.offset(at);
    return { ...logical, x: logical.x + offset.x, y: logical.y + offset.y };
  }

  reset() {
    this.x = 0;
    this.y = 0;
    this.at = 0;
  }

  private offset(at: number) {
    const remaining = Math.max(0, 100 - Math.max(0, Math.trunc(at) - this.at));
    return { x: Math.round(this.x * remaining / 100), y: Math.round(this.y * remaining / 100) };
  }
}
