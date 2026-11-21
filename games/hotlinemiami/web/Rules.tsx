import { characterGuide, mapGuide, weaponGuide } from "./content";

export function Rules() {
  return <div className="hm-rules">
    <p>2 至 4 人自由混战。房主从 {mapGuide.length} 张地图中选图与时长，每人选择 {characterGuide.length} 个造型之一和 {weaponGuide.length} 类武器之一，准备后开战。击杀加 1 分，自杀减 1 分；时间到得分最高者获胜，最高分并列时整场和局，所有参与者积分变化为 0。</p>
    <details className="hm-rule-group">
      <summary>地图与路线</summary>
      <p>地图保留原版房间区域，出生点和武器点为混战调整。关闭的门、墙体和家具挡住路线与射线；玻璃被子弹或投掷击碎后，通路会改变。</p>
      <dl className="hm-guide-list">{mapGuide.map(map => <div key={map.id} data-hotlinemiami-rule-map={map.id}>
        <dt>{map.name}</dt><dd>{map.description}<span className="hm-guide-recommendation">推荐 {map.recommendedPlayers} 人。</span></dd>
      </div>)}</dl>
    </details>
    <details className="hm-rule-group">
      <summary>武器与弹药</summary>
      <p>每次复活携带自己在房间选择的起始武器。枪械弹药有限，不能装填；弹药用尽时，可投掷空枪后拾取其他武器，也可直接拾取换枪。拾起掉落枪械会保留它的剩余弹药，不能靠反复拾取补满。近战武器不消耗弹药。</p>
      <h4>枪械</h4>
      <dl className="hm-guide-list">{weaponGuide.filter(weapon => weapon.group === "gun").map(weapon => <div key={weapon.id} data-hotlinemiami-rule-weapon={weapon.id}>
        <dt>{weapon.name}</dt><dd>{weapon.description}</dd>
      </div>)}</dl>
      <h4>近战</h4>
      <dl className="hm-guide-list">{weaponGuide.filter(weapon => weapon.group === "melee").map(weapon => <div key={weapon.id} data-hotlinemiami-rule-weapon={weapon.id}>
        <dt>{weapon.name}</dt><dd>{weapon.description}</dd>
      </div>)}</dl>
      <p>小刀与武士刀投掷可直接击杀；其他武器投掷使对手倒地，斧头与砍刀也遵循这条击倒规则。投掷后手中变为徒手，可以近身挥拳击倒对手，也可以捡回落地武器。</p>
    </details>
    <details className="hm-rule-group">
      <summary>角色造型</summary>
      <p>所有造型的移动速度、生命与碰撞体相同，使用同一武器池。面具和 Boss 身份只改变外观，没有额外属性加成。可以重复选择造型，使用轮廓颜色和姓名识别彼此。</p>
      <dl className="hm-guide-list">{characterGuide.map(character => <div key={character.id} data-hotlinemiami-rule-character={character.id}>
        <dt>{character.name}</dt><dd>{character.description}</dd>
      </div>)}</dl>
    </details>
    <details className="hm-rule-group">
      <summary>键鼠、触屏与手柄</summary>
      <ul className="hm-rule-controls">
        <li><strong>PC 键鼠</strong><span>WASD 移动，鼠标瞄准，左键或 J 攻击，右键或 Q 投掷，E 拾取，F 开关门，空格处决。按住攻击可持续射击或挥击。</span></li>
        <li><strong>手机触屏</strong><span>左摇杆移动；右摇杆轻推瞄准，向外推动攻击，松开停止。中间按钮负责拾取、投掷、处决与开门，多指可以同时操作。</span></li>
        <li><strong>实体手柄</strong><span>左摇杆移动，右摇杆瞄准，右扳机攻击。拾取、投掷、处决与开门沿用平台动作映射；可在统一控制器设置中修改键位。</span></li>
      </ul>
    </details>
    <details className="hm-rule-group">
      <summary>交战、复活与托管</summary>
      <p>有效枪击和持械近战可直接击杀。徒手攻击与普通武器投掷使对手倒地；倒地期间无法移动和攻击，短暂等待后会起身。靠近倒地对手可以处决，处决时自己也会短暂锁定动作，仍可能被第三人击杀。</p>
      <p>死亡约 2 秒后复活，出生保护约 0.8 秒。主动攻击或投掷立即解除保护。复活保留所选造型与起始装备，不重置其他人的分数或比赛时钟。</p>
      <p>可通过平台添加电脑或主动启用、取消托管。断线不会自动托管；重连与手动接管继续同一局，不补播后台输入。含电脑或托管的对局保留结果与历史，不计竞技积分。</p>
    </details>
  </div>;
}
