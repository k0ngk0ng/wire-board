const finite = (value: number | undefined, fallback = 0) => Number.isFinite(value) ? value! : fallback;
const clamp = (value: number, low: number, high: number) => Math.max(low, Math.min(high, value));
export function classicGaugeAngle(value: number, maximum: number): number {
  return -130 + 260 * clamp(finite(value) / Math.max(1, finite(maximum, 1)), 0, 1);
}
export function classicGaugeTicks(maximum: number, major: number, minor: number) {
  return Array.from({ length: Math.floor(maximum / minor) + 1 }, (_, index) => {
    const value = index * minor;
    return { value, angle: classicGaugeAngle(value, maximum), major: value % major === 0 };
  });
}
export function classicGaugeMode(width: number, height: number): "analog" | "compact" {
  return width > 0 && width <= 480 && width <= height ? "compact" : "analog";
}
export function classicGaugeReading(car: { speed: number; rpm?: number; gear?: number }, redline: number) {
  const speed = Math.round(Math.abs(finite(car.speed)) * .36);
  const rpm = Math.round(Math.max(0, finite(car.rpm)) / 100) * 100;
  return { speed, rpm, gear: car.gear === -1 ? "R" : String(clamp(Math.round(finite(car.gear, 1)), 1, 8)),
    speedAngle: classicGaugeAngle(speed, 400), rpmAngle: classicGaugeAngle(rpm, 10000),
    redlineAngle: classicGaugeAngle(redline, 10000) };
}
