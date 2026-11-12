# 可撞毁物件规则与验收

2026 年 11 月 12 日。本记录覆盖数值 catalog、权威碰撞、存档与位图、电脑过滤、客户端规则和本地预测；实际画面、动画与手机操作由浏览器验收另行说明。机器摘要和证据路径见 [props-validation.json](props-validation.json)，实现前清单见 [classic-props-failures.md](../../../games/racing/tests/classic-props-failures.md)。

## 分类与版本

仅启用已核实的 normal 对象、属性位置一致且 collision type 为 2 的白名单。France 29、Coastal 83、UK 108、Park 45，共 265 件。现有 route 中的全部 type 2 数量分别为 88、312、180、113；多出的 billboard／special 等对象不能按同一标签猜测为可撞毁。

新增数值文件位于 `games/racing/server/data/props/<catalogHash>.json`，由 `data/prop-index.json` 校验实际字节摘要，绑定旧 geometry 与原规则文件 SHA-256。每件保留 0-based index、原身份和展开碰撞边 `ruleEdges`。四份旧 route、geometryHash、server hash-named 规则文件均未改动，逐字节检查记录在 `.local/racing-nfs-backend/props-immutable-road-check.json`。目录来源与转换验证由原资源流水线说明。

State／View 使用 `props:{catalog,broken}`。broken 为固定长度 base64 位图，每件占一位，UK 最大为 14 个原始字节、20 个 base64 字符。位图按比赛独立持有；静态规则、catalog 和空间索引不因破坏而修改。

旧 v3 缺字段时按固定首版 catalog 解释为全部未破坏，不改车况、epoch、进度或期限。普通 Restore 保留原字节，首次正常 timer 保存才记录明确版本。明确但未知的 catalog、几何不匹配、错误 base64、错误长度和未用高位非零均拒绝恢复。旧 physics=0 不使用道具状态。

## 接触与预测

权威物理步在首次有效接近接触时置位，立即跳过该物件全部边的静态墙反弹，后车和电脑选线不再受其阻挡。高度过滤与固定墙求解保持原行为。一个物件的多边、多次求解和多车接触不能重复生成破坏事件。

新增事件共用原有 24 条环：`type:"prop"`、`prop`、`dir`、`high`。prop 的 0 有效；位置由 catalog 获取，事件 X/Y 为 0。方向来自碰撞前速度，量化到 0.001 弧度；法向接近速度不低于 8 m/s 时选 high。8 m/s 为重建约定，原版触发阈值未经实机测量。原 low/high 动画的 6／17 帧、延时 6 及社区 64 Hz 解释也不作为原版运行输出认证。

位图决定当前世界，事件仅提供有界动画信息。丢帧、迟到观战和超过事件环时，直接使用位图，不依赖补播旧动画。客户端预测复制位图做本次接触推算，不修改已确认位图或公共场景显隐。

## 实际结果

| 检查 | 结果 |
| --- | --- |
| 完整 `go test ./games/racing/... -count=1 -timeout=25m` | 通过；server 0.710 秒，tests 445.837 秒 |
| 60 场新版混合车型自然比赛 | 全车真实完赛、正确圈数、没有 reset；最长 988.2 秒，首末车最大差 14.724 秒 |
| 75 场旧版自然比赛 | 全车真实完赛，仍为 physics=0，不输出 props |
| 普通 drive 朝 France 原物件行驶 | 4 m/s 与 13 m/s 均只触发一次，正确分档，没有静态墙停车；保存／恢复、接管和房间隔离通过 |
| 普通 drive 沿 UK 原路线接触物件 0 | 破坏位与 `prop:0` 事件均保留 |
| 265 件 × 5 车 × 5 状态数值对照 | 6,625 个 Go／TypeScript probe、2,797 次 first-hit 事件一致 |
| 高程分离与已破坏物件 | 各 1,325 个 probe：目标未误破坏、未重复发同物件事件 |
| 真实 Module 输入跨首次破坏的预测对照 | 53 帧；最大位置差 0.001133 分米，最大 RPM 差约 0.0607；已确认位图未变 |
| 六车、六喇叭、真实接触与完整 24 条事件环 | 13 位 Unix 毫秒开局，最大动态 JSON 7,575 字节，小于 8,192 字节 |
| 独立 TypeScript 类型检查 | 所属规则、预测和公共类型通过，未构建 dist |

六车压力负载通过普通输入产生两次不同的真实物件破坏，其余为碰撞与满足速度／冷却条件的手动复位事件。它不等于 24 次真实道具破坏，也不与禁止 reset 的自然比赛混用。60 场基本电脑选择绕过完整物件，没有产生破坏；这组记录证明比赛回归，不能写成逐场撞毁验收。

普通行驶接触直接覆盖 France 与 UK 的目标；其余物件由原资源转换验证及完整数值 probe 覆盖，不声称每件都已在浏览器里开车撞过。原版 Windows 的阈值、音频和画面仍无有效实机对照。

复跑命令：

```sh
. scripts/env.sh
go test ./games/racing/... -count=1 -timeout=25m
node --experimental-strip-types --test games/racing/tests/classic-props.test.mjs
```
