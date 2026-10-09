import type { CatanRuleContext } from "./catan-rule-context";
export function CatanCaravansAttackRules({ info }: { info: CatanRuleContext }) {
  if (!info.caravans || !info.attack) return null;
  return (
    <section aria-label="商队与蛮族组合规则">
      <h4>
        商队＋蛮族进攻{info.citiesKnights ? "＋城市与骑士" : ""} · {info.target}{" "}
        分
      </h4>
      <p>
        以沿海水源替换沙漠。
        {info.fiveSix
          ? "本站五六人配方：两处水源、五个有效出口、33 辆马车，沿用蛮族地图数字和配对回合。"
          : "水源有两个有效出口，共 22 辆马车。"}
        蛮族不阻挡马车放置。
      </p>
      <p>
        本回合建造村庄或升级城市后，回合末触发一次商队投票。本站结算顺序：先完成骑士移动和战斗，再出价、投票、放马车，最后切换玩家；建设胜利或战斗胜利会立即结束游戏。
      </p>
      <p>
        每张{info.citiesKnights ? "木材或砖块" : "羊毛或粮食"}
        算一票，出价公开且不能撤回。票数过半者选位置，否则由投票决定；平票由最高出价者按回合顺序决定。出价全部归还银行。
      </p>
      <p>
        马车沿队头延伸，不能分叉或重复占边；己方道路有马车时按两段计入最长路线，建筑连接至少两辆马车加
        1 分。建筑被征服时，其建筑分与商队加分都暂时失效，解放后恢复。
      </p>
      {info.two && (
        <p>
          双人每轮尽量放满、最多两辆。唯一最高出价者选择两条不同商队；平票时双方各放一辆，第二辆可延续第一辆。中立建筑不触发投票；规则或位置不足时沿用已标注的双人少放补充规则。
        </p>
      )}
      {info.citiesKnights && (
        <p>
          本站三模块组合使用道路骑士和进步牌，15
          分获胜；商队出价改用木材或砖块，不使用海上蛮族和强盗。发明仅交换允许的内陆数字。
        </p>
      )}
    </section>
  );
}
