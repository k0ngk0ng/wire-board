# 桑原 motion：实现前验证范围

恢复来源 2 的 `1e/1f/20`（`1ac58/1ad28/1adc8→15afc`）和共享 `0f/11` 新来源上下文。只维护父体／三保留子槽及既有 FX 申请，不推进 `12dd4/12c96` 生命周期、共享尾段或世界。接口为 `StepFighter2Motion(ChuSpecialMotionState, JumpParameters)`，所有必需字段已有 canonical 所有者，没有新状态副本。

实现前失败场景：仅更改 tag 而沿用其他来源的参数／父动画／形状；`1e` 在非 frame4/ticks0 或主槽 busy 时建立，错误把完整 VX 当偏移、改写子速度／Power、word 坐标偏移误改小数、朝向 byte 非零条件混淆；`1f` 使用或重写父存储 VX、错误帧建立残影、未先位移再处理 frame4 contact.bit0 清 ticks／增帧、完成标志错误按 bit 测试；`20` 帧／ticks0/1/2门槛错误、先查 busy 才写纵速／游标、Gravity 高 word 轮转及低 word／word 回绕丢失、子槽顺序错误、初始化误用幻海 OR4000 而非 OR2000、把 Y−0x34／X±0x38当十进制、错清旧子 Power 或以高 word代替完整 VX复制；正常忙碌或 FX 不足回滚已完成副作用；非法实际游标／FX诊断错误泄露前置改写；漏掉全父三子FX原字节、声音、资源小数或 RNG；复用错误角色姿态／时钟，或连续流回灌期望状态；把阶段／子槽建立写成完整规则／画面／对局。

先取得原 CPU 自然与有界脏输入完整输出、实际动画单初态流，再写缺实现测试，最后实现独立 Go。自然过程复用同输入的未观察 core 原基线，独立观察 core 冷启动逐视频核对 RAM／画面；额外特殊输入保留独立来源。私有使用 canonical A5 和 `FFFF00` 下的栈，审计全部 RAM 写入与实际返回 SP。静态入口来自已有直接 Ghidra 清单；REA 不支持 raw 68000，不称为 REA 反编译结果。

## 已验证结果

| 证据 | 调用／步骤 | 实际范围 |
| --- | ---: | --- |
| 两条真实菜单自然流 | 252 | `0f` 25、`11` 33、`1e` 108、`1f` 33、`20` 53，全部接受槽有自然记录 |
| 私有完整原 CPU 调用 | 3,415 | `1e` 484、`1f` 1,460、`20` 1,299、共享 `0f` 108／`11` 64 |
| 单初态实际时钟流 | 18 条、782 步 | 九组场景各有空／满 FX 池版本，全部恢复站立 |
| 独立 Go 验证 | 4 项顶层测试 | native `-race` 8.990 秒；Node/Wasm 2.404 秒，全部通过 |

普通输入的 3,600 视频步基线和统一 projectile 输入的 3,602 步基线由 selector 任务独立原 core 生成；本采集器分别从冷启动运行观察 core，逐视频完整 RAM／画面摘要一致。两条输入、各自参考帧／manifest 保留独立摘要；特殊输入为 `fighter2-projectile-inputs.json`（SHA-256 `90d7d03e229a1cdd4eac98273f867f3c8118a34cfcdbe275d1932e630c529af8`）。私有资源指针从实际来源 2 初始化复制，不使用其他来源改 tag。

每次比较完整父人物 256 bytes、三个保留子槽 768 bytes、完整 FX 池 2,048 bytes、申请游标及全部已拥有字段，无人物／子槽／FX 字节屏蔽。未拥有的旧 Duration、速度、Power 和其他残留由原字节适配器保留并比较。私有调用额外审计完整 64 KiB RAM，投技／形状等本阶段未写区域也不得改写；返回 SP 必须恢复。motion 没有修改声音请求或消耗共享 RNG，独立音频消费者仅按实际标记解释队列改写。

四项测试先于生产，真实缺少 `StepFighter2Motion` 的日志保存在 `testdata/fighter2-motion-red.jsonl`。测试初始／最终 SHA-256 均为 `981c63e33db1f8a72754e3a8f1283402c6b82d0e2503467ab121ed426af7f06c`。第一次可执行原规则对照即通过，没有为 Go 结果改写预期；最终只删除一个无意义的字段读取 helper，再以相同四项 native／Wasm 验证绑定最终源码。详细来源见 `testdata/fighter2-motion-validation.json`。

## 初始化与正常副作用

`1e` 仅帧 4／ticks0 且主槽空闲时建立 `12dd4`。子 X 复制完整父 X 后只按父 VX 高 word 和非零朝向偏移，Y 复制完整父 Y。属性使用 OR `0x4000`，Pose 为 `0x0c`；旧子 VX、VY、Power 均保留。正常 busy 不改变状态，完成分支按原优先级恢复站立。

`1f` 在帧 4／5 使用 `0x40000` 位移并每四 ticks 尝试残影，帧 6／7 分别使用 `0x20000/0x10000`；不读取或改写父存储 VX。方向按整个 facing byte 是否为零判断。帧 4 的 contact.bit0 在本步位移之后清 ticks 并增帧，不能提前跳过本步运动。

`20` 在帧 5 的 ticks0／1／2 先写纵速 `0/−0x30000/+0x30000`，再增加 Gravity 高 word `0x100`、遇 `0x300` 清零，并保留低 word；对应 `+0x200/+0x300/+0x100`。派生游标决定目标后才检查 busy，正常 busy 仍提交父纵速／游标。字回绕 `0xff00+0x100→0` 也由原 CPU 验证。

成功的散射子槽使用 OR `0x2000`、Pose `0x0d`、完整父 VX 作为横速，并按非零朝向取反；Y 高 word 减 `0x34`、X 高 word 偏移 `±0x38`，小数保持。子 Power 等未写字段保留。复用前缀函数时明确区分这个 OR2000 与 `1e` 的 OR4000，没有套用幻海的错误属性。

## 连续流与剩余验收

每条流只初始化一次，实际 `19c96` 与独立 `StepAnimationClock` 连续推进，并由各自当前 pose／index 选择相同只读原动画记录，仅刷新 Duration／Flags。不回灌期望状态，不运行共享尾段、形状／声音提交或子／FX 生命周期。`20/a8` 的原帧 5 持续 40 ticks，进入该帧的 ticks0／1／2 连续触发三发；全 busy 流同样验证纵速和游标继续推进。

主槽 busy 的 `1e`、`1f/a4` 与带接触的 `1f/a5`、两种弹速／朝向的 `20`、全 busy 散射、以及新来源的 `0f/11` 均运行到站立。新子 `12dd4`、散射 pose13 后续、完整 actor／世界、新建对局及浏览器验收由独立任务继续组合；本报告不覆盖生命周期或画面可玩性。

## 复跑

从 worktree 根目录运行；采集需要已有固定原 ROM／观察 core 和两个原参考帧，Go 验证只读规范夹具：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter2_motion.py \
  --out .local/md-yuyuhakusho/fighter2-motion-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighter2Motion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighter2Motion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

最终只清理本任务重复采集／stdout，中间产物清单见 `testdata/fighter2-motion-cleanup.json`。规范夹具、真实红灯、最终日志、两组原参考帧／输入均保留；没有删除共享缓存或更改其他任务／8799 服务资产。
