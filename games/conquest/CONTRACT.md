# 征服时代接口（schema 1）

游戏 ID `conquest`，2～8 个席位，自由争霸。按旧版 Domination 执行代码，最后一个存活势力获胜，不要求继续清扫无主中立地区。世界、欧洲、美国、中国共用同一规则，前端仅加载当前 `maps/{mapId}.json`。地图数据和摘要定义见 [地图契约](maps/CONTRACT.md)。

## 创建选项

```ts
type Options = {
  mapId: "world" | "europe" | "usa" | "china";
  difficulty: "easy" | "normal" | "hard";
  turnSeconds: 60 | 120 | 180;
  fog: boolean;
};
```

默认 `world / normal / 120 / false`。人数由平台容量决定，初始地区由私有对局种子从全图随机选择，玩家之间不得陆地或海路相邻。已有存档和等待房间的显式地图不被默认值覆盖。固定自由争霸和无外交，不提供未实现的同盟选项。

## 视图

```ts
type Province = {
  id: string;
  owner: number;
  troops: number;
  mobile: number;
  town: boolean;
  fort: boolean;
  tower: boolean;
  population: number;
  economy: number;
  morale: number;
};
type Player = {
  seat: number;
  color: string;
  gold: number;
  income: number;
  upkeep: number;
  economySpending: number;
  movement: number;
  king: string;
  territories: number;
  troops: number;
  submitted: boolean;
  eliminated: boolean;
  deadline: number;
  tax: number;
  morale: number;
  economyRate: number;
  militaryRate: number;
  mp10: number;
  lostRound: number;
};
type Order = {
  id: number;
  type: "move" | "recruit" | "build" | "disband" | "tax";
  from?: string;
  to?: string;
  province?: string;
  amount?: number;
  king?: boolean;
  building?: "fort" | "tower" | "town";
  value?: number;
};
type Event = {
  id: number;
  type: string;
  from?: string;
  to?: string;
  province?: string;
  seat: number;
  amount: number;
  captured: boolean;
  at: number;
  until: number;
  displayUntil?: number;
  applied?: boolean;
  skipped?: boolean;
  before?: Province;
  after?: Province;
  text: string;
};
type Controls = {
  canPlan: boolean;
  canSubmit: boolean;
  canUnsubmit: boolean;
  canPause: boolean;
  canResume: boolean;
  canResign: boolean;
  canBuildTower: boolean;
  recruitCost: number;
  fortCost: number;
  towerCost: number;
  townCost: number;
  seaCost: number;
  moveCost: number;
  recruitMovement: number;
  fortMovement: number;
  towerMovement: number;
  townMovement: number;
  maxOrders: number;
};
type View = {
  session: string;
  epoch: number;
  round: number;
  phase: "planning" | "resolving" | "paused" | "finished";
  mapId: string;
  mapVersion: number;
  mapSHA256: string;
  options: Options;
  provinces: Province[];
  players: Player[];
  orders: Order[];
  projected: Province[];
  availableGold: number;
  availableMovement: number;
  pauseVotes: boolean[];
  resumeVotes: boolean[];
  events: Event[];
  controls: Controls;
  winner: number;
  log: string[];
};
```

`owner == -1` 表示中立，`winner == -1` 表示尚未结束。`king` 是所在地区 ID，界面用皇冠区分君主，不使用席位编号装饰。

`income`、`upkeep` 和 `economySpending` 是按当前局面计算的经济估算，不代表上一回合已到账流水。金币使用整数，`movement` 是行动点，内部 `mp10` 使用十分之一点的整数单位，计算、费用和结转保持原样。界面「可行军 N 次」为 `floor(availableMovement / moveCost)`，仅用于显示完整行军次数，不改变点数；计划条数独立使用 `orders.length`。

`orders` 仅在规划或暂停阶段返回本人的计划；观众、结算和结束阶段为空。`projected` 在规划或暂停阶段反映本人的预留资源和计划兵力，不包含任何敌方计划；结算和结束阶段只反映已经发生的实际状态。己方预计到达的部队可增加目的地的 `troops`，但不增加该地本轮可用的 `mobile`，不能将尚未到达的部队再次规划移动。君主每轮只可随军一次。

## 信息可见性

默认公开地图显示各地区当前兵力。开启战争迷雾时，仅本人地区和己方瞭望塔的陆地邻省可见；未知地区的 `troops`、`mobile`、`population`、`economy`、`morale` 为 `-1`。君主位于城镇时，沿用旧版规则公开王冠位置但不公开其未知兵力；位于其他不可见地区时，君主位置为空字符串。某势力存在不可见地区时，其总兵力为 `-1`；领土归属和领土总数仍公开。

其他席位的金币、经济估算、税率、士气和行动点始终以 `-1` 隐藏，不通过 `projected` 或事件附带快照泄漏。迷雾下观众不获得任何席位的私有视野。当前界面应将未知值显示为未知，而不是当成零兵力；本人负金币是实际负债，不能误当作隐私占位。

事件只向实际能观察到其相关地区的席位或行动者提供。未知地区的事件 `before/after` 不返回；本人亲历行动的出兵数、地点和占领结果可用于后续判断。电脑只能通过此视图选择正常动作，不能读取隐藏兵力、敌方计划、随机源或待执行队列。

## 动作

每个动作必须携带 `{session, epoch}`。服务端使用平台认证后的 `Context.Actor` 校验身份；前端 `act` 附加平台版本与 nonce。

