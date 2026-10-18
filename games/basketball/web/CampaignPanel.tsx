import { useEffect, useLayoutEffect, useState } from "react";
import { Modal } from "../../../web/src/ui/board-host";
import type { GameClock } from "../../../web/src/game-api/module";
import type { BasketballNetworkView } from "./network-scene";
import {
  CampaignPreparationEditor,
  campaignItemName,
} from "./CampaignPreparation";
import "./campaign.css";
type Props = {
  data: BasketballNetworkView;
  busy: boolean;
  assets: string;
  act: (action: Record<string, unknown>) => Promise<void>;
  clock: GameClock;
  onOpenChange: (open: boolean) => void;
};
export function CampaignPanel({
  data,
  busy,
  assets,
  act,
  clock,
  onOpenChange,
}: Props) {
  const half = data.phase === "intermission" ? data.preparation : undefined;
  const c = data.campaign;
  const menu =
    c && ["reward", "defeat", "prepare"].includes(c.stage) ? c : undefined;
  const key = half
    ? `${data.epoch}:half:${half.period}`
    : menu
      ? `${data.epoch}:${menu.stage}`
      : "";
  const [closedKey, setClosedKey] = useState(""),
    [sending, setSending] = useState(false),
    [error, setError] = useState("");
  const [now, setNow] = useState(() => performance.now());
  const open = !!key && closedKey !== key;
  useLayoutEffect(() => {
    onOpenChange(open);
    return () => onOpenChange(false);
  }, [open, onOpenChange]);
  useEffect(() => {
    setError("");
  }, [key]);
  const automaticAt = menu?.automaticAt ?? 0;
  useEffect(() => {
    if (!automaticAt) return;
    setNow(performance.now());
    const id = setInterval(() => setNow(performance.now()), 200);
    return () => clearInterval(id);
  }, [automaticAt]);
  const seconds = Math.max(
    0,
    Math.ceil(
      (automaticAt - clock.serverNow - Math.max(0, now - clock.receivedAt)) /
        1000,
    ),
  );
  const countdown = automaticAt ? (
    <p
      className="basketball-campaign-countdown"
      data-campaign-countdown
      role="status"
    >
      {seconds > 0
        ? `电脑将在 ${seconds} 秒后${menu?.stage === "prepare" ? "开始下一场" : "继续战役"}`
        : "电脑正在确认…"}
    </p>
  ) : null;
  const title = half
    ? "中场准备"
    : menu?.stage === "prepare"
      ? "下一场准备"
      : menu?.stage === "reward"
        ? "比赛获胜"
        : "比赛失利";
  const locked = busy || sending;
  const send = async (partial: Record<string, unknown>) => {
    // Capture the displayed snapshot, not a later room version or fresh index.
    const request = half
      ? {
          type: "campaign-preparation",
          epoch: data.epoch,
          preparation: {
            ...partial,
            period: half.period,
            revision: half.revision,
          },
        }
      : {
          type: "campaign",
          epoch: data.epoch,
          campaign: { ...partial, revision: menu!.revision },
        };
    setSending(true);
    setError("");
    try {
      await act(request);
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : "操作未完成，请检查当前准备状态后重试。",
      );
    } finally {
      setSending(false);
    }
  };
  if (!c && !half) return null;
  const preparation = half ?? menu?.preparation;
  const canEdit = half ? !!data.canPrepare : !!menu?.canAct;
  return (
    <div className="basketball-campaign-panel">
      <div className="basketball-campaign-toolbar">
        <span>
          {half ? "中场休息" : c ? `战役 · 第 ${c.round} 场` : "战役"}
        </span>
        <button
          type="button"
          data-campaign-open
          disabled={!key}
          onClick={() => setClosedKey("")}
        >
          {key
            ? `查看${half ? "中场准备" : "战役进度"}`
            : c?.stage === "complete"
              ? "战役已通关"
              : c?.stage === "quit"
                ? "战役已结束"
                : "战役进行中"}
        </button>
      </div>
      {!open && countdown}
      {open && (
        <Modal
          title={title}
          className="basketball-campaign-modal"
          onClose={() => setClosedKey(key)}
        >
          {countdown}
          {error && (
            <p role="alert" className="basketball-campaign-error">
              {error}
            </p>
          )}
          {menu?.settlement && menu.stage !== "prepare" && (
            <div className="basketball-campaign-result">
              <p className="basketball-campaign-score">
                {data.scores[0]} : {data.scores[1]}
              </p>
              {menu.stage === "reward" ? (
                <p>
                  获得{campaignItemName(menu.settlement.reward)}
                  {menu.settlement.rewardStored
                    ? "，已放入库存。"
                    : "，但库存已满，未能存入。"}
                </p>
              ) : (
                <p>继续战役会保留已有进度和装备。</p>
              )}
            </div>
          )}
          {preparation && (
            <CampaignPreparationEditor
              progress={preparation.progress}
              teammates={preparation.teammates}
              tactic={preparation.humanTactics[0]}
              assets={assets}
              busy={locked}
              canEditTeam={canEdit}
              canChooseTactic={
                half ? half.canChooseTactic : !!menu?.canChooseTactic
              }
              revision={half?.revision ?? menu!.revision}
              snapshotKey={`${key}:${half?.revision ?? menu!.revision}`}
              opponent={half ? undefined : menu?.preparation?.opponent}
              opponents={menu?.opponents}
              canEditCharacter={() => canEdit}
              chooseCharacter={(position, character) => {
                const pair: [number, number] = [...preparation.teammates];
                if (pair[position ^ 1] === character)
                  pair[position ^ 1] = pair[position];
                pair[position] = character;
                void send({ type: "teammates", teammates: pair });
              }}
              send={(a) => {
                void send(a);
              }}
            />
          )}
          {c?.password && (
            <details className="basketball-campaign-password">
              <summary>查看原版密码</summary>
              <p>
                <code>{c.password}</code>
              </p>
              <p>密码仅保存战绩和奖励；当前装备与库存顺序随房间存档保留。</p>
            </details>
          )}
          <div className="basketball-campaign-actions">
            {half ? (
              <button type="button" onClick={() => setClosedKey(key)}>
                完成准备
              </button>
            ) : menu?.stage === "prepare" ? (
              <button
                type="button"
                data-campaign-start
                disabled={locked || !canEdit}
                onClick={() => {
                  void send({ type: "start" });
                }}
              >
                开始下一场
              </button>
            ) : (
              <button
                type="button"
                data-campaign-continue
                disabled={locked || !canEdit}
                onClick={() => {
                  void send({ type: "continue" });
                }}
              >
                {menu?.stage === "reward" ? "准备下一场" : "继续战役"}
              </button>
            )}
            {menu && (
              <button
                type="button"
                disabled={locked || !canEdit}
                onClick={() => {
                  void send({ type: "quit" });
                }}
              >
                结束战役
              </button>
            )}
          </div>
          {half && <p>关闭窗口后，由第一席按 START 开始下半场。</p>}
          {!canEdit && <p>当前仅可查看，等待第一席玩家确认。</p>}
        </Modal>
      )}
    </div>
  );
}
