# 疯狂坦克重构独立审查

本次审查先读原版证据、实现契约、第二阶段验收说明及当前差异，再通过公开游戏模块执行针对性诊断。未修改游戏实现，未重跑完整测试矩阵。以下问题是审查时工作树的实测结果；后续修复须在末尾追加复验，不能把本记录当作最终验收通过。

## 1. 已确证：部分出生点与新完整碰撞模型不兼容

**影响：高。** `stage-1` 的一个正常出生位置在开局即不能向任一方向移动，并有车身采样点嵌入地形。原始射击诊断只覆盖 `0..40°`，该子集均近距离自伤；坦克 1 实际合法炮角是 `0..90°`，因此这份修复前记录不能证明所有合法射击都会自伤，也不能证明不存在射击脱困路线。电脑最终完成对局不能证明出生条件正常。

复现：公开 `Module.Start` 创建 `stage-1` 四人局，四席通过 `Apply(select)` 选择坦克 1。开局后 3 号席（零基编号）位于 `(1266,924)`。为不让之前的炮弹改变地形，诊断只把独立存档的 `Current` 设置为该席位；地图、出生位置、生命、燃料、风、其他席位都来自正常开局。随后全部移动、开火、命中均使用公开 `Apply/OnTimer`。

- 向左、向右移动均返回「移动力不足或前方不可通行」。
- 诊断硬编码角度 `0..40`（仅占合法 `0..90` 的一部分）、整数力度 `0..100`、双向共 **8,282 发**，每发独立从未破坏的初始地形开始。
- 上述子集的每发命中都会使自身损失 **102～163 HP**。
- 上述子集的所有落点距离 `(tank.x, tank.y-26)` 不超过 **45.96670000952439** 世界像素；最远参数为朝右、角度 0、力度 0，落点 `(1308.2625,916.078125)`。
- 同一出生点也对应八人局 5 号席；六人局同样会使用这个蓝队出生槽。

修复前证据：仓库内 `.local/fortressblue-refactor/review/trapped-before.json`；当前 `trapped.go` 已改为读取车型实际炮角并只复验当前实现，`trapped.json` 是修复后的全范围结果，不是原始失败数据。初稿错误地把这个子集写成了「全部合法整数角度」，现已撤回这一范围声明与不可避免自伤的结论。未保留修复前的完整 Match 快照，因此未伪造原始存档进行全角度复验；保留的结果只证明 `0..40°` 子集。纠正来源见 [scope-correction.json](../../../.local/fortressblue-refactor/review/scope-correction.json)。

17 张地图、每图 8 个出生槽共 **136 个位置**的先行诊断还发现：

| 地图 | 八人局席位，零基 | 开局坐标 | 双向移动 | 说明 |
| --- | ---: | --- | --- | --- |
| stage-1 | 5 | (1266,924) | 均拒绝 | 车身嵌入；0..40° 子集射击自伤 |
| stage-3 | 4 | (1159,888) | 均拒绝 | 有 7 个简化车身采样点在地形内；可向一侧远射 |
| stage-5 | 3 | (375,712) | 均拒绝 | 有 6 个车身采样点在地形内；可向一侧远射 |
| stage-5 | 4 | (1159,708) | 均拒绝 | 有 4 个车身采样点在地形内；可向一侧远射 |

另有 `stage-17` 的 1、6 号席各有一个车身采样点在地形内，但实际能移动爬坡，不能据此称为完全困死。

`stage-17` 的 0、7 号席虽然能移动，却位于低层空间。坦克 1 从原始出生点分别向两侧持续执行公开移动直到阻挡或燃料结束，所得区间为 `x=11..85.5,y=860` 与 `x=1454..1526,y=860`。每个可达位置双向检查角度 `0..40°`、间隔 5°，力度 `0..100`、间隔 25 的组合，非出界炮弹最远仅约 **93.83** 与 **97.00** 像素。这是出生射界仍需检查的证据，尚未枚举破坏地形后多个回合的脱困能力，不能称为永久不可脱困。

相关位置与根因：