| 动作 | 字段与含义 |
| --- | --- |
| 行军 | `{type:"move", from, to, amount, king?:boolean}`；沿明确陆地邻接或海路移动、攻击；君主随军时 `king:true` |
| 招募 | `{type:"recruit", province, amount}`；本地区人口相应减少 |
| 建设 | `{type:"build", province, building:"fort"|"tower"|"town"}` |
| 解散 | `{type:"disband", province, amount}`；部队返回本地区人口，不直接退款 |
| 税率 | `{type:"tax", value}`；0～100，按 10 递增 |
| 撤销 | `{type:"undo", id}`；删除本人指令，并删除因此失效的后续依赖指令 |
| 提交 | `{type:"submit"}`；所有存活席位提交后开始结算 |
| 撤回提交 | `{type:"unsubmit"}`；结算前且原操作窗口仍有效时重新规划，不延长时限 |
| 暂停投票 | `{type:"pause", enabled:boolean}`；仅规划阶段可发起 |
| 恢复投票 | `{type:"resume", enabled:boolean}`；仅暂停阶段可发起 |
| 认输 | `{type:"resign"}`；本人领地恢复中立，若只剩一个存活势力则立即结算 |

每轮最多保留 128 条计划，反复编辑累计 100,000 次时停止追加，但仍可撤销或提交，不生成不可恢复的存档。同一 `epoch` 内其他玩家的操作不会使本人的合法动作失效，但每次都重验自己的资源和计划。开始结算、新轮、暂停、恢复和结束会更新 `epoch`。过期真人不能继续规划、提交或撤回提交，托管通过平台受控席位接口接续；取消托管仅恢复该席过期窗口，不改变他人期限。

## 原版规则与执行顺序

规则证据来自官方旧版桌面 Age of Conquest III 3.0.5318 的执行代码及原类探针，详见[旧版规则研究](../../docs/research/conquest/legacy-rules.md)。尚未证明与早期 iOS World 逐项相同。

- 开局每个势力一个带城镇和瞭望塔的地区，500 兵、5,000 金币、3.2 行动点；地区人口 25,000、经济 50,000。税率 10%，士气 90%，经济与军费支出率固定 80%。
- 所有计划先统一排序：招募、建设、税率先执行，解散随后，移动最后。移动按该席指令序号排序，同序位按结算开始时领土、兵力、金币从多到少排序，再按势力索引排列。网络请求到达时间不作为战斗优先级。
- 招募每兵 1 金币、1 行动点，单次不超过该地人口一半；解散消耗 0.1 行动点。移动消耗 1 行动点，海路额外每兵 2 金币。
- 工事 5,000 金币／0.5 点；瞭望塔 2,500 金币／0.3 点；城镇 75,000 金币／5 点。公开模式不提供造塔，迷雾模式才允许。
- 执行前重验来源归属、兵力、金币和行动点。来源失守的命令跳过；当前兵力少于请求量时按实际兵力执行。海运必须先满足请求出兵数的费用门槛，再按实际人数扣费；招募请求超过当前金币或人口上限时整条跳过。
- 战斗使用原版探针确认的强度指数与分步取整。工事额外 80% 防御、城镇 50%、君主攻防 30%。相等攻防时守方留地、守军为零。君主随军进攻失败时仍在出发地。
- 君主所在地区被占时，剩余军队降为 70%，正金币转交占领者；剩余地区不足 3 块则灭国，否则保留 `max(1, floor(remaining * 0.18))` 块并重立君主，其余割让。保留次序按兵力、经济、人口和地图稳定索引决定。
- 回合末依次更新人口增长与陆地迁移、经济、军队、税率造成的士气变化、行动点与收支；最终金币仍为负时再减 1 士气。此顺序不能用一次简单加减替代。行动点仍按 `mp10 = min(10000, mp10 / 2 + int((2.2 + territories^0.63) * 10))` 恢复，除法按存储整数精度执行，小数可参与结转。

电脑策略为本项目的独立启发式策略，难度只改变决策；不复制原包部分难度的额外金币。地图是独立制作的地理数据，原版地图文件不作为发布依赖。

## 演出、暂停与存档

`phase:resolving` 时禁止修改计划。每条操作都有服务端时间线：行军 `until-at=1000ms`，其他操作 `900ms`；到期才提交资源、兵力和归属变化，并在同一事件 ID 上设置 `applied`、补充 `after`。随后以 `displayUntil` 保留至少 `500ms` 的结果停留，再启动下一条命令。失效计划使用 `skipped` 和说明，不能显示为成功行动。最后一步结果停留结束后才显示胜负。

事件保留最近 64 条。客户端按真实时间显示位置、路线、行动者、人数和结果，不提前扣兵或换色；减少动态效果时保留文字与静态标记。首次载入或重连不重播旧动作。旧存档的已有计划、点数、出生点和当前截止时间保持不变，不重新随机。

暂停与恢复均需所有存活真人同意，电脑视为同意但不会主动恢复。暂停使用平台显式 `Match.Paused`，清空宿主行动和系统期限，在游戏状态内保留所有未淘汰席位的剩余预算，包括已提交者。恢复还原各席预算；已过期的窗口不会因暂停重新获得完整时限。已提交者随后撤回提交也使用同一份预算。

存档绑定 schema、地图 ID、地图版本与 SHA-256。读取时验证地区和势力范围、君主归属、指令字段、待执行队列席位、事件时间线及阶段与平台信封的一致性，损坏存档应返回错误，不能导致计时回调越界。明确暂停的房间保留 30 天；普通闲置关闭、战绩与计分由平台管理。

电脑策略可依据本席可见战报，估计同一敌方领地补充上一战损失的可能性，并受当前可见人口限制；不读取敌方私有计划或金币，不修改战斗和资源计算。
