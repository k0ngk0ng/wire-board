import test from "node:test";
import assert from "node:assert/strict";
import { sweptPointerContact } from "../web/prediction.ts";
import { projectToScreen, pointerOverBall } from "../web/view-space.ts";
test("fast upward motion hits between frames, with the actual contact timestamp", () => {
  for (const [w, h] of [
    [294, 562],
    [1120, 700],
  ])
    for (const seat of [0, 1]) {
      const dir = seat === 0 ? 1 : -1,
        ball = { x: 0, y: 1, z: dir * 1.5 };
      const view = {
        expected: seat,
        phase: "rally",
        ends: 0,
        reachDepth: 2.25,
        options: { teams: [0, 1] },
        flight: {
          contactStart: 1000,
          segments: [
            {
              at: 1000,
              until: 1200,
              p: ball,
              v: { x: 0, y: 0, z: 0 },
              a: { x: 0, y: 0, z: 0 },
            },
          ],
        },
        controls: { closeAt: 1100 },
      };
      const local = { ...ball, z: 1.5 },
        point = projectToScreen(local, w, h);
      const from = { point: { x: point.x, y: point.y + 90 / h }, at: 1005 },
        to = { point: { x: point.x, y: point.y - 90 / h }, at: 1045 };
      assert.equal(pointerOverBall(from.point, local, w, h), false);
      assert.equal(pointerOverBall(to.point, local, w, h), false);
      const hit = sweptPointerContact(view, seat, from, to, w, h);
      assert.ok(hit);
      assert.ok(hit.at > from.at && hit.at < to.at);
      assert.ok(pointerOverBall(hit.point, local, w, h));
      assert.equal(
        sweptPointerContact(
          view,
          seat,
          { ...from, at: 800 },
          { ...to, at: 850 },
          w,
          h,
        ),
        undefined,
      );
      assert.equal(
        sweptPointerContact(view, seat, { ...from, at: 850 }, to, w, h),
        undefined,
      );
    }
});
test("fast serve motion is also sampled between pointer events", () => {
  const view = {
    expected: 0,
    phase: "serve",
    ends: 0,
    reachDepth: 2.25,
    options: { teams: [0, 1] },
    controls: { openAt: 1000, closeAt: 2000 },
  };
  const point = projectToScreen({ x: 0.34, y: 1.1, z: 1.28 }, 294, 562);
  assert.ok(
    sweptPointerContact(
      view,
      0,
      { point: { x: point.x, y: point.y + 0.2 }, at: 1010 },
      { point: { x: point.x, y: point.y - 0.2 }, at: 1040 },
      294,
      562,
    ),
  );
});
