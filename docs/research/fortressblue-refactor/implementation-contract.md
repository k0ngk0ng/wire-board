# 疯狂坦克第一阶段服务端接口

顶层 `phase` 保持 `select | playing | finished`。`playing` 内新增可选 `resolution`，没有该字段时为普通行动阶段。以下时间均为服务端绝对 Unix 毫秒；旧存档没有新增字段时视为旧炮弹已完成，不重播。

```ts
type Resolution = {
  stage: 'flying' | 'settling';
  shotId: number; // 0 表示主动移动引发落地
  at: number;
  impactAt: number;
  settleAt: number;
  falls?: {seat:number;fromX:number;fromY:number;toX:number;toY:number;at:number;endAt:number}[];
};
type Arc = {
  path: {x:number;y:number}[];
  pathTimes?: number[]; // 与 path 一一对应，相对 shot.at 的毫秒
  impact: {x:number;y:number};
  hit: number;
  impactAt?: number;
  outOfBounds?: boolean;
};
type Shot = {
  id:number;seat:number;weapon:string;arcs:Arc[];at:number;
  impactAt?:number;settleAt?:number;
  blasts: Blast[]; // 发射时为空，命中后才写入
  craters?:Crater[];
};
```

`flying` 时，玩家 HP、位置、存活状态和地形保留发射前值。`impactAt` 处理命中、伤害、弹坑及命中音效事件，进入 `settling`。`falls` 在该阶段发布，不含未来伤害；前端在 `at` 至 `endAt` 之间插值绘制。各落地节点才发布最终位置、坠落伤害及死亡。`settleAt` 后完成胜负判定或开启下一回合。

`resolution` 存在时，`canMove/canFire` 为 false，match actors/attention/timeoutActors 为空，行动 deadline 为 0，systemDeadline 为下一阶段节点。下一席位操作期限从实际可操作时刻开始。Restore/Reclaim 保留阶段和时间；不刷新其他席位期限。

`Blast` 保留原字段并增加可选 `hits: {victim:number;damage:number;kind?:string}[]`；每个伤害目标仍有独立 `Event`。出界炮弹不在地图边界产生替代爆炸。`shot` 事件在发射时产生，`explode/hit/death/fall/turn/finish` 在各自发生时产生。前端统一按 `Event.at` 播放与显示，不能按收到同一快照的顺序提前播放未来效果。

本阶段不改变固定行动顺序，不自创 delay 算法、炮种或原版精确常数。弹道速度、伤害等待原版证据对齐。

## 原版证据对齐后的接口补充

以下补充覆盖前文第一阶段中「发射时立即产生 shot 事件」的描述。

- `Shot.launchAt = Shot.at + 334`，`Resolution.launchAt` 相同。初始请求产生 `fire_ready`；到 `launchAt` 才产生 `shot`。炮弹在出膛前不可见，发射音也以该事件时间为准。`arcs[].pathTimes` 仍相对 `Shot.at`，首点为 `334`，命中时间同样顺延。旧记录无 `launchAt` 时使用 `at`。334 ms 是项目按 30 Hz、每两 tick 一帧及源第 5 帧出膛所作的显示校准，不表示原版运行频率已获证明。
- 各 `Tank/TankView.specialUnlimited` 为布尔值；新局为 true，`special = -1`。旧存档没有该标志时继续解释旧有限库存。服务器接受归一化 `power` 0～100，角度按该车型 `MinAngle`～`Angle` 验证。
- 各 `Tank/TankView.pendingShotEffect = 'heal'` 表示能量补充弹已经装填，使用道具不会立即补 HP 或移动力。下一发 `Shot.weapon = 'healing'`；只有命中时才对范围内存活的双方坦克回复 30% 最大 HP，不挖坑、不造成伤害。`Event.type = 'heal'`、`target` 为受益席位、`amount` 为实际回复量、`at` 为命中时间；`Blast.kind = 'heal'`、`terrain = false`，`hits[].healing` 为实际回复。
- 防护罩按一次敌方攻击消耗，不随回合倒数；护盾事件为 `shield`，实际生命不减少。强化和穿风标记按下一次攻击消耗。穿风不消除普通风；已确认的是穿龙卷风并增加 50% 威力。本轮没有额外引入龙卷风地形。

