import type { CatanRuleContext } from "./catan-rule-context";

export function CatanRiversAttackRules({ info }: { info: CatanRuleContext }) {
  if (!info.rivers || !info.attack) return null;
  return (
    <section aria-label="河流与蛮族组合规则">
      <h4>河流＋蛮族进攻{info.citiesKnights ? "＋城市与骑士" : ""}</h4>
      <p>
        共用金币，唯一最富者加 1
        分，最贫者不扣分。不使用强盗；起始河岸村庄和城市各领 1
        金币，后续河岸村庄或道路领 1 金币，建桥领 3 金币，升级城市不重领。
      </p>
      <p>
        桥梁费用为 1 木材＋2 砖块，每家最多 3
        座，不占道路库存，计入最长路线；已征服地块旁不能新建桥。骑士可通过桥位。每回合最多两次用
        2 金币购买普通资源；金币可交易，也可按兑换比例出售
        {info.citiesKnights ? "资源或商品" : "资源"}
        换金币，金币供应不足时继续记账。
      </p>
      <p>
        {info.fiveSix
          ? "本站五六人组合地图：保留三条河流、两座城堡及独立固定数字；使用配对回合。"
          : "使用官方组合地图；同一地块的 2 与 12 均触发生产或蛮族登陆。"}
      </p>
      {info.two && (
        <p>
          真人建桥后代建中立桥；没有合法中立桥位或桥梁库存时改建道路。中立势力不领金币、不参与财富比较。
        </p>
      )}
      {info.citiesKnights ? (
        <p>
          本站三模块适配：13
          分获胜。发明只交换合适的内陆数字；外交不能拆桥，移除河岸道路需归还 1
          金币。不使用海上蛮族和城市劫掠保城操作。
        </p>
      ) : (
        <p>
          本站补充规则：城堡骑士当前三步内无合法落点，且无法支付粮食到达五步内的合法落点时，可本回合暂留；其他骑士继续行动和战斗。
        </p>
      )}
    </section>
  );
}
