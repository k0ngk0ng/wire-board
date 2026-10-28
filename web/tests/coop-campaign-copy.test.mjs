// Failure cases before implementation: close confirmation promises zero points
// for a rated challenge; pending loading charges loss; closed/expired failures
// show generic cancellation; robot/practice/legacy exclusions show wrong reasons.
import test from "node:test";
import assert from "node:assert/strict";
import {
  closeTableDescription,
  cooperativeOutcomeLabel,
  unratedDescription,
  cooperativeRatingNotice,
} from "../src/ui/cooperative-rating.ts";

test("human-only cooperative notice omits unavailable computer and autoplay conditions", () => {
  const original =
    "计分挑战通关时全员 +20，失败或主动结束时全员 −10；连续 24 小时无操作（暂停时 30 天）按放弃结算。含电脑或练习规则不计分，真人主动托管仍计分。";
  assert.equal(cooperativeRatingNotice(), original);
  assert.equal(cooperativeRatingNotice(true), original);
  const human = cooperativeRatingNotice(false);
  assert.match(human, /通关时全员 \+20/);
  assert.match(human, /失败或主动结束时全员 −10/);
  assert.match(human, /24 小时.*30 天/);
  assert.doesNotMatch(human, /电脑|托管|练习/);
});

test("close confirmation distinguishes an active rated challenge from loading and legacy", () => {
  assert.match(
    closeTableDescription({
      ratingPolicy: "coop_campaign_v1",
      ratingStarted: true,
      ratingEligible: true,
    }),
    /每位.*10/,
  );
  for (const room of [
    {},
    {
      ratingPolicy: "coop_campaign_v1",
      ratingStarted: false,
      ratingEligible: true,
    },
    {
      ratingPolicy: "coop_campaign_v1",
      ratingStarted: true,
      ratingEligible: false,
    },
  ]) {
    assert.match(closeTableDescription(room), /不.*积分/);
    assert.doesNotMatch(closeTableDescription(room), /扣.*10/);
  }
});
test("cooperative outcome names distinguish victory, defeat, abandonment and interruption", () => {
  assert.equal(cooperativeOutcomeLabel({ outcome: "cleared" }), "全队通关");
  for (const outcome of ["failed", "abandoned", "expired"])
    assert.equal(cooperativeOutcomeLabel({ outcome }), "挑战失败");
  assert.equal(cooperativeOutcomeLabel({ outcome: "cancelled" }), "挑战已取消");
  assert.equal(
    cooperativeOutcomeLabel({ outcome: "interrupted" }),
    "挑战已中止",
  );
});
test("unrated copy explains the actual exclusion and keeps old cooperative compatibility", () => {
  assert.match(unratedDescription({ unratedReason: "computer" }), /电脑/);
  assert.match(unratedDescription({ unratedReason: "practice" }), /练习/);
  assert.match(
    unratedDescription({ unratedReason: "not_started" }),
    /尚未开始/,
  );
  assert.match(
    unratedDescription({ unratedReason: "administrative" }),
    /管理员/,
  );
  assert.match(
    unratedDescription({
      cooperative: true,
      unratedReason: "cooperative_unrated",
    }),
    /未启用计分/,
  );
  assert.match(unratedDescription({ cooperative: true }), /未启用计分/);
});
