# 球场背景动画恢复记录

已恢复原 bank 4 `$9B95` 的八场地分派。球场 0 的水面、球场 2 的交替机关及球场 3 的门动画已接入运行和显示管线；其他球场仍为独立组件。当前结果不代表八场地已在平台开放或完整玩法已经验收。

## 接口和阶段

`StepCourtAnimation(state, context)` 输入可保存的 `CourtAnimationState` 与当前球场、活动状态、主循环计数、相机 X、共享 RNG、比分及篮筐归属／得分锁。运行接口不读取 ROM、原 RAM、采样文件或预录轨迹。

- `Active` 对应原 `$0589 & $7F` 不为 0 或 2。接口字段 `VideoFrame` 保留早期名称，实际对应原 `$0300` 主循环计数：固定 `$CA58` 每轮加 1，现模型使用 `World.LogicFrame`，不能传入 `Runtime.VideoFrames`。偶数计数不推进。
- `CameraX` 对应原 `$0397`，接线使用 `World.Camera.DisplayedX`，而非本轮已经推进的 `Camera.X`。只按此值裁切当前写入，不修改计数推进规则。
- `Patches` 按原三个视频队列的提交顺序输出。每项为 `{plane,row,column,tiles}`；当前采用逐 tile 项，`tiles` 是 JSON 数字数组。水平镜像的 `$2800/$2C00` 对应 plane 1，不修改属性表。
- `Palette=true` 时，`PaletteID` 是原 `$0304` 调色板选择。原 `$FCEC` 调用 bank 4 `$94E0` 重建调色板，并非切换 CHR。此组件发出选择命令，不装载显示颜色。
- `Sounds` 保留发声顺序。`Random` 和 `Scores` 返回本阶段的权威结果；老虎机使用同一世界 RNG，不能在前端独立随机。

机关触发须由对应玩法阶段更新语义状态。球场 0 已接入下述时序与显示历史；其他场地的机关和调色板运行接线仍待验收。

| 球场 | 原入口 | 恢复行为 |
| --- | --- | --- |
| 0 | `$9BBF` | 四帧水面、三行更新、相机右侧裁切 |
| 1 | `$A32C` | 无操作 |
| 2 | `$9D28` | 两侧交替检查触发、两步冷却、三行局部更新 |
| 3 | `$9E62` | 触发后八步机关动画、静态闪动、纵向写入 |
| 4 | `$9FBA` | 三态正／反向调色板周期 |
| 5 | `$9FEF` | 两侧交替触发、冷却、不同左右裁切 |
| 6 | `$A124` | 六滚轴、两侧停止进度、图案奖励、音效及调色板闪动 |
| 7 | `$A32C` | 无操作 |

## 证据与限制

REA 证据：`ev_3fb4a9e82c28e78326cecbf7989d21c6afda7465ddd29c906610caa566273c7a`，保存在 `.local/basketball/rea/court-animation/analysis/`。反编译中的移位、进位和间接表按原机器码核对，未直接翻译反编译器的临时变量。

`court-animation.json.gz` 保留八场地共 **912 次**完整函数调用。沿用图形验收场景，只在首次球场初始化时选择球场；随后按原输入运行，八次采样的整段 RAM 和截图均与相同配置、无动画钩子的参考运行一致。该证据不是自然菜单选择记录。原采样中球场 2／5 没有触发机关，球场 6 没有获得奖励，不能用这些样本声称自然对局覆盖奖励。

`court-animation-probes.json.gz` 另保留 **13,312 次**隔离原指令执行，覆盖 256 个相机位置、触发位、冷却结束、门动画、正反调色板、滚轴停止及四类匹配奖励。原 RNG 指令实际执行；声音和调色板外设入口仅记录命令。隔离探测不计为自然比赛或浏览器验收。

实现前失败场景见 `VERIFICATION.md`。测试先失败记录、通过记录及 native race／Wasm 输出分别为 `.local/basketball/engine/court-animation-{before,after,race,wasm}.log`。

## 重复执行

在仓库根目录执行：

```bash
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_court_animation.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/probe_court_animation.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_court_animation_tables.py
gofmt -w games/basketball/engine/court_animation_tables.go
go test -race ./games/basketball/engine -run '^TestCourtAnimation'
basketball_goroot=$(go env GOROOT)
GOOS=js GOARCH=wasm go test -exec "$basketball_goroot/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestCourtAnimation'
```

采样依赖已授权的本地原 ROM、FCEUX、已有配置参考场景及工作区内 Python 环境；普通 Go／Wasm 测试只读取已保留的测试数据。

## 球场 0 运行接线

原 `$D352` 在对象投影后调用 `$FCFB → bank 4 $8009 → $9B95`。运行层在两处 `StepWorldLoop` 成功并处理列流送后调用 `observeRuntimeCourtAnimation`；暂停入口当次以 phase 2 禁止推进，暂停／恢复专用投影分支不调用该函数。

水面帧只保存在权威 `$06D7` 槽，即当前 `World.Preparation.Actions.Actors[0].Player.NewYorkPassage` 字段。球场 0 地形适配先区分球场，不把它解释为纽约通道；适配器临时构造动画输入，返回后立即写回同一槽，不新增第二份持久动画状态。

`QueueDisplayPatches` 只暂存 tile 命令，下一次 `CommitDisplayVideo` 才提交。提交顺序与原视频例程一致：先列流送，再背景 patch。`CourtPatches` 仅保留每格最后写入，列覆盖时删除同一物理列旧 patch，最多 1,728 格。恢复先重放至多 32 个 `CourtColumns`，再重放剩余 `CourtPatches`；patch 独立提交也递增 `CourtSerial`，使客户端重新绘图。背景重置同时清除已提交及待提交 patch。

新增测试先捕获未接线时水面帧不变，再验证真实运行的 36 视频帧、6 次水面推进、6 次视频提交及每帧 JSON 恢复一致。另覆盖裁切取旧显示相机、逻辑计数与视频计数相反时的相位、列覆盖、多 tile 展开、历史容量、输入切片隔离及非法 patch 原子拒绝。日志位于 `.local/basketball/engine/court-animation-{runtime-before,runtime-after,display-before,display-after,display-race,display-wasm}.log`。该短运行验证不是完整球场 0 比赛或原画面逐像素一致性证明。

集成中的第 600 视频帧恢复检查另发现：相机移出水面区域、列流送清掉最后一个 patch 后，非 `nil` 空切片被 `omitempty` 丢弃，恢复成 `nil` 导致结构不一致。已先用 `TestDisplayCourtFinalPatchOverwriteRoundTrips` 复现，再将空历史规范化为 `nil`；保留旧快照和纽约序列的字段省略行为。先失败记录为 `.local/basketball/engine/court-animation-empty-before.log`。


球场 3 接线与原调用／完整对局／浏览器门禁见 [COURT_THREE_INTEGRATION.md](COURT_THREE_INTEGRATION.md)。
