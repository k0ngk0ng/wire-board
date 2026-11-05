# 飞影双真人吞龙完整组合：实现前边界

该 scope 独立于原普通／特殊 12 组夹具。只复用 selector 提供的 `b303ca…` 输入和唯一 `9f282f…` 原 core 基线，覆盖 5502 帧、5500 帧入口及 2 帧排空。source3 真人位于 `8400`／channel0，source4 飞影真人位于 `9000`／channel1，父／子为 `9000`／`9100`。只捕获完整 input、action 和 active actor 返回，不运行 Go 世界，不修改原六项冻结断言或旧 12 组数据。

先行失败场景：沿用旧 collector 的 3600 截止遗漏增强到恢复；误用 8400 来源或 channel0 控制飞影；重复采集唯一原 core control；只记录吞龙瞬间便声称生命周期完成；natural 行不存在 12578→15946 或后续恢复默认移动源／负能量／站立／子消失；规则改源后参数仍取默认值；三层之间 +90／A038／ChildExtras／FXPhase 丢失；把 child 回调之间的变化当成人物 API 自行推进；仅比较 decoder 状态而跳过完整 raw 输出；没有真实音频 PC／地址便排除写入；观察器改变回放；入口之后的调用未完整排空；为使测试通过而改输入、基线、期望中间态或原六冻结。

三项独立 wrapper 先冻结并保存缺失夹具的真实红灯，再实现专用 natural-only collector 和采集。完整调用检查复用现有原版状态／raw 对照函数；生命周期断言只观察有序自然返回，不给下一行注入期望状态。source4 AI 仍须为 0，双真人不代表自然 AI；人物 API 不包含 child、sprite、collision manager 或世界调度的闭环。

## 最终结果

三项先行 wrapper 在首次消费 Duo 夹具时即通过 native 竞态检查和 Node/Wasm 验证，分别用时 39.893 秒和 11.718 秒。冻结测试 SHA-256 为 `cf4990b5493177c9b5c936a85fb61377baf252672c0dc4d870676b44b7c2b4b4`，未改生产代码或测试断言。原六项冻结测试、旧 collector 和原 12 组 CPU 夹具及 manifest 均未变化。

| 证据 | Action | Input | Actor |
| --- | ---: | ---: | ---: |
| 完整返回 | 3316 | 3316 | 3316 |
| 增强移动源行数 | 711 | 710 | 711 |
| 自然 source4 AI／录制调用 | 0／0 | 0／0 | 0／0 |
| 完整状态及 raw 差异 | 0 | 0 | 0 |

专用 collector 仅读取 hook、寄存器和 RAM，不恢复存档，不主动调用 CPU 运行接口或写入 RAM。5502 帧的 RAM／画面摘要全部匹配 selector 提供的唯一原 core control；只接受 5500 帧及以前的入口，并排空后续 2 帧。实际绑定为飞影 `9000`／channel1、幽助 `8400`／channel0；`9000`／`8400` 为原版对象编号，channel1／channel0 为原版输入通道，与平台席位分开。输入路线为 disabled 273 行、human 3043 行。

## 自然时间点

| 视频标签 | 观察位置与结果 |
| --- | --- |
| 3589 | 同次完整 Action／Actor 从 `2a` 进入 `2b`，移动源从 `12578` 改为 `15946`。 |
| 3590～4299 | Input 入口观察到增强移动源，共 710 行；Action／Actor 另包含吞龙当次，共 711 行。 |
| 3598 | 子对象首次创建为 `13fc8`；这是创建／回调时间点，与 3589 的人物改源时间不同。 |
| 4300 | 完整人物 API 首次已取回 `12578`；能量 long 为 `-4096`，高 word 为 `-1`，子对象已清空，但人物仍为 action5／pose25。 |
| 4379 | 首次同时满足默认移动源、负能量、idle 和 child0；整段共有 866 行满足该联合条件。 |

上一帧 4299 的人物返回仍持有增强移动源，随后独立 child 回调恢复默认源；下一次人物入口才在 4300 看到恢复结果。世界帧末的 4299 观察与这里的 4300 API 入口观察一致。`testdata/fighter4-duo-composition-milestones.json` 保存了这些时间点的原始夹具派生字段，未修改 CPU 夹具。

三层 wrapper 每行独立比较该完整 API 的所有 canonical 状态及 raw 输出，并保留同次改源后的真实参数元数据。自然行之间由原版继续调度 child／世界；这些测试没有向下一行注入期望中间态，也不声称 Go 人物 API 自行完成 child 生命周期。完整世界和 sprite／实际绘制仍由 Root 的独立验收负责。

## 复现与记录

固定原版输入和唯一 control 已位于仓库；collector 不重新生成 control：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter4_duo_composition.py --out .local/md-yuyuhakusho/fighter4-duo-composition
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" go test -race -count=1 -json -run '^TestFighter4DuoComposition' ./games/yuyuhakusho/engine
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" GOOS=js GOARCH=wasm go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestFighter4DuoComposition' ./games/yuyuhakusho/engine
```

先行缺失夹具红灯、冻结证明、采样摘要及最终两端日志位于 `testdata/fighter4-duo-composition-*`。最初 wrapper 的 `ai_calls` 字段声明遗漏在冻结前修正；该编译失败也已保留。最终 validation 绑定三组新 CPU 数据、其 manifest、固定输入／control、采样器及依赖、验证时生产和适配器 SHA、两端日志和清理记录。

只在确认三组 gzip 与 canonical 文件逐字节相同后，删除本任务 Duo 原始采样目录和重复摘要；同时归档并删除已闭合的旧 metadata 诊断临时目录。清理范围和字节数见 `testdata/fighter4-duo-composition-cleanup.json`。共享缓存、ROM、工具、其他 agent 产物、Root 日志及 8799 页资源未修改。
