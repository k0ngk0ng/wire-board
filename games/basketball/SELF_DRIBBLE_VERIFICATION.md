# 无持球者的自引用运球

实现前失败场景：将合法对象编号 6 当成非法玩家；错误选择最近球员或上一持球人；首次进入清除竖速而改变原行为；自引用位置复制引入小数或坐标变化；运球动画和 PoseSource 不一致；将动作 1 的证据泛化到动作 2 或无效对象；修复局部后原完整动作阶段或真实存档恢复仍有差异。

真实失败配置为场地 1、球队 `[2,3]`、球员 `[0,1,2,3]`、种子 `0x12345678`、全部电脑，失败视频帧 19313。原指令受控整世界执行证明：球员 1 的动作 16 在 `$90AA` 将球动作从 4 改为 1，未写持球者 6；随后原球动作 `$B53E` 使用 Y=6 复制球自身位置。没有证据支持为此指定另一位球员。

REA / Ghidra 保留了 `$B53E..$B584`（证据 `ev_e5a720578ce6436577389f8370b59b69f347fb4f0b8fa83997457dba8f1d75b1`）及 `$907E..$90B0`（证据 `ev_a96fad29a2b9b83a9434e6600d9cfeb7538f31ceef2293f7fb50248c794b3c08`）的完整函数体。原始证据位于 `.local/basketball/rea/held-owner-six/analysis/`。

`verify_self_dribble.py` 将失败存档中的语义状态一次性编码为原 RAM，从 `$D314` 执行至动作阶段，再执行原 `$D345..$D348` 全阶段。该阶段确实发生上述自引用行为。另有 24 次原球分派调用，覆盖首入／已进入、左右朝向、动画循环边界与不同竖速。所有调用保留精确坐标、动画、世界字段及音效对照。

实现仅允许 `BallDribble && Holder == 6` 的已证明路径；动作 2 和其他非玩家索引继续报错。自引用运球使用球自身位置作为复制源，不寻找替代球员，也不推进球物理。首入仅按原代码停止水平运动；竖速、重力和位置不因自引用而改变。

复现命令：

```sh
source scripts/env.sh
.local/monopoly/venv/bin/python scripts/basketball/verify_self_dribble.py
go test ./games/basketball/engine -run '^TestSelfDribble' -count=1
go test -race ./games/basketball/engine -run '^TestSelfDribble' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestSelfDribble' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
BASKETBALL_RUNTIME_DIAGNOSTIC=1 BASKETBALL_RUNTIME_SELECTION='{"court":1,"teams":[2,3],"characters":[0,1,2,3]}' go test ./games/basketball/engine -run '^TestRuntimeTeamRosterDiagnostic$' -count=1 -v
```

原始失败和受控原机证据均保留；这不是从原版菜单自然运行到此处的证明，也不改变此前已确认的悬空球缺陷。

## 当前结果与持续行为

24 次原调用、真实原动作全阶段和 300 个视频帧的保存恢复检查在 Native、race、Wasm 均通过。固定配置的完整局诊断越过原失败帧，但到 48,000 视频帧仍未结束：球保持动作 1、持球者 6、隐藏姿态，坐标不再变化。结果保存在 `.local/basketball/rea/held-owner-six/full-match.log`，该配置不能标记为完整局通过。

进一步在原整机中只对齐一次视频帧 19313 的语义状态，之后不再注入玩法数据。原机与独立 Go 连续 864 个完整世界状态一致；比较排除了原零页接触临时区和投影。后续 863 个世界均维持球自引用运球，最终时钟为 0，比分为 56:52，仍处于比赛阶段。观察版和不逐世界记录的参考版在 4,000 个视频帧的 RAM 与 3 张截图上一致，证据为 `original-world-result.json` 和 `continuous-comparison.json`。

该结果仅证明对齐上述初态后的原行为，不证明从原版菜单自然运行必然到达同一状态。没有替换持球者、强制落球、改变电脑决策或提前结算。该问题也不同于此前球离开高台后悬停的静止／滚动动作缺陷。
