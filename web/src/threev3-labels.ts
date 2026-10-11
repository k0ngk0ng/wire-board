// Shared labels for the legacy 3v3 prompts so the table and the tests agree.
export function threeV3PromptLabels(
  ask?: string,
): { value: string; label: string }[] {
  switch (ask) {
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
