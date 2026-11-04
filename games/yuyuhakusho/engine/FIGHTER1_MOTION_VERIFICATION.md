# 藏马 motion：实现前验证范围

范围为来源 1 的 `1a/1b/1c/1d`（`1aac2/1a8f0/1a9be/1ab92→15afc`）和共享 `0f/11` 的新来源上下文。API 使用最小 `Fighter1MotionState{ChuSpecialMotionState; EffectExtras[16]ReactionEffectExtra}`／`StepFighter1Motion(state, JumpParameters)`；本范围不写 PendingSound，不新增该字段或任何 canonical 存储。四次 FX 初始化通过临时 F0 视图复用既有 helper，只回写 Pool／Extras。

实现前失败场景：仅改 tag 但沿用其他来源三资源指针；`1a` 帧2／ticks0主槽门槛、VX高word偏移和父／子小数保留错误、属性OR6000或pose9错误、意外清旧子速度／Power；`1b` 完整父VX复制与非零facing取反错误、Y−0x34／X±0x20误用十进制、属性仅加0x60误套mask、未写子字段误补齐；`1c` 完成／ticks1／触发帧优先级错误、frame0四FX先于Power游标改写及busy检查的顺序错误、Power word加0x100／到0x300清零／word回绕错误、busy阻止游标推进或回滚四FX、属性只加0x40+游标>>4误套mask、把父X写child.X而非原child.VX、子X／Pose／Power保留错误；`1d` 分段常量位移、帧18..26残影、非零facing、父存储VX保留错误、帧18先位移再contact／48tick分支、超时直接27和接触增帧顺序错误；四FX仅第1／4成功写Duration8和部分池游标／旧字节丢失；无效派生游标晚期错误泄露先前FX／Power；真实单初态流注入期望状态；漏比父三子FX／资源／声音／RNG或冒充完整child／actor／world。

先完整原CPU自然／dirty边界／实际动画时钟流，再冻结四项测试与真实缺实现红灯，最后新Go。原菜单普通／特殊输入与未观察core参考帧由Root／selector统一，本采集器独立观察core逐视频核对RAM／画面，资源指针来自来源1真实初始化。私有canonical A5、`FFFF00` 下栈、refresh-cycle sidecar恢复和全RAM改写审计均保留。静态入口来自既有直接Ghidra清单；REA不支持raw68000，不称为REA反编译结果。

## 当前验证结果

| 证据 | 调用／步骤 | 实际范围 |
| --- | ---: | --- |
| 真实普通／特殊自然流 | 329 | 共享 `11` 33／`0f` 4、`1a` 57、`1b` 39、`1c` 59、`1d` 137，全部接受槽有自然记录 |
| 完整原 CPU dirty 矩阵 | 4,272 | `1a` 480、`1b` 480、`1c` 1,032、`1d` 2,108、共享 `0f` 108／`11` 64 |
| 单初态实际时钟流 | 22 条、1,570 步 | 11 组空／满 FX 池版本，全部恢复站立 |
| 独立 Go 对照 | 4 项顶层测试 | 最终 native `-race` 11.988 秒；Wasm 3.269 秒，全部通过 |

普通与特殊观察 core 分别从冷启动执行来源 1 的真实菜单输入，与 selector 保存的各自独立未观察原 core 全部 3,602 参考视频逐帧 RAM／画面一致。特殊输入 `fighter1-special-inputs.json` 的 SHA-256 为 `4c4ebc2a611db9d98337010cc5d38eea6cabeaf2bf60826387a0f34733a1fd8d`。四个专属槽自然完整调用分别得到 57／39／59／137 次，并运行至原恢复；`1d` 在第 3,115 个视频标签已恢复 stand。父动画／形状／参数指针来自实际来源 1 初始化，输入、参考帧和初始化绑定在 manifest 中。每次完整父人物 256 bytes、三保留子槽 768 bytes、FX 池 2,048 bytes、游标和 Duration extras 均比较；未拥有的旧字段由原字节适配器保留，无父／子／池字节屏蔽。原 RNG／声音请求在 motion 中不改写，独立音频队列只能由实际消费者标记解释。私有返回 SP 和全部 64 KiB RAM 改写范围审计通过。

初始原 CPU 私有／时钟夹具和四项测试先于生产，真实缺少 `Fighter1MotionState`／`StepFighter1Motion` 的 red 保存在 `testdata/fighter1-motion-red.jsonl`；`-gcflags=-e` 也保留并行范围的独立缺符号，没有临时桩。冻结测试 SHA-256 为 `d4687c0085f8b9dfa5dd9d1862300c36bbc1bba98b18f872e627e581158c7223`，首次可执行规则验证全部通过，无生产语义修正。特殊自然流在生产／四测试冻结后独立追加，同四项 native／Wasm 刷新仍全部通过，没有改动 Go 或断言。完整来源和日志见 `testdata/fighter1-motion-validation.json`。

## 初始化与正常副作用

`1a` 在帧 2／ticks0 主槽空闲时创建 `12dd4`／pose9；属性先加 `0x40` 后 mask／OR `0x6000`，X只按父VX高word和非零facing偏移，旧子VX／VY／Power保留。`1b` 同触发点创建 `12ee8`，属性只加 `0x60`，完整父VX复制后按facing取反，Y高word减 `0x34`，X高word偏移 `±0x20`；不补旧子VY／Pose／Power。

`1c` 完成分支优先；其余仅 ticks1 且帧 `0/1/4/8/10/11` 处理。帧 0 先进行四次独立 FX 申请，只有第 1／4 次成功写 Duration8，其他旧字节保持；再增加 Power word `0x100`，到 `0x300` 清零，派生 `+0x200/+0x300/+0x100` 后才检查 busy。正常 busy 保留已完成 FX 和游标。成功建立 `1300c` 时原样将父 X long 写在子 `+0x10`，子 X／Pose／Power均不写；Y高word减 `0x10`，子纵速为 `−0x28000`，属性仅加 `0x40+(cursor>>4)`。

`1d` 不读取或改写存储 VX：帧 18..26 使用 `0x40000` 位移且每四 ticks 尝试残影，27 为 `0x30000`，28 为 `0x20000`，29..31 为 `0x10000`，其后为零。非零facing取反。帧 18 先位移，再以 ticks≥48 优先跳 27，否则 contact.bit0 清ticks并增帧；正常满池仍继续本步运动。

## 连续流与范围

每条流只初始化一次，原 `19c96` 与独立时钟按实际记录推进，并从各自当前 pose／index 选择同一只读原记录，仅刷新 Duration／Flags；没有期望状态回灌。主槽 busy 的 beam、不同速度／朝向的 `12ee8`、A0／A1连射及三槽全 busy、接触／超时分段冲刺和共享冲刺／后退都自然结束。步数分别为 beam57、projectile39、burst59／91、ultimate接触137／超时153、共享dash29／retreat33，每种分别空／满池。

本阶段不执行共享尾段、形状提交、子／FX生命周期或世界；完整 actor/world、创建对局和浏览器验收由后续组合补齐，不将规则 phase 通过写成完整玩法验收。

## 复跑

从 worktree 根目录运行；采集需要固定原 core／ROM和原参考视频，Go 验证只读取规范夹具：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter1_motion.py \
  --out .local/md-yuyuhakusho/fighter1-motion-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighter1Motion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighter1Motion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

最终只清理本任务重复采集／stdout，删除摘要见 `testdata/fighter1-motion-cleanup.json`。规范原 CPU 数据、真实 red、最终日志及原参考帧均保留；共享缓存、其他任务和 8799 资产未动。
