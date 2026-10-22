// Failure scenarios before implementation: cosmetic-only slope, wrong sign on
// reversal, clamped >90 degree world trajectory, pointer double-counts slope,
// detached support rotates the shot, preview and server disagree.
import test from "node:test";
import assert from "node:assert/strict";
import { previewPath, aimFromPoint } from "../web/aim.ts";
import { supportSlope } from "../web/slope.ts";

test("preview incorporates actual support for both shooting directions", () => {
  for (const grade of [-0.5, 0, 0.5]) {
    const mask = new Uint8Array(1250);
    for (let cy = 0; cy < 100; cy++)
      for (let cx = 0; cx < 100; cx++) {
        if (cy * 4 >= 200 + (cx * 4 - 200) * grade) {
          const i = cy * 100 + cx;
          mask[i >> 3] |= 1 << (7 - (i & 7));
        }
      }
    for (const facing of [-1, 1]) {
      const path = previewPath({
        width: 400,
        height: 400,
        mask,
        maskCols: 100,
        maskRows: 100,
        cell: 4,
        startX: 200,
        startY: 200,
        angle: 45,
        power: 60,
        facing,
        rangeScale: 1,
        wind: 0,
        limit: 4,
      });
      const [p, q] = path;
      const angle =
        (Math.atan2(-(q.y - p.y), (q.x - p.x) * facing) * 180) / Math.PI;
      const want = 45 - ((Math.atan(grade) * 180) / Math.PI) * facing;
      assert.ok(
        Math.abs(angle - want) < 5,
        JSON.stringify({ grade, facing, angle, want }),
      );
    }
  }
});

test("support exposes continuous angle and pointer converts world to local aim", () => {
  const support = supportSlope(
    { x: 100, y: 100 },
    (x, y) => y >= 100 + (x - 100) * 0.5,
  );
  assert.ok(Math.abs(support.angle - 26.565) < 0.01);
  assert.equal(supportSlope({ x: 100, y: 100 }, () => false).angle, 0);
  const right = aimFromPoint({ x: 100, y: 126 }, { x: 200, y: 0 }, 90, 26, 20);
  const left = aimFromPoint({ x: 100, y: 126 }, { x: 0, y: 0 }, 90, 26, 20);
  assert.equal(right.angle, 65);
  assert.equal(left.angle, 25);
});