地面实体由 `View.hazards` 提供：

```ts
type Hazard = {
  id:number; kind:'fire'|'poison'; x:number; y:number; r:number;
  at:number; endAt:number; nextTickAt:number; source:number;
};
```

火和毒固定在落点的实际地面，不再给新命中目标添加携带三回合的状态。每秒在范围内检查所有存活坦克，离开区域或位于不同高度层时不受伤害；后续地形破坏移除原支撑时，该实体消失。伤害事件沿用 `type='status'`，`text='fire'|'poison'`，`target` 为受伤席位，`amount` 为实际扣除 HP，`at` 为该次 tick 的绝对时间。其伤害在其他炮弹飞行中继续，不等待玩家回合。刷新及恢复保留 `nextTickAt/endAt`。

落地途中位置按 X 线性、Y 的进度平方计算，与前端一致。地面效果按该时刻位置检查，途中死亡时立即公布该位置，并截短对应 fall，不回到落地起点。同一时间节点先处理炮弹命中／落地，再处理地面接触伤害。迟到的计时器按节点顺序处理，完成后才给下一名玩家完整可操作时间。

火半径 40、寿命 8 秒、每跳 18，毒半径 50、寿命 12 秒、每跳 25，以及上下 24／12 世界像素的地面接触带，均为明确的项目参数；原版证据确认了地面持续效果，尚未确认这些精确常数。旧存档携带的 poison/burn 状态仅作兼容，在所属玩家获得行动时推进；新火／毒炮弹不再创建该类携带状态。


## 最终边界补充

疯狂坦克显式开启 `Module.PreciseTimers`。宿主每 50 ms 检查其到期系统事件；这不是实时控制帧，普通模块仍保持一秒系统事件时钟，玩家超时和电脑调度不变。

射手在 `launchAt` 前死亡时，服务端取消待发炮弹，写入 `Shot.cancelledAt` 和 `shot_cancelled` 事件，清除对应结算阶段；已经出膛的炮弹继续飞行。同一时间节点依然先处理出膛，再处理地面伤害，因此恰在 `launchAt` 死亡的炮弹视为已经出膛。`cancelledAt` 是历史诊断标记，前端不得据该 `LastShot` 绘制炮弹、爆炸、Fire 或追踪镜头。该死亡边界是项目明确选定的规则，原版服务器语义尚未确认。

非致命受击不打断 Fire 的出膛帧：以当前图集时间保留到 `launchAt + 67 ms`，随后可显示受击或下落；不以完整 Fire 时长阻止已经发生的坠落。使用道具时，以服务端 `item` 事件播放原生 Item 动作，优先级低于发射、受击和移动。

新局在原出生槽附近选择外部支撑表面，检查与当前移动模型一致的车身净空、支撑、可移动方向及有效射界，并在全部席位放置后再次检查队友阻挡。此修正不重写原版地图素材，也不在恢复已有对局时移动坦克。完整覆盖为 17 图 × 4 种人数 × 14 种车型，共 952 场、4,760 席位；它不等于穷举全部混合车型排列。

## 准备取消与出生布局

`Shot.cancelledAt?: number` 为已取消待发炮弹的绝对时间。准备阶段射手死亡产生 `shot_cancelled`，清除待发射 `resolution`，客户端不再绘制该历史炮弹的准备、飞行或爆炸。已在 `launchAt` 出膛的炮弹不会因射手死亡而取消；同一时刻按出膛先、地面伤害后处理。

安全出生只在新局完成选车时运行。目录坐标是水平偏好，实际位置选在外部顶面，并验证移动碰撞体／炮口净空、履带支撑、可移动方向及不会自伤的实际射界。全队布置后再次检查互相遮挡，不改变恢复对局的现有坐标。详见 [backend-stage3-verification.md](backend-stage3-verification.md)。
