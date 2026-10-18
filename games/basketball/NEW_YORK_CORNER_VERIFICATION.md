# 纽约球场通道边界修复

正常冷启动的 `tactic2-button64` 完整比赛曾在第 12,305 视频帧因相机进入不支持的页面而失败。根因发生在第 12,233 帧：人物斜向移动后已经离开高台深度范围，Go 仍判定其进入通道，导致人物持续处于地下。修复要求移动后的深度仍在高台范围内；同时恢复进入通道前按原地形属性调整纵向位置的操作。未改动相机、投影、随机种子或比赛时钟。

## 原版依据

原 ROM SHA-256：`0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`。

- `engine/testdata/new-york-corner.json.gz` 包含 480 次原版 `E165..E168` 完整持球动作调用。修复前 60 次误入通道，另有 20 次通道入口纵向位置不一致；修复后全部一致。测试先于实现编写，失败日志保留在 `.local/basketball/rea/new-york-corner-fix/before.log`。
- 坐标覆盖两侧边界及 72、73、74、79 像素深度，保持原位置的小数部分。原持球动作会按方向重新计算速度，输入速度组合不代表 480 条互不相同的轨迹；这些是受控函数对照，不能作为全部边界组合或自然菜单证据。
- 从第 12,233 帧的同一语义状态对齐原整机后，原版继续执行 4,000 视频帧。有观察钩子和无观察钩子的最终 RAM 及 4 张截图相同。修复后的 Go 与其中 864 个连续世界状态全部一致；比较排除接触候选临时数据和投影，保留人物、球、输入和 AI 等持久玩法状态。来源记录位于 `.local/basketball/rea/command-tactic2-projection/`，当前比较结果为 `.local/basketball/rea/new-york-corner-fix/continuous-comparison.json`。
- `verify_new_york_corner.py` 从已保留的原 RAM 初态重新执行未修改的 ROM 指令，生成内容与提交的 480 次调用逐字节一致。来源、初态和夹具摘要见 `engine/testdata/new-york-corner-source.json`。

## 验证结果

- 480 次边界调用与既有 9,233 次地形运动对照在 Native、race、Wasm 下通过。
- 原失败配置在 Native、Wasm 下分别连续完成两场。每场 22,373 视频帧、7,429 逻辑帧、84 次保存恢复检查，比分为 3:95；四次轨迹摘要均为 `824d00b7ca0ae887e1c6217dbe6b196919e55a35f72d574160e8a896b67668e6`。
- 六个内部战术 ID × A/B 的 12 组完整比赛在 Native 下分别完成两场，共 24 场。内部 ID 0 不代表原菜单存在第六种战术。
- 篮球引擎的非可选测试全套通过，耗时 137.108 秒；显式扩展矩阵仍按上列实际运行范围记录。
- 日志保留在 `.local/basketball/rea/new-york-corner-fix/`。此次仅验收篮球范围，没有运行其他游戏的规则矩阵。

## 重复验证

在本分支工作树根目录运行；原 ROM 与 Python 依赖沿用既有本地工具目录。

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/verify_new_york_corner.py
go test ./games/basketball/engine -run '^(TestOriginalNewYorkCornerCarrierCalls|TestOriginalPlateauMotion)$' -count=1
go test -race ./games/basketball/engine -run '^(TestOriginalNewYorkCornerCarrierCalls|TestOriginalPlateauMotion)$' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^(TestOriginalNewYorkCornerCarrierCalls|TestOriginalPlateauMotion)$' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
BASKETBALL_COMMAND_FULL_MATCH=1 go test ./games/basketball/engine -run '^TestRuntimeHumanCommandCompleteMatches$' -count=1 -v
BASKETBALL_COMMAND_FULL_MATCH=1 GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestRuntimeHumanCommandCompleteMatches/tactic2-button64$' -count=1 -v -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
```

本次修复未关闭完整战役接入、扩展阵容停滞、全部多人网络与素材发布验收等剩余事项。未发布，也未将整体复刻标记为完成。