- `scripts/fortressblue/prepare-original-assets.py:safe_ground` 注释写「highest」，实际从 `rows-1` 向上扫描，先选最低可用层；仅检查中心列上方 5 格，即 20 像素净空。
- `games/fortressblue/server/engine.go:beginPlaying` 只从旧出生高度向下重新求支撑，不检查整车与炮口的净空。
- `games/fortressblue/server/timeline.go:walkGround` 现在检查横向 ±10、纵向 12／20／28 的车身采样点。新移动规则与旧出生验证条件不一致。
- `games/fortressblue/server/physics.go:bulletMaskFor` 已改为完整实体地形。旧出生点在原本忽略浮台的炮弹模型下更容易被现有测试漏过。

建议：新局出生时以同一车身、支撑与炮口模型选择安全位置，并验证可用射界；保留旧对局已有位置，避免恢复时把玩家传送到别处。先增加公开模块回归，至少覆盖 17 图全部出生槽、合法车型炮角，以及上表实际失败位置。不能简单恢复炮弹穿透可见平台来使这些测试通过。

## 2. 已确证：准备阶段死亡后仍会出膛

**影响：中；原版语义未知。** 服务端正确按时间给地面效果扣血，但待发射炮弹与射手死亡没有联动。下面的数据来自有队友仍存活的四人局，不涉及提前结束整场比赛。

公开模块夹具在 `stage-16` 正常开局后，将 0 号席 HP 设为 18，并在其实际地面生成已有地火，下一跳为开火请求后 100 ms。随后通过公开开火和计时器推进：

| 相对请求时间 | 实际结果 |
| --- | --- |
| 0 ms | `fire_ready`，进入准备阶段 |
| 100 ms | 地火 `status` 造成 18 点伤害；HP 0、`alive=false`，产生 `death` |
| 334 ms | 仍产生 `shot` |
| 4043 ms | 仍产生 `explode` |
| 4493 ms | 进入下一回合 |

证据：`.local/fortressblue-refactor/review/prepare-death.go`、`prepare-death.json`。

位置：`games/fortressblue/server/timeline.go:processResolution` 到 `launchAt` 只检查 `PendingShot.Launched`，不检查射手是否仍存活；`hazards.go:hazardDamage` 处理死亡及正在落地轨迹，但不取消未出膛炮弹。

建议采用明确的项目规则：未出膛时死亡，取消待发射炮弹；已经出膛则继续既有弹道。这里没有原版服务端证据，不能声称原版一定采用这一规则。取消时还应验证没有残留炮弹、命中、地形变化、音效或过期 `resolution`，且同时间所有伤害处理后再判胜负。

**相关渲染风险，仅代码推断：** `render.ts` 中 `damage` 优先于 `fire`。若准备期在 100 ms 受到非致命地火伤害，角色先显示持续 667 ms 的受击动作，但服务端仍在 334 ms 出膛；于是画面不会在 Fire 第 5 帧发射。应明确射击准备与非致命受击的动作优先级，并在真实浏览器按帧验证。此条尚无浏览器实测，不能与上面的死亡复现混为同一证据等级。

## 3. 已检查的契约及证据边界

以下为源码审查结论，不替代负责完整套件与浏览器验收的任务：

