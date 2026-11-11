# 赛车后端验收记录

2026 年 11 月 12 日。范围为生产模块、HTTP 鉴权、准备选车、数值规则、独立驾驶核心、电脑完整比赛、存档恢复及动态消息。可核验摘要、逐场数据路径、证据摘要和本次源码摘要见 [backend-validation.json](backend-validation.json)。浏览器画面、输入到像素延迟和正式发布由各自验收记录说明。

## 回归结果

| 检查 | 结果 | 记录 |
| --- | --- | --- |
| 完整 `go test ./games/racing/...`，含新版 60 场混合车型和旧版 75 场自然比赛 | 通过；server 0.733 秒，tests 447.197 秒 | `.local/racing-nfs-backend/final-regression.log` |
| 最后一次喇叭结束保护修改后的当前源码，重跑上述两组完整比赛之外的全部赛车测试 | 通过；server 0.585 秒，tests 43.358 秒 | `.local/racing-nfs-backend/current-source-regression.log` |
| 当前源码的喇叭、断线保护、六车消息压力和全部 HTTP 配置／输入隔离检查 | 通过；tests 4.592 秒 | `.local/racing-nfs-backend/horn-deadline-after-implementation.log` |

完整回归的编译发生在最后一次喇叭结束保护之前。该修改只增加结束阶段的喇叭投影限制，并清除保存的喇叭状态，没有改动运动、电脑策略、检查点、排名或期限。随后按当前源码重跑其余全部测试；两组完整比赛复用刚结束的完整回归结果，不将它们写作修改后再次完整运行。

可从仓库根目录复跑全部当前源码检查：

```sh
. scripts/env.sh
go test ./games/racing/... -count=1 -timeout=25m
```

测试通过生产 `Configure/Prepare/Start/BotAction/Apply/OnTimer/View/Restore/Result`，使用可控测试时钟推进原有 20 ms 物理步长。自然比赛不写入位置、速度、圈数、期限或结果；电脑只发送普通 `drive`，禁止复位或传送完赛。

## 新版自然比赛

四条独立路线 `france/coastal/uk/park`，分别覆盖 2、3、4、5、6 席和 1、3、5 圈，共 60 场。每场开始时明确断言车型按 `p911/fm50/ldia/mcf1/ccor` 循环；六席为五车型加一辆 P911，不能用全部 P911 的旧记录替代。

60 场全部达到真实完赛时间、正确圈数和 `lastReset=0`。最长一场为 Coastal 六席五圈，988.2 秒；所有比赛保留原有 1,200 秒上限。最大首末车完赛差为 14.724 秒，保留原有 30 秒余车期限。逐场检查还核对了最终车型序列、结束状态与各期限，不仅检查有无胜者。

逐场原始记录为 `.local/racing-nfs-backend/module/TestClassicNaturalCompleteRaces--<track>-<n>-seats-<laps>-laps.json`，汇总核查为 `.local/racing-nfs-backend/natural-mixed-audit.json`。另有五款车型分别两辆同车的一圈自然比赛基线，独立保存为 `TestClassicEveryVehicleNaturalRace--<vehicle>.json`。

电脑采用公开基础车型能力形成保守的基本驾驶速度包络，以当前原生路段曲率、制动距离和转向建立时间预看；更强发动机通过较低油门遵守该包络，真人车辆性能没有被统一。真实墙体仍参与碰撞；UK 中位于 VROAD 参考线的墙体保留，电脑使用可通行侧并通过正常前进／倒车恢复。路线参考线不被假定为始终可直行。

旧版显式 ID `metro/harbor/woodland/coast/canyon` 的 75 场自然比赛也全部完赛，继续使用 `physics=0`。旧存档几何保留、缺失版本字段不被隐式升级。记录为 `.local/racing/module/<track>-<n>-seats-<laps>-laps.json`，核查为 `.local/racing-nfs-backend/legacy-audit.json`。这些旧路线不计入新版四条交付赛道。

## 规则、运动与恢复

原路线分别保留 France 973、Coastal 1,075、UK 839、Park 939 个原生中心点，以及每点高程、非对称路宽、法线、方向和分段数据。服务端冻结完整几何摘要，仅嵌入数值规则；客户端模型、纹理和声音不进入服务端规则数据。新存档保存精确规则引用，恢复不替换几何，公开动态 View 不重复携带静态道路或碰撞表。

Go／TypeScript 对照核查四条完整数值规则、108 个路面／抓地采样和 96 个碰撞采样，共 147,196 个数值比较，最大差约 `8.3e-12`。结果为 `.local/racing-nfs-backend/road-parity-report.json`。运动核心的 24 个场景、14,700 步逐字段一致性结果见 [motion-parity-report.json](motion-parity-report.json)，参数来源与独立设计边界见 [MOTION.md](MOTION.md)。

当前源码另验证了准备选车鉴权、额外字段拒绝、实际人数少于房间容量、车型独立传动状态、模拟制动兼容、低速前进／倒车、保护制动、输入过期、取消托管、混合真人／电脑、恢复和自然结束。`TestClassicMixedDrivingAssistanceRestoreAndReclaim.json` 保存手动驾驶、显式托管、接管、断线保护、恢复及最终自然完赛的同场证据。

## 六车动态消息与喇叭

| 压力负载 | 实测 |
| --- | ---: |
| 六车普通行驶的最大动态 JSON | 5,169 字节 |
| 六车碰撞／合法低速复位／同时按住喇叭的最大动态 JSON | 7,036 字节 |
| 固定消息上限 | 8,192 字节 |
| 压力测试中的实际碰撞 | 15 次 |
| 压力测试中的合法手动复位 | 20 次 |
| 完整保留的事件历史 | 24 条 |
| 同时公开的喇叭标记 | 6 个 |

压力负载为 UK 六辆手动输入 LDIA：前段反复真实接触，后段加入满足速度与冷却约束的正常复位动作。没有注入事件、坐标或结果，每帧核对保存事件数与公开事件数一致。这个允许手动复位的消息压力负载与禁止一切复位的电脑自然比赛分别说明，不把两者混用。

喇叭沿用普通 `drive.horn?:boolean`，省略为 false；仍受席位、`epoch` 和严格递增 `seq` 鉴权。每份待执行输入保存自己的按住／松开值；View 从已应用输入投影，与车位时间线一致。释放、保护、过期、接管、合法复位、车辆完赛及比赛结束都会停止喇叭。85 帧成对有声／无声输入的运动、排名和期限完全一致。

最后补充的两个生产 E2E 先复现了结束阶段问题：真实首车完赛后的 30 秒期限，以及原有 20 分钟上限，均可能令未完赛车辆的喇叭保留为 true。失败记录在 `horn-deadline-before-implementation.log`；修正后两个场景及其余当前源码回归通过。测试不修改快照或期限。对应结果为 `TestClassicHornStopsForUnfinishedCarAtRaceDeadline--race-limit.json` 和 `--finish-deadline.json`。

## 证据边界

当前驾驶核心基于原始 CARP 参数和独立的 SI 双轴模型。Go／TypeScript 一致性与完整比赛证明当前实现内部行为和规则流程符合测试，不能证明与原版二进制逐函数等价。原版 Windows 的加速、制动、转向和音频输出尚无有效运行测量，本记录不声称已达到原版实机误差范围。浏览器渲染、实体手机性能、扬声器实际听感和网络延迟也不属于本后端记录的结论。
