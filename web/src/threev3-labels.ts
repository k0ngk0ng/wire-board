// Shared labels for the legacy 3v3 prompts so the table and the tests agree.
export function threeV3PromptLabels(
  ask?: string,
): { value: string; label: string }[] {
  switch (ask) {
    case "sg_3v3_pile":
      return [1, 2, 3, 4].map((n) => ({
        value: String(n),
        label: `第 ${n} 堆（8 名，暗置）`,
      }));
    case "sg_3v3_first":
      return [
        { value: "us", label: "我方先选" },
        { value: "them", label: "对方先选" },
      ];
    case "sg_3v3_first_side":
      return [
        { value: "cold", label: "冷色方先手" },
        { value: "warm", label: "暖色方先手" },
      ];
    case "sg_3v3_side":
      return [
        { value: "vanguards", label: "两名前锋连续行动" },
        { value: "leader", label: "主帅行动" },
      ];
    default:
      return [];
  }
}

export function threeV3CampName(camp?: number): string {
  return camp === 0 ? "冷色方" : "暖色方";
}

// The official 3v3 room sends no laid-out pool until both leaders have taken a
// pile, so the table must never read `.length` of a missing list.
export function threeV3PoolList(pool?: string[] | null): string[] {
  return Array.isArray(pool) ? pool : [];
}

export function threeV3PoolHint(
  stage?: string,
  pool?: string[] | null,
): string | null {
  if (threeV3PoolList(pool).length > 0) {
    return null;
  }
  if (stage === "piles") {
    return "取堆阶段：两方主帅各取一堆后，平摊 16 名武将。";
  }
  if (stage === "pick" || stage === "assign") {
    return "选将结束，等待双方主帅分配出场武将。";
  }
  return null;
}
