type RatingSummary = {
  cooperative?: boolean;
  outcome?: string;
  unratedReason?: string;
};

export function closeTableDescription(room: {
  ratingPolicy?: string;
  ratingStarted?: boolean;
  ratingEligible?: boolean;
}): string {
  return room.ratingPolicy === "coop_campaign_v1" &&
    room.ratingStarted &&
    room.ratingEligible
    ? "结束牌桌视为放弃挑战，全队失败，每位参战玩家扣除 10 积分。"
    : "结束后本局不加减积分。所有玩家可以离开牌桌，或由房主重新开局。";
}

export function cooperativeOutcomeLabel(result: RatingSummary): string {
  switch (result.outcome) {
    case "cleared":
      return "全队通关";
    case "failed":
    case "abandoned":
    case "expired":
      return "挑战失败";
    case "cancelled":
      return "挑战已取消";
    case "interrupted":
      return "挑战已中止";
    default:
      return "合作挑战结束";
  }
}

export function unratedDescription(result: RatingSummary): string {
  switch (result.unratedReason) {
    case "computer":
      return "本局有电脑玩家参与，不计积分，成绩已保存。";
    case "practice":
      return "练习对局不计积分，成绩已保存。";
    case "not_started":
      return "挑战尚未开始，本次取消不计积分。";
    case "administrative":
      return "管理员中止了本局，不计积分。";
    case "cooperative_unrated":
      return "本局合作规则未启用计分，成绩已保存。";
    default:
      return result.cooperative
        ? "本局合作规则未启用计分，成绩已保存。"
        : "本局不计积分（含电脑或积分上线前开始的对局）。";
  }
}

export function cooperativeRatingNotice(supportsBots = true): string {
  const notice =
    "计分挑战通关时全员 +20，失败或主动结束时全员 −10；连续 24 小时无操作（暂停时 30 天）按放弃结算。";
  return supportsBots
    ? notice + "含电脑或练习规则不计分，真人主动托管仍计分。"
    : notice;
}
