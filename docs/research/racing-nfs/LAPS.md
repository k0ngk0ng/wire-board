# 路肩计圈与复位回归

用户报告凯尔特遗迹三圈比赛一直显示 1/3，并且复位位置有时偏远。故障来自有效检查点范围：驾驶模型允许驶上两侧 3 m 路肩，但检查点只接受车身覆盖柏油路。839 个采样点中漏掉一个后，后续检查点全部等待；显示的已完成圈数保持 0，复位也回到过期的已通过检查点。

修改使检查点与驾驶模型共用两侧 3 m 路肩范围，仍要求车身相交、正确高程、按序且正向经过全部门线。没有更改圈数显示公式、复位动作、赛道或运动参数，也没有增加协议字段。

## 先行复现

失败清单为 `games/racing/tests/lap-reset-failures.md`。生产 Module 从原起点以普通 `drive` 在 UK 第 20～35 点短暂驶上左侧路肩，然后正常返回道路。

- 修复前：实际行驶 3,877.1 m，路线长 3,867.1 m；已完成圈数仍为 0，gate=20、checkpoint=21、routeIndex=1。
- 修复前加入正常制动／复位：车辆回到约 209.1 m 以外的过期检查点。
- 修复后：同样路线正常计圈；复位距离约 3.57 m，复位前后圈数与该动作提交时的进度相同；随后经 Restore 并继续普通驾驶，完成三圈且生成完赛时间。

另有四条原路线共 28 项隔离几何检查，覆盖铺装路面、路肩内外、脱离路线的捷径、不同高程、逆向和静止。旧版逆向／终点捷径保护与同帧冲线顺序检查保留。

## 重放与证据

```sh
. scripts/env.sh
export WIRE_BOARD_RACING_EVIDENCE="$PWD/.local/racing-laps/replay"
go test ./games/racing/... -run 'TestClassicHumanLinesCountCompletedCircuit|TestClassicGateRoadShoulderAndProtection|TestOrderedGateProtection|TestFinishRanksUseActualCrossingWithinStep' -count=1 -v
go test ./games/racing/... -count=1 -timeout=45m
npm --prefix web run build
python3 -B scripts/racing/check-nfs.py --out .local/racing-laps/browser-new --phase laps
```

网页脚本从正常起点执行同样的路肩驾驶，用原生 R 键复位，再通过平台托管继续实际三圈比赛。它等待真实服务器时间，核对每次权威圈数与可见 HUD，保留 2/3、3/3 和最终完赛截图，不写入坐标、检查点、圈数、事件或时钟。复位前后比赛身份、时限和已完成圈数必须一致。

本次针对性红／绿日志为 `.local/racing-laps/all-before.log`、`targeted-after.log`，轨迹在 `before/module/` 与 `after/module/`。完整回归和实际网页结果以同目录的 `full-regression.log` 与 `browser-01/report.json` 为准。

完整 Go 回归通过（server 0.829 s，tests 470.681 s）；实际网页七项检查通过，观察到已完成圈数 1、2、3，仪表与权威状态一致，第三圈生成有效完赛时间。脚本在正常鉴权的 HTTP 驾驶后使用原生 R 键，再通过平台托管完成实际比赛；未使用实体手机测试。摘要与产物哈希见 [laps-validation.json](laps-validation.json)。
