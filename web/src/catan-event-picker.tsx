export function CatanEventPicker({
  value,
  disabled = false,
  citiesKnights = false,
  cloth = false,
  friendly = false,
  fishing = false,
  pirateIslands = false,
  transport = false,
  explorer = false,
  onChange,
}: {
  value: boolean;
  disabled?: boolean;
  citiesKnights?: boolean;
  cloth?: boolean;
  friendly?: boolean;
  fishing?: boolean;
  pirateIslands?: boolean;
  transport?: boolean;
  explorer?: boolean;
  onChange: (enabled: boolean) => void;
}) {
  return (
    <fieldset className="catan-helper-options" disabled={disabled}>
      <label>
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
        />
        事件牌 · 本站牌组
      </label>
      <p className="muted small">
        {explorer
          ? "探索者按2025官方组合说明只取生产点数，忽略全部事件文字；双人同样每回合一张，五六人仅①号抽牌。36张普通牌与新年重洗；金币补偿、7点海盗和独立鱼群骰保持原规则。"
          : "用抽牌代替生产骰，先完成事件，再按牌面生产。36张普通牌与新年重洗；双人每回合抽两张，五六人仅①号抽牌。"}
      </p>
      {citiesKnights && (
        <p className="muted small">
          {explorer
            ? "城市骑士保留独立红骰／事件骰，结算后按牌面点数生产。炼金术替代抽牌，牌堆不变。"
            : "城市骑士先执行事件文字，再结算独立红骰／事件骰，最后生产。炼金术替代抽牌；本站补充规则：贸易优势可随机偷资源或商品。"}
        </p>
      )}
      {fishing && (
        <p className="muted small">
          {explorer
            ? "湖泊和渔场按牌面点数产鱼，忽略全部事件效果；满额换鱼后再处理引水渠。鱼不影响未获资源的金币补偿，也不参与7点弃牌。"
            : "湖泊和渔场按牌面点数产鱼；瘟疫不减少鱼筹码。事件先完成，满额换鱼后再选择金矿资源或引水渠补偿；鱼不参与资源赠送、偷取或7点弃牌。"}
        </p>
      )}
      {pirateIslands && (
        <p className="muted small">
          本站补充规则：事件牌只决定生产，另掷两颗舰队骰并取较小值决定巡航和攻击；事件完成后先结算舰队奖励，再生产。强盗逃跑没有效果。
        </p>
      )}
      {transport && (
        <p className="muted small">
          运输：地震损坏的道路花2移动点，经过对手道路仍付路费；7点弃牌后移动蛮族，强盗逃跑无效果。抽到2或12仍执行事件，不重抽；金币和货物不参与资源赠送、偷取或弃牌。
        </p>
      )}
      {friendly && (
        <p className="muted small">
          友善保护仍适用于7点的强盗和海盗；强盗逃跑回沙漠且不偷牌。冲突、贸易优势属于卡牌偷牌效果，不受友善保护。
        </p>
      )}
      {cloth && (
        <p className="muted small">
          本站补充规则：强盗逃跑只进入布匹地图的大岛沙漠，没有合法沙漠则移到场外，不偷牌、不移动海盗。
        </p>
      )}
      <details>
        <summary>牌组说明</summary>
        <p className="muted small">
          本站采用已交叉核对的旧版点数与事件配比；尚未核实与2025实体牌表完全相同。
          {explorer ? "探索者忽略事件文字。" : "按2025事件效果执行。"}
          关闭后仍用原来的骰子。
        </p>
      </details>
    </fieldset>
  );
}
