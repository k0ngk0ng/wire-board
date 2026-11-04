# 幽助核心 motion：实现前验证范围

恢复来源 3 的 `13/21/23/24` 与结束构造 `1900c`，复用已有 `0f/11/15/17` 并验证新来源上下文。范围从分派入口到 `15afc`，不执行共享尾段或子／FX 后续；`22+1863a` 由独立任务恢复，本模块不覆盖。真实菜单已确认来源 3 为「幽助」，不按名单顺序推断身份。

实现前失败场景：只改 tag 而沿用其他来源的参数／动画／形状表；`13` 在第 0 帧错误移动或建立尘土、帧≥2减半先于本步位移、朝向 bit0 与完成 Flags.bit2 恢复错误；`21` 的 26 项表／跳组边界越界，错误把 long.bit24 当 long.bit0、将锁单独存储、忽略先按 `Pressed&0x70` 置锁、在组帧清锁前错误检查 ticks、No-op 表项意外清锁、完成后站立再清锁顺序错误；`23` 混用幻海的成本16和恢复槽、成本4不足时错误回滚已清 Completed 或擅自站立、成功构造未保留速度低 word／Gravity低24 bits、漏掉 Power255和模式byte0；`24` 在错误帧或 ticks 比较前后 OR A、丢掉扩展 pressed byte、B／C优先级错误、把 ActionKind 临时改17伪装共享调用、漏回写 Controls；正常主子槽／释放光效 busy 回滚已完成弹速与建立；独立诊断错误泄露锁位或 FX 修改；能量小数、word回绕、声音和 RNG 被省略；真实时钟流注入期望状态或把 phase 覆盖写成完整 actor／世界验收。

接口采用透明别名 `Fighter3MotionState = Fighter0MotionState` 和 `StepFighter3Motion(state, JumpParameters)`。连击锁仍由原 `Body.VelocityX` 的 bit24 持有；弹速和蓄力模式仍使用 `AlternateVelocity` 与 `Body.Gravity`。没有新 canonical 字段。`24` 的 `Controls.Pressed` 是实际输出，组合层必须完整合并回去。

先保存原 CPU 完整父人物／三子槽／FX及Duration extras／请求／声音／资源和 RNG 输出及单初态实际动画时钟流，再写缺实现测试，最后实现。自然流程复用 selector 任务独立未观察 core 的普通与 special 基线，再由本采集器独立运行观察 core 并逐帧比较 RAM／视频；两流保留各自来源。私有栈位于 `FFFF00` 下，并核对完整 RAM 改写范围及返回 SP。入口／表来自已有直接 Ghidra 清单，REA 不支持 raw 68000，不能称为 REA 反编译或执行结果。

## 验证结果

| 证据 | 调用／步骤 | 覆盖 |
| --- | ---: | --- |
| 两条真实菜单自然流 | 266 | `11` 33、`21` 91、`17` 142；其他本模块接受槽没有自然记录 |
| 私有完整原 CPU 调用 | 4,164 | `13` 576、`21` 668、`23` 856、`24` 1,520；共享 `17` 288、`0f` 108、`11` 64、`15` 84 |
| 单初态实际时钟流 | 26 条、1,632 步 | 13 组情境各有空／满 FX 池版本，均返回站立 |
| 独立 Go 验证 | 4 项顶层测试 | native `-race` 11.356 秒，Node/Wasm 3.185 秒，全部通过 |

每次比较完整父人物 256 bytes、三子槽 768 bytes、FX 池 2,048 bytes、游标、Duration extras、声音请求、能量小数及全部 typed 状态，没有人物／子槽／池字节屏蔽。原 CPU motion 没有消耗或改写共享 RNG。两行自然调用有实际独立音频消费者标记；只解释被 `37c56/37cac` 与 `F80A..F80D` 证明的队列改写，声音请求仍完整比较。私有调用同时审计完整 64 KiB RAM 改写范围和返回 SP，未出现未知写入。