- `launchAt`、`impactAt`、逐车 `Fall.endAt`、地面效果 `nextTickAt` 均使用绝对毫秒。`processWorldTimeline` 依时间节点依次处理；同节点先炮弹／落地，再地面效果，然后判定结果。`beginFalls` 不提前扣除落地伤害。
- HP、存活状态与结果来自服务端当前状态，`PlayerSummary` 没有前端提前预测生命。飞行时锁定 actors、attention、timeoutActors；接管不重写时间轴或他人期限。
- `Reclaim/Restore` 保留进行中的发射和落地节点。没有新字段的历史已完成炮弹不应重播。坦克 13 的旧最大生命 990 有有限弹药存档兼容分支；其余 13 种车的旧 HP 与新原表相同。
- 充能由单个 source/token 管理；满力回调与 release 消耗同一个 latch，`firedTurn` 防止同回合二次提交。隐藏页面、失焦、切阶段、失去控制权会取消输入。`pointercancel` 与 `lostpointercapture` 不走开火提交。
- `internal/server/server.go` 当前用 `PreciseTimers` 为选择该能力的非实时模块提供 50 ms `SystemDeadline` 时钟，疯狂坦克已经启用；普通游戏保持原节奏。没有再把该项列为未修问题。
- 完整角色帧的 atlas 打包保留 `w/h/ax/ay` 和原始左右方向，渲染位置为 `(tank.x-ax,tank.y-ay)`，没有二次缩放角色帧来适配自创碰撞体。前景按 `map.offY` 绘制，服务端使用同一完整 mask。审查期间坡度帧选择已接入 `render.ts`，所以不把最初读取时的未接入状态列入最终缺陷。
- 原版每两次更新推进一动画帧、Fire 第 5 帧出膛有静态证据；30 Hz、334 ms、持续伤害数值等仍是项目校准。本文没有新增「原版精确物理已还原」的结论。

## 多弹时间轴专项复核

公开模块分别用全部 14 种坦克发射主炮和特殊弹，共 28 发，均只有 1 条 Arc，且 `arc.impactAt == shot.impactAt`。证据为 `.local/fortressblue-refactor/review/arc-count.go` 与 `arc-count.json`。当前 `resolveShot` 和仓库 HEAD 中的旧实现均只赋值 `[]Arc{arc}`，8／14 号坦克也没有多弹分支。

**仅潜在兼容风险：** `decode` 允许多条 Arc，人为构造不同 `arc.impactAt` 的存档时，前端会按各条时间移除炮弹，而 `impactShot` 在统一 `Resolution.ImpactAt` 应用所有伤害。当前没有正常公开操作或已发布实现能生成这种存档，不能把它列成现行可玩性阻塞。若以后确认原版多弹机制并实现，必须同时补逐条命中节点、逐条地形与伤害结算；当前不要自行给 8／14 号车发明多弹规则来覆盖这一分支。

## 复现命令与产物

先列出的失败场景位于 `.local/fortressblue-refactor/review/plan.md`。脚本仅创建独立内存对局，不访问生产、不写游戏实现。

```sh
source scripts/env.sh
go run .local/fortressblue-refactor/review/diagnose.go
go run .local/fortressblue-refactor/review/trapped.go
go run .local/fortressblue-refactor/review/reachable.go
go run .local/fortressblue-refactor/review/prepare-death.go
go run .local/fortressblue-refactor/review/arc-count.go
```

输出分别为同目录 `spawns.json`、`trapped.json`、`reachable.json`、`prepare-death.json`。`source-sha256.json` 记录审查实现的文件哈希。全部构建缓存与临时目录使用 `scripts/env.sh` 限制在仓库内；没有保留独立编译可执行文件，没有创建浏览器 profile。

## 修复后复验

负责实现的任务增加运行时安全出生和未出膛取消后，本审查重新执行原诊断。以下先记录第一次修复的中间态结果，再单列最终全炮角复验，不能将不同范围和不同实现的数值混为同一次测试：

- 第一次修复中间态（尚未包含最终车身碰撞范围调整）的 17 图 × 8 槽、坦克 1 共 136 个正常开局位置，车身嵌入由 6 处降为 0，双向移动均拒绝由 4 处降为 0。对比文件为 `spawn-comparison.json`。这不是对全部车型、人数矩阵的替代；当前最终实现由负责后端的 952 项地图／人数／车型公开模块矩阵覆盖，本审查没有把中间态 136 槽结果冒称该最终矩阵，也未重复运行矩阵。
- 第一次修复中间态的 `stage-1` 四人局原失败席位出生于 `(1246,728)`。按与修复前相同的 `0..40°` 子集再次枚举 8,282 发，子集内最小自身伤害由 102 变为 0；其中 2,971 发满足「非出界、无自身伤害、落点距离至少 100 像素」。最远符合条件落点距离约 1,245.91 像素。该同范围对比保存在 `trapped-after-0-40.json`；它不是完整 `0..90°` 枚举。这项距离条件不等于每发都命中敌方。
- 同一次中间态复验中，`stage-17` 的原低层 0／7 号槽移至 `(56,504)` 与 `(1480,504)`，均能向两侧移动，原地已有长距离射界。
- 同一准备期死亡夹具在 100 ms 产生 `death`、`shot_cancelled` 并切换行动席位；继续推进旧 334／4043／4493 ms 节点，没有 `shot` 或 `explode`。服务端取消路径已独立复现通过。

