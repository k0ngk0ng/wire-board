import test from "node:test";
import assert from "node:assert/strict";
import {
  projectToScreen,
  paddleRadiusPixels,
  pointerOverBall,
  paddleReachRadius,
  paddleAtScreen,
} from "../web/view-space.ts";
test("visible fixed-size paddle controls contact independently of ball depth", () => {
  for (const [w, h] of [
    [1120, 700],
    [294, 562],
  ])
    for (const z of [0.2, 1.3, 2.25]) {
      const ball = { x: 0, y: 1, z },
        screen = projectToScreen(ball, w, h),
        r = paddleRadiusPixels(w);
      assert.equal(pointerOverBall(screen, ball, w, h), true);
      const outside = { x: screen.x, y: screen.y + (r * 1.13 + 22) / h };
      assert.equal(pointerOverBall(outside, ball, w, h), false);
      const edge = { x: screen.x + (r + 4) / w, y: screen.y };
      assert.equal(pointerOverBall(edge, ball, w, h), true);
      const p = paddleAtScreen(edge, z, w, h);
      assert.ok(
        Math.hypot(p.x - ball.x, p.y - ball.y) <=
          paddleReachRadius(edge, z, w, h),
      );
      assert.ok(paddleReachRadius(edge, z, w, h) <= 0.45);
    }
});