原 CPU 基线分别为普通输入 3,600 视频步和 special 输入 3,602 步。固定 ROM／未观察 core 的独立参考帧、输入和 manifest 均按 SHA-256 绑定；本采集器从冷启动重新运行观察 core，全部 RAM／视频摘要吻合。参数、父动画和形状指针来自真实来源 3 初始化，不能用其他来源改标签代替。

原 CPU 夹具和四项测试先于生产实现，真实缺少 `Fighter3MotionState`／`StepFighter3Motion` 的日志见 `testdata/fighter3-motion-red.jsonl`。测试初始／最终 SHA-256 均为 `11f22937535ae279283da3dc4cf5768a58a1c254411c7ec34cbf681e8547dac8`；首次实现即通过全部规则对照，没有按 Go 结果改写原预期。来源和日志绑定在 `testdata/fighter3-motion-validation.json`。

## 顺序与唯一存储

`13` 帧 0 不移动；后续帧在 ticks0 先建尘土再按朝向 bit0 位移，帧≥2 才在本步位移后算术减半。完成时按原 Flags.bit2 站立／蹲下恢复。

`21` 先按本次 `Pressed&0x70` 置 `VelocityX.bit24`，再读取 26 项表。只有 `3/5/7`、`11/13/15`、`19/21/23` 三组有跳帧逻辑：锁已置时先清锁并保留当前帧，检查不依赖 ticks；没有锁且 ticks0 时才分别跳至 `9/17/25`。其他表项保留锁；完成先站立，再清锁。其余 31 bits 始终保留，诊断越界时连先置的锁也原子回滚。

`23` 在完成分支先清 Completed，再尝试 `1900c` 的成本 4。正常不足保留这次完成消费，继续保持 `23`，不报错、不回滚、不进入站立。成功只改所拥有部分：进入 `24/a4`、Power255、Gravity 最高 byte 清零、横／纵速度高 word 写 32／−48，并保留能量小数与速度低 word。其他持续分支保留原帧 31 回复、帧 0 位移／残影以及 64 tick 超时优先级。

`24` 在原始帧 4／ticks16 时先 OR `Pressed.A`，保留扩展 byte 和其他基础位，然后进入真实共享 `1a430`。不能先让共享主干把 ticks 与 `0x3f` 相与后再判断；例如原 ticks80 不触发 OR。共享主干继续保留原 B／C 优先级、正常 busy 及部分建立语义，ActionKind24 始终保留到真实转换。返回 Controls 必须由完整 action／actor／world 合并回去。

## 连续与剩余范围

每条流只初始化一次，原 `19c96` 与独立 `StepAnimationClock` 自行推进；每次 phase 后按各自当前 pose／index 选择同一份只读原动画记录，仅刷新 Duration／Flags。没有注入期望人物、子槽、FX、时钟或 RNG，没有运行共享尾段／形状提交／子或FX生命周期。

无输入连击流 47 步恢复，每两步预定 B 脉冲的连击流完整延续至 83 步；`23` 接触后经过完整原动画、成本4构造和 `24` 自动释放，165 步恢复；直接自动释放 64 步，释放光效 busy 后预定 C 取消为 105 步。其他共享新来源上下文、滑动和普通充能也分别连续验证。空中 `22+1863a`、完整人物、世界、新建对局和浏览器运行继续由独立组合任务验收，本报告不覆盖。

## 复跑

从 worktree 根目录执行，采集需要已有固定研究 core／ROM及两组参考帧；Go 验证只读取规范夹具：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter3_motion.py \
  --out .local/md-yuyuhakusho/fighter3-motion-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighter3Motion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighter3Motion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

最终仅清理本任务重复采集／stdout 文件，规范夹具、红灯、最终日志、两组参考帧和来源说明均保留。精确清单见 `testdata/fighter3-motion-cleanup.json`；共享 Go 缓存、ROM／工具、独立 `22` 任务及 8799 资产未修改或删除。
