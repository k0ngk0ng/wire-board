type RoadRotation = {
  set(x: number, y: number, z: number, order: "YXZ"): unknown;
};

/** Backend pitch/roll are the road plane's forward/+X slope angles.
 * The model faces +Z. Yaw is applied by its parent; positive pitch raises the
 * nose and positive roll raises +X. Convert the two slopes to consecutive
 * rotations so the road node's up vector follows the plane on compound grades.
 */
export function applyClassicRoadAttitude(
  rotation: RoadRotation,
  pitch: number,
  roll: number,
): void {
  rotation.set(-pitch, 0, Math.atan(Math.tan(roll) * Math.cos(pitch)), "YXZ");
}