修复前结果保留为 `*-before.json`，同名 JSON 记录各诊断最后一次执行结果，只有 `trapped.json` 已按最终实现重跑完整炮角；`trapped-after-0-40.json` 和 `source-sha256-after.json` 保留第一次修复中间态。不能再用修复前数据声称当前实现仍有对应服务端缺陷。

后续修复中曾发现、现已闭合的前端边界：新增 `cancelledAt` 必须阻止 `render.ts` 绘制保留下来的 LastShot 弹道；镜头已有 `resolution?.shotId` 前置条件，取消后不会继续追踪。非致命受击的第一次优先级修复只保护 `now < launchAt`，在 334 ms 恰好切回 Damage；通过直接执行 `tankPose`，333 ms 仍为 Fire 第 4 帧，334／350／400 ms 均为 Damage。该半开区间仍不能保证 Fire 第 5 帧与炮弹同时出现，已反馈负责 UI 的任务继续修复。最终真实浏览器的逐帧证据由浏览器验收记录证明。


## 前端边界最终闭合

根任务在后续修复后复核：`cancelledAt` 已由实际绘制与镜头共同过滤；Fire 的保护延续至出膛帧结束约 401 ms，不延误 514 ms 起始的落地。相关辅助测试先红后绿，且最终统一浏览器运行 `refactor-final-r2` 实测两种情况：致命地火在前摇中取消炮击，之后没有弹体、发射、爆炸或追弹镜头；非致命地火正常扣血和显示伤害数字，同时实际绘制保留原生 Fire 第 5 帧（含坡度偏移）。对应逐帧记录为 `cancelled-windup.json` 与 `nonfatal-windup.json`。本报告此前列出的两项前端待办已闭合，详见 [最终验收报告](verification.md)。


## 炮角范围纠正及最终实现复验

初始 `trapped.go` 硬编码 `0..40°`，初稿却误称「全部合法整数角度」。这是一项审查范围错误，相关全称结论已撤回；保留的修复前 JSON 只能支持这个子集，不能证明原出生位置没有其他可用射角。没有修复前完整 Match 快照，未尝试重建后冒称原始证据。

当前 `trapped.go` 从 `data.TankByID(t.TankID)` 读取 `MinAngle` 和 `Angle`。坦克 1 实际范围 `0..90°`，配合力度 `0..100`、双向共 **18,382 发**。当前最终实现的四人局该席位出生于 **(1266,732)**；其中 **7,615 发**满足非出界、无自身伤害、落点距离至少 100 像素，最远符合条件距离 **1,265.7341845716053** 像素，自身伤害最小值为 0。该结果与前一修复中间态的 8,282 发必须分开引用。

最终完整范围保存在 `trapped.json`，其中明确写入炮角、力度及步长；[scope-correction.json](../../../.local/fortressblue-refactor/review/scope-correction.json) 保留错误范围、撤回结论及证据对应关系；`source-sha256-final.json` 绑定当前实现。复现命令仍为 `source scripts/env.sh` 后执行 `go run .local/fortressblue-refactor/review/trapped.go`。没有修改产品代码。

此外，本审查直接执行当前 `tankPose/renderableShot`，独立确认 334／350／400 ms 均为 Fire 第 5 帧，401 ms 回到 Damage，514 ms 的落地仍为 Fall；已取消炮弹返回不可绘制。结果位于 `ui-boundary-after.json`，可执行 `node --experimental-strip-types .local/fortressblue-refactor/review/ui-boundary-check.mjs` 复验。这项是实际辅助函数的针对性验证，真实浏览器逐帧结果另见下节。
