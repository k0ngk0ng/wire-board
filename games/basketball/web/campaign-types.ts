export type CampaignCharacterGrades = {
  Run: number;
  Jump: number;
  Shot: number;
  Recovery: number;
  Attack: number;
};
export type CampaignAttributes = {
  current: CampaignCharacterGrades[];
  inventory: {
    index: number;
    item: number;
    after: CampaignCharacterGrades[];
  }[];
};
export type CampaignProgress = {
  mode: number;
  period: number;
  winBits: [number, number];
  rewardSet: number;
  inventory: number[];
  equipment: number[][];
};
export type CampaignOptions = {
  attributes?: CampaignAttributes;
  entry?: "new" | "password";
  password?: string;
  opponent?: number;
  tactic?: number;
  preparation?: CampaignProgress;
  availableOpponents?: number[];
};
export type CampaignPreparationView = {
  progress: CampaignProgress;
  teammates: [number, number];
  humanTactics: [number, number];
  period: number;
  revision: number;
  canChooseTactic: boolean;
};
export type CampaignRoundPreparation = {
  progress: CampaignProgress;
  random: number;
  opponent: number;
  teammates: [number, number];
  humanTactics: [number, number];
};
export type CampaignView = {
  attributes?: CampaignAttributes;
  stage: "playing" | "reward" | "defeat" | "prepare" | "complete" | "quit";
  round: number;
  revision: number;
  decisionAt: number;
  automaticAt: number;
  canAct: boolean;
  canChooseTactic: boolean;
  password?: string;
  opponents: number[];
  preparation?: CampaignRoundPreparation;
  settlement?: {
    progress: CampaignProgress;
    reward: number;
    rewardStored: boolean;
    nextStage: string;
    random: number;
  };
};
