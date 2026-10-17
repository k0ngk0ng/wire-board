# 纽约高台地形验证

修改实现前的失败场景：高台落地被当成未实现；在台面静置时 Air 无法清除；越出台缘没有转下落；正面撞台和侧向通道入口混用；有球与无球的台面推离不同；低于台面且无竖直运动时旧 Air 被错误保留；回退旧 XY 时误用新小数；普通边界钳位误改动作；场地低位地形属性被当高度；持球跟随前漏掉地形反弹请求。原附着对象直接跟随父物体的路径不得当作自由物理验收。

先记录完整 EEFF 调用，再根据原始输入／输出验证 terrain helper；所有数据来自原 6000 帧场景，无 RAM 注入。

## 原代码和自然记录

REA 复用 `.local/basketball/rea/auxiliary-rest/terrain/` 的表面主分支；新增批量证据 `ev_d1a2ac6742dc81078ff4fda47d598dcbb0492134a4a44086dfa918b553789b8a` 位于 `.local/basketball/rea/plateau/`，包括 `$8249/$840B/$8458/$86AF/$873C`。纽约的 `$8458` 分派到 `$84D6` 返回；不能拿其他场地副作用套入纽约。

纽约地形表深度 72～79 行在台面格为 67（高度 64、属性 3）；80～87 行为 66（高度 64、属性 2）。属性控制落地后的深度修正；球动作 1／2 明确跳过该修正。高度主判断看高字节与低字节，但侧碰辅助比较仅用低字节，通道中负高度不能直接用有符号整值代替。

自然原场景完整记录 13,229 次 EEFF，原始 RAM 与截图均和未加探针参考一致。定向验证覆盖全部四个人物 8,055 次（2,000／2,027／2,017／2,011）和全部篮球 1,178 次；无附着跟随调用需要排除。另保存 3,996 次道具调用，当前此测试不将其算入玩家／球验证。

所验证人物／球输入包含 218 次终态处于高台区域、63 次台面落地状态（其中 18 次由腾空转落地）、9 次离开高台区域和 6 次人物通道入口。记录严格逐调用比较，不把动作阶段首末差异当作内部地形证据。

关键样本：原帧 3653 的持球高度 32、位置 X=135.25、Depth=86，零速度和重力；地形请求球动作 8、反弹 X，并清 Air，X 不得因没有水平位移而回退到 128.25。接下来通道中高度为 -256 的球不应重复制造碰撞。

复现：

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_plateau_motion.py
source scripts/env.sh
go test -v ./games/basketball/engine -run TestOriginalPlateauMotion
```

原始压缩输入／输出及身份在 `engine/testdata/plateau-motion.ram.gz` 和 `plateau-motion-source.json`。`plateau-before.log` 保留修改前真实失败；`plateau-after.log` 为通过记录。仍直调旧 `advanceOnFlatCourt` 的篮筐和持筐入口不由本次修改自动改接；完整 world 验收继续由调度器工作项处理。
