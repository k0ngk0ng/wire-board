# 正式三人／四人公开资源显示

本轮基于 `2ac14ca9`，只修改游戏的 `PlayerSummary` 和对应数值分组样式。公共玩家栏同时显示体力与灵力，直接读取所渲染的权威 View；资源下限按原体力显示方式钳制为 0。网络时钟、输入、RPC 和规则源码未修改。

## 实际平台验证

| 模式 | 真实组成 | 原版 slots | 原版 baseTeam | 结果 |
| --- | --- | --- | --- | --- |
| 四人组队 | 2 个真人账号、2 个显式电脑 | 0／1／2／3 | 1／1／4／4 | [正式结果](../platform-mode-ui/four-team-energy-green01/result.json) |
| 三人混战 | 2 个真人账号、1 个显式电脑 | 0／1／3 | 1／2／4 | [正式结果](../platform-mode-ui/three-free-energy-green01/result.json) |

创建、加入、添加电脑、准备、选角与开始均使用正常平台页面。原版 slot、输入 channel 与队伍字段由只读 Go 工具解码实际认证 `/sync` 返回的 checkpoint 核实，JavaScript 不解析或重序列化 checkpoint 内部数据。

两个模式都先取得公共玩家栏缺灵力的真实 red。修复后，公开文本与同一渲染帧的权威数据、真实 WebSocket 记录逐席相同；四人示例灵力为 223／223／199／191。原生 PC 按键和三个视口的原生 C 触摸均进入实际服务器规则帧，保留输入和截图。

零值来自实际对局自然结束。完成手动操作后，两个真人明确通过公共入口开启托管，原版策略完成对局；四人结算帧为 3881，三人结算帧为 4610。所有席位真实体力／灵力均为 0，文本和资源字框都位于 320×740 可视区及自己的公共玩家卡中。未注入数值、跳帧或伪造结束状态。

## 已报告给 Root 的独立界面问题

- 320 px 普通页面中，三／四人公共卡片占高后将手柄推到初始可视区下方。四人原版卡片区为 213 px，补灵力后为 253 px；手柄初始底边为 882.8 px，超出 740 px 视区。原始截图与几何均保留，游戏 CSS 未隐藏或重排公共界面。
- 四人组队数据正确，但准备和公开玩家区域尚未清楚标明成员队伍关系；实际截图已提交 Root 判断。
- 准备阶段的自有下拉框使用暗背景并继承平台暗色文字，对比度不足；公共 Options 与 Preparation 还重复显示场地／赛制。此轮未擅自修改公共装配。

这些问题不被资源显示验收的 `pass` 掩盖，完整三／四人界面仍需 Root 决定后续最小方案。740×360 原生全屏的上下布局、完整画面、触屏和 ESC 在两个模式均有效。

## 来源与构建

Wasm 重新编译后包含较新的 `caf9b131` VCS 构建元数据，字节摘要因此变化。[依赖检查](public-energy-wasm-source-invariance.json) 逐项确认实际链接的 110 个项目源码与嵌入输入和 `2ac14ca9` 字节相同，未把二进制摘要变化误报为逻辑修改。

重复执行使用独立 label：

```sh
python3 scripts/yuyuhakusho/check_platform_modes.py --label four-team-recheck --mode four-team --zero
python3 scripts/yuyuhakusho/check_platform_modes.py --label three-free-recheck --mode three-free --zero
```
