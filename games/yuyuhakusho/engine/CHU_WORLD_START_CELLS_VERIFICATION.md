# 开局 HUD 共享单元物化

本补充只核对 manager 开局阶段发出的 `words` 请求与共享 `Cells` 的关系。已有 `stepChuWorldStart` 只保留请求，没有将 `FE68` 的真实内存写入交给共享单元，因此全新世界缺少血条／能量条边框及控制器字样的初始模板。

实现前失败场景：把 `E600` 错写成后续人物 HUD 视图的 `E602`；把 byte 偏移当作 word 索引；模板尾端丢失或覆盖邻单元；按类型分组打乱原请求顺序；只补新世界构造而漏掉正常 manager 阶段；暂停／资源未就绪仍写单元；奇数、跨尾端或 word 回绕偏移静默裁切；晚期无效请求导致对象、色盘、单元或事件部分提交；把名字字体或硬件 DMA 结果混作本次规则验证。

先采集原 CPU `FE68` 的自然调用和私有明确偏移调用，完整保存 `E600..EC00` 前后数据；再建立共享世界集成测试并运行真实失败，最后实现。本补充不执行原 CPU，也不在生产状态保存内存镜像。名字及 portrait 继续使用原来明确的呈现事件边界。

## 恢复结果与证据边界

`stepChuWorldStart` 在合并对象前将 `words` 请求按原顺序写入唯一的 `Cells`。`FE68` 的目标基址是 `E600`，byte 偏移除以 2 后才是 word 索引；此处不能沿用人物更新的 `E602` 视图。写入先在值副本中完成，奇数或跨越 `EC00` 的请求拒绝整个世界轮次。原重置的两处行滚动首单元先写，HUD 请求随后写，不增加第二份 HUD 状态。

原 CPU 证据包含 6 次自然调用和 121 次隔离调用。隔离覆盖全部 4 组 HUD 偏移、各位置的 5 种控制器模板、血条与能量条，以及共享单元的首端和最后可写 word。每次都比较完整的 1,536 bytes，确认邻单元保留；观察内核的 3,600 帧 RAM／视频摘要与固定原版基线一致。奇数、越界和回绕偏移属于独立实现的拒绝边界，不声称原 CPU 对这些非法输入返回同样错误。

三个新增测试分别验证全部 127 次原 CPU 写入、顺序覆盖与原子拒绝、以及全新世界的 manager 集成。新鲜配置覆盖 2／3／4 人，各有显式电脑和真人通道版本，未就绪与暂停时不写模板；资源就绪后的首个完整 tick 保留血条／能量条边框和控制器标签。晚期第二个人物的非法偏移须同时回滚首个人物、色盘、时钟、对象和事件。测试的真实缺实现红灯是 `matchStartCells` 未定义，保留在 `testdata/chu-world-start-cells-red.jsonl`。

新增测试已通过 native `-race` 和 Node/Wasm。另对既有世界流执行了 8 项顶层回归：4,758 轮、62,333 次回调的逐轮与连续对照通过，native 110.790 秒，Wasm 29.852 秒。这两份回归日志在随后增加呈现事件载荷之前生成，独立报告明确保留这一时间边界，不将其绑定成扩展后事件载荷的验证；新增三项聚焦日志在扩展后刷新。

此补充只恢复 `FE68` 的共享模板写入。`FDBE→0FE0` 的名字字形光栅、portrait、窗口平面、VRAM 和 DMA 仍由现有呈现事件边界负责；没有把它们宣称为完整 HUD 像素验收。来源与日志绑定见 [验证报告](testdata/chu-world-start-cells-validation.json)。

## 复跑

在 worktree 根目录运行，取证需要已有固定 ROM 和观察内核；Go 验证只读取语义规则包与规范夹具：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_match_start_cells.py \
  --out .local/md-yuyuhakusho/match-start-cells-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestChuWorldStartCells' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestChuWorldStartCells' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

取证复跑输出与 `testdata/match-start-cells.json.gz`、manifest 摘要应一致，不能通过改写预期结果绕过原基线或夹具摘要失败。
