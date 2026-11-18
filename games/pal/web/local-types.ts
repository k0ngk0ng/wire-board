import type { ReplayOperation } from "./native-runtime";
export type LocalInputRequest = {
  epoch: number;
  seq: number;
  direction: number;
  keys: number;
  interrupt?: boolean;
  preview?: boolean;
  commitPreview?: boolean;
  cancelPreview?: boolean;
};
export type LocalMeta = {
  generation: number;
  revision: number;
  core: string;
  digest?: string;
  boundary?: string;
  remainingMS: number;
  serverAt: number;
  seq: number;
  appliedSeq: number;
  discardedSeq: number;
  direction: number;
  until: number;
  records?: {
    revision: number;
    operations: ReplayOperation[];
    digest: string;
    appliedSeq: number;
  }[];
  resync?: boolean;
  checkpoint?: string;
};
export type LocalAction = {
  type: "local_input" | "local_release";
  epoch: number;
  seq: number;
  boundary: string;
  direction: number;
  keys: number;
  controlAt: number;
};
