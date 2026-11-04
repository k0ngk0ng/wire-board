# 幻海持续动作：实现前验证范围

恢复来源 0 的新动作槽 `0e/16/17/18/19`，复用已验证的 `11` 和 `15`。范围从各动作分派入口到共享尾段前 `15afc`，维护完整父人物、三个保留子槽、FX 池／游标、声音请求、能量小数和 RNG；建立 `12c96/134c2/1313e/132de` 子对象但不调度其生命周期。真实菜单输入为 `source0-next-inputs.json`，HUD 名称已确认「幻海」；来源号与平台席位不混用。

实现前失败场景：沿用其他来源的动画／参数指针而只改 tag；`0e` 的地面高 word 与小数、朝向 bit0 位移和任意非零尘土方向混淆；`16` 的 13 项帧表越界、按姿态 unsigned byte 的特效门槛、帧 3 四次独立分配、固定 `3000/4000` 重力增量、旧纵速符号、横速从正值减到负值的最后一次位移、落地先钳完整 Y 再跳帧和成对尘土顺序错误；`17` 错把能量 word 非零当正数、蓄力 word 回绕／钳制和小数保留错误、ticks 先与 `3f` 相与及声音时序错误、蓄力模式最高 byte 与低 24 bits 混淆、C／B／A／后方向取消优先级错误、主子槽忙仍须保留弹速改写、主子槽成功而 `+300` 忙时错误回滚或提前清蓄力；`18` 忽略 ticks 门槛和先写纵速／递增游标、轮转 `+200→+300→+100` 错误、忙槽阻止游标推进、子槽高 word／完整 long 写入范围错误；`19` 完成恢复的能量成本、帧 31 回复能量、帧 0 残影／位移、接触 bit0 和 64 tick 优先级错误；FX 部分分配时游标和旧槽字段丢失、把四分配中仅第 1／4 写 `+5a=8` 错铺到所有槽；无数据或非法实际查表／游标错误泄露中间改写；私有栈覆盖 `FFE0` 统计；逐步回灌期望状态；把私有流、子槽建立或静态清单写成自然完整招式／对局。

接口采用 `StepFighter0Motion(Fighter0MotionState, JumpParameters)`。状态在 `ChuSpecialMotionState` 之外只持有 `PendingSound` 和 `[16]ReactionEffectExtra`；`Gravity` 的最高 byte 仍由原 `Body.Gravity` 持有，弹速由 `AlternateVelocity` 持有，横／纵速度高 word 继续保留其偏移或游标语义。新增子槽建立所写字段均由 `ChuSpecialChild` 覆盖，不增加相同 canonical 字段的第二份存储。正常忙碌／分配不足仍提交此前正常副作用；接口诊断错误原子拒绝。

先取得完整原 CPU 自然／私有输入输出和真实动画时钟连续流，再写实际缺实现测试，最后实现独立规则。双内核自然对照复用同输入、固定 ROM 与未观察原内核的完整 3,600 帧基线，再独立运行观察内核逐帧核对 RAM／视频。静态入口复用已保存的直接 Ghidra 证据，`1fbaa` 常数来自只读 Capstone 5.0／68000 内存解码；REA 不支持 raw 68000，不能称为 REA 的反编译结果。

## 已验证结果

| 证据 | 调用／步骤 | 实际范围 |
| --- | ---: | --- |
| 两条真实菜单自然流 | 262 | `0e` 50、`11` 31、`17` 128、`18` 53；`15/16/19` 没有自然记录 |
| 明确私有原 CPU 调用 | 3,923 | 七个接受槽；包含能量／蓄力 word 边界、busy 子槽、FX 部分申请、全部帧表及发射游标 |
| 单初态真实时钟流 | 20 条、1,302 步 | 10 组情境各有空／满 FX 池版本，全部恢复为站立 |
| 独立规则测试 | 4 项顶层测试 | native `-race` 10.322 秒；Node/Wasm 2.735 秒，全部通过 |

