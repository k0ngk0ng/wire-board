# 可撞毁物件网页验收

2026-11-12。测试使用与手机输入验收相同的冻结构建，赛车 70 份源码摘要逐项一致，期间未修改产品源码或重建网页。浏览器结果与摘要见 [props-browser-validation.json](props-browser-validation.json)。规则、资源和原版对照边界另见 [PROPS.md](PROPS.md)。

## 实际页面结果

| 场景 | 结果 |
| --- | --- |
| France 白昼 prop1，目标 4／13 m/s | 从正常起点驾驶，分别触发一次 low／high 事件；实际渲染器采到 23／87 帧对应动作 |
| France 夜景 prop1；UK 昼夜 prop0 | 权威位图、catalog 和画面一致；原 CAN 动作按 0.46875／1.5 秒播放一次，时间单调，完成后不重播 |
| 独立观众与后车 | 观众收到相同破坏状态并看到动作；另一原起点车辆穿过已破坏位置，不重复产生破坏事件 |
| 刷新 | 已破坏物件直接隐藏，无旧动作重播；epoch 与比赛时限不变 |
| UK 固定墙 | 实际接触未映射物件的 wall edge9，胶囊中线距墙 0.95008 m，发生权威碰撞并减速；物件位图未变 |
| 六车桌面 | 60.01 FPS，帧耗时 P95 18.2 ms，最大动态消息 5,414 字节 |
| 六车 320px 仿真 | 60.00 FPS，帧耗时 P95 18.3 ms，最大动态消息 5,405 字节 |

保留 PC ≥50 FPS／P95 ≤35 ms、320px ≥30 FPS／P95 ≤50 ms 和 8,192 字节动态消息预算，未降低几何精度。性能场景先产生真实物件破坏，再用平台托管让六车继续移动；没有重复传输静态目录。

第一轮扩展测试在固定墙的预设目标距离判定上失败：车实际已被另一段固定墙阻挡，无法接近该候选目标。该次报告保留为失败，仅复用此前六项已完成的画面与联机检查。独立复测改为从正常起点驶向可达的墙外目标，并核对未映射的实际墙段、车身胶囊距离、权威碰撞、减速和位图不变；该门槛已通过。未通过放宽性能预算或修改游戏碰撞来处理测试路线问题。

## 重放与产物

```sh
. scripts/env.sh
npm --prefix web run build
python3 -B scripts/racing/check-nfs.py --out .local/racing-nfs-browser/props-minimal-01 --phase props-minimal
python3 -B scripts/racing/check-nfs.py --out .local/racing-nfs-browser/props-new --phase props
```

第一条浏览器命令产生 France 低／高速基线；第二条要求基线与当前赛车源码及构建摘要完全相同，再执行 UK 昼夜、France 夜景、观众、后车、刷新、固定墙和六车性能。输出目录必须尚不存在；保留当前已验证基线时，可直接用新目录重跑第二条。固定墙与性能也可分别用 `--phase props-wall`、`--phase props-perf` 独立重放。

本次产物为 `.local/racing-nfs-browser/props-minimal-01/`、`props-browser-01/`、`props-wall-01/` 和 `props-performance-01/`，包含截图、普通输入、真实网络车况与逐 RAF 动作采样；聚合结果为 `.local/racing-nfs-browser/props-aggregate.json`。全部测试房间服务和浏览器已关闭，临时 profile 与对应 socket 已清理。

## 验证边界

目标驾驶使用正常鉴权的 HTTP `drive` 动作与真实时间，不写入坐标、速度、位图、事件或时钟。原生触屏、键盘与 Gamepad 模拟边界的操作检查由 [手机输入验收](MOBILE-SAFETY.md) 覆盖。320px 结果为 Chromium 触屏仿真，不声称实体 iOS、具体手柄硬件或原版 Windows 实机已测；不是逐件开车撞过全部 265 件物件。
