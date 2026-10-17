# 特殊球 22 的恢复与验证

## 实现前失败场景

- 首次进入时漏掉音效 30、原有动画重置、尾段光标 4、垂直速度 -1472、水平速度 ±8、目标篮筐和放球；错误改写重力、深度速度或共享投篮目标。
- 已进入时重复初始化或重复音效；动画主段与尾段混用，主段 FD FF 的 255 次循环与后续动作处理错误。
- 运动后使用过期篮筐；落地时忽略原版未镜像的 pose 31，或将只改垂直速度高字节错误实现为整值覆盖。
- 左右转向线的等号、亚像素速度、低于 88 与等于 88 高度边界错误；偏转后未归零深度速度。
- 二次下落时 X 半场、保留的小数位、固定深度和重力错误；主段落地未转普通反弹。
- 设备替换动作后按球 22 的偏移判断；未知选择器不应猜测。
- 连续链每帧重置上下文、JSON 恢复丢状态，或只凭孤立函数宣布完整玩法通过。

证据和结果将在本文件中追加。范围仅为受控原机函数与连续链，不代表全部自然比赛覆盖。

## 来源与恢复结果

- ROM SHA-256：`0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`。
- REA/Ghidra 分析 `bank6 BD23`、`BDD2`、`BAAB`；完整证据位于 `.local/basketball/rea/ball22/analysis/`。批次 Evidence ID：`ev_dac80373a79e243901074958000b47dedb8ee272753c0aa7197d9d9fa01cc77e`。静态反编译仅解释分支，由未修改原指令的 py65 执行补充动态对照。
- 进入时请求声音 30，动画从光标 4 开始；垂直速度置为 -1472，水平速度置为 ±2048，保留原重力和深度速度。按球当前高度与朝向选择篮筐，持有者改为球自身。
- 尾段动画为 pose 32、255 tick；达到目标附近的转向线时偏移为 16 像素。低于 88 像素先转动作 12 并只修改垂直速度高字节；达到 88 像素后进入动作 22 主段、按原半场重定位下落。主段依次 pose 32/30/31，其 FD FF 结束操作保留原 255 次计数和下一动作机制。
- 球落地但尚在尾段时，原版强制未镜像 pose 31。主段落地转普通反弹动作 6。
- `CompleteOriginalBall22` 读取运动后的实际篮筐；设备替换动作造成未恢复的转向表选择时显式失败。没有将未知硬件/共享状态填入常量。

## 可重复执行验证

先执行 `source scripts/env.sh`，再运行：

```sh
python3 scripts/basketball/verify_ball22.py
go test ./games/basketball/engine -run '^TestOriginalBall22' -count=1
go test -race ./games/basketball/engine -run '^TestOriginalBall22' -count=1
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run '^TestOriginalBall22' -count=1
```

原指令夹具：340 个受控调用；8 条持续执行链，长度依次 48、45、43、71、27、58、59、72（423 次调用）。链包含动作 22、原有动作 12，以及最后转反弹 6；不是每步重置原快照。Go 比较全部 `WorldActionState`、音效顺序，并在每步序列化与恢复。测试先于实现，初始失败及中间失败均保留。

这些证据不等同于自然完整比赛、所有场地、所有玩家或设备替换路径通过；共享世界接入由独立适配器验证覆盖。

Native、race、Wasm 定向验证均通过，夹具重新生成哈希一致。日志和结果索引：`.local/basketball/rea/ball22/result.json`。