每次比较父人物完整 256 bytes、三个保留子槽 768 bytes、完整 FX 池 2,048 bytes、游标、FX extras、待处理声音、能量小数及全部已建模状态，没有人物／子槽／FX 字节屏蔽。原 RNG 在全部 motion 调用中均未消耗或改变。两行自然调用存在独立音频清槽，只有实际 `37c56/37cac` 和 `F80A..F80D` 标记允许解释队列变化；声音请求 `A030` 仍逐行完整比较。私有调用另外检查完整 64 KiB RAM 改写范围和返回 SP，只有父／子、FX、请求／游标及 `FFFF00` 下的声明栈可以改变。

采集器复用来源 0 selector 的独立未观察内核 3,600 帧基线，并追加 projectile 任务已保存的 3,602 帧基线；各自输入、参考帧及 manifest 保持独立来源。两个观察内核冷启动运行的全部 RAM／画面摘要均与各自原基线一致。私有参数／动画／形状指针复制自真实来源 0 初始化，并绑定来源人物摘要；没有改其他来源的标签来假装幻海。

原 CPU 夹具和全部四项测试先于生产实现。真实缺少 `Fighter0MotionState`／`StepFighter0Motion` 的红灯保存在 `testdata/fighter0-motion-red.jsonl`。生产接口仍只有原状态与参数，没有原 CPU／ROM 解释器；来源、夹具、日志和当前源码绑定见 `testdata/fighter0-motion-validation.json`。本轮没有修改旧 Go 文件、数据 bundle、PROGRESS 或本地服务资产。

## 部分写入与顺序

`17` 在帧 4 先处理蓄力子槽，再按 word 非零能量更新蓄力，保留能量小数。ticks 先与 `0x3f` 相与，随后才判断每四步的 `0xa6` 声音请求和取消／发射。发射先改写完整弹速，再检查主子槽；主子槽建立后才检查 `+0x300`。忙碌属于正常结果，已完成的弹速、主子槽、蓄力和声音写入继续保留。只有成功建立释放光效后才跳帧并清蓄力。

`18` 的触发帧先设置父纵速，再增加父横速高 word 作为游标；高 word 轮转 `0→0x100→0x200→0`，对应依次使用 `+0x200/+0x300/+0x100`。低 word 保留；忙槽仍推进游标并提交父纵速。子槽只写原初始化字段：ground／偏移的 word 与坐标／速度的 long 各自按原范围改写，其他旧字段保留。

`1fbaa` 每侧独立调用既有申请器，X 高 word 偏移依次为 `−24/−8/+8/+24`，完整横速为 `−0x20000/−0x10000/+0x10000/+0x20000`，不按朝向翻转。只有第 1／4 次成功分配后写 Duration 8，中间两槽保留旧 Duration。这个 byte 通过已有 `EffectExtras.Phase` 回传，再由 canonical `Clock.Duration` 视图合并；没有新增平行存储。

## 连续流边界

每条流只初始化一次，原 `19c96` 与独立 `StepAnimationClock` 推进当前时钟。每次 motion 之后，双方按各自当前 pose／index 选择同一份已绑定只读动画记录，只刷新 Duration／Flags。测试没有注入期望时钟、人物、子槽、FX 或 RNG；该刷新不执行形状提交、阶段掩码或声音尾段，不能描述成完整 actor 更新。

两条 `17` 流分别在预定第 20 步按 B 发射或按 C 取消；`18` 流自然建立三个子槽，后续忙碌仍按原顺序轮转；`19` 分别覆盖 64 tick 超时与接触后完整动画、帧 31 能量回复、`189ae` 成本检查、转 `16/a3` 后落地恢复。原 motion、动画时钟和这些正常副作用都连续推进，未调度已建立子对象或 FX。更多完整人物／世界、子生命周期、画面和平台验收由后续组合完成。

## 复跑

从 worktree 根目录运行。采集需要已有固定 ROM、观察内核与两个原参考帧文件；Go 验证只读取规范夹具：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter0_motion.py \
  --out .local/md-yuyuhakusho/fighter0-motion-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighter0Motion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighter0Motion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

最终仅删除本任务重复采样和 stdout 中间文件；完整来源和删除摘要见 `testdata/fighter0-motion-cleanup.json`。规范夹具、真实缺实现红灯、最终日志、未观察参考帧、ROM／工具、共享 Go 缓存、其他任务及 8799 服务资产均保留。
