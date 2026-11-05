# 树（来源 7）selector／构造验证

本范围恢复 `17790/178ea/17960/17a08` 四个 selector 的完整返回，以及所达普通攻击、投掷、前后冲与树专属构造。继续动作、子对象、组合调用、碰撞和完整对局不在本页通过范围内。

## 先行证据与测试

失败场景先写入 [SOURCE7_ATTACKS_FAILURES.md](SOURCE7_ATTACKS_FAILURES.md)。普通输入阶段只使用主任务冻结的 original-core 3602 帧 control：

- 输入：`source7-next-inputs.json`，SHA-256 `fed38aa84737d5d17bdf7d02f78f109ec0836f1cb244de6e2def47a8b9fc904f`。
- 帧序列：`engine/testdata/fighter7-attacks-reference-frames.json`，SHA-256 `ffebd6741b4a5a44a76832fc658045e144fd3607fec5fefd563a01be1809a65d`。
- ROM、原 core、hook core 分别保持会话固定的 `c39d52…c06`、`1d08b7…95da`、`e87739…7306` 身份。

hook core 的 3602 帧 RAM／视频均与该 control 一致，没有额外采集 original control。四个数据集完成后，冻结专属测试和夹具；`fighter7-attacks-test-first-freeze.json` 绑定其来源。真实 own-symbol red 日志在 `engine/testdata/fighter7-attacks-own-symbol-red.log`，失败原因仅为尚不存在的 `Fighter7AttackState`／`SelectFighter7Attack`。

冻结测试 SHA-256：`454f1c19afb68202377d75206f8fb5064a834cbc41a09e3b2209153694169fc4`。实现之后没有改变这些断言或夹具。

## 原 CPU 结果

| 数据集 | 完整调用 | 结果 |
| --- | ---: | --- |
| 普通自然输入 | 1681 | 四 selector 均达到；ground 722、ground-special 753、air 103、air-special 103 |
| 声明的代表初态 | 57 | 四对象地址；扣费成功／拒绝、主子槽阻塞、弱生命条件 |
| 脏内存边界 | 11164 | 22 个构造／投掷入口，12 个本范围根均有实际成功输出 |
| 单初态持续流 | 1040 | 52 条流，每条 20 步；仅注入声明的三个基础输入通道 |
| 特殊动作自然输入 | 2124 | 5502 帧完整对照；14 个玩家可达根各有一次真实成功 |

每次返回均绑定 SP 平衡、同一 A5、完整父体字节、四父体与三个子槽、能量小数、RNG、效果池和投掷缓冲。CPU 64 KiB 的变化逐 byte 归属，未解释变更为 0。普通自然调用含 94 个实际 IRQ span 和 4 个 `37c56/37cac` 外部声音消费标记，IRQ 地址保留原始 32 位值，并按顺序串联多次写入；没有用未知区域掩码通过。私有调用使用 SR2700，无 IRQ。

`03d8` 返回哨兵的 hook 位于指令执行前。该原 ROM 指令为 `JSR 219ea`，停止执行器前仍会将已经消费的栈 4 byte 改为 `000003de`。采集结果取哨兵入口处的完整 selector 返回；这一次执行器栈副作用单独检查，不算进 selector 状态。

失败清单中的空中 A 方向是假设，冻结的原 CPU 用例 `air-2-64-0` 否定了只列 `0/4/8` 的分组。实际为 Held 低四位 `0/2/4/6/8/10→95`、其余 `→94`：两个不同的间接跳转目标最终到达同一个 `18930` 构造。首版实现被该冻结用例挡住，仅修正实现后通过，未修改测试。

## 特殊动作自然成功

追加阶段先写入 [特殊动作失败清单](SOURCE7_ATTACKS_SPECIAL_FAILURES.md)，再新增独立 `capture_fighter7_special_attacks.py`。原采集器、测试及 14 份冻结来源均保持原 SHA，生产实现未改。

本阶段只使用 Root 冻结、未附加 observer 的 5502 帧 original control。入口截止 5500，另有 2 帧排空。输入 `source7-special-discovery-inputs.json` 的 SHA-256 为 `5354fe1e66a6875483398763ce7644c363682e864537f827786fd42b0fab05fd`；参考帧 SHA-256 为 `98a17dffa4906b63a6ad2e309c7cfb402ca38e0a46fdfce7a5b0bd81456d55dd`。冷启动 hook 的全部 5502 帧 RAM／视频逐帧匹配，没有注入游戏 RAM、寄存器、周期或中间状态。

| 模式与请求 | 按钮 | 成功帧 | 实际成本 |
| --- | --- | --- | --- |
| 地面 R2 | C／B／A | 1893／2007／2070 | 各 16 |
| 地面 R1 | B／A | 2157／2226 | 各 16 |
| 地面 R3 | C／B／A | 2326／2409／2492 | 各 16 |
| 空中 R2 | B／A | 2669／2720 | 16／20 |
| 空中 R3 | C／B／A | 2781／2876／3028 | 各 16 |
| 地面 R6 | Held.Down | 4038 | 200，Energy 201→1，Health 96 |

14 个根均核对自然调用的请求、按钮优先级、构造入口、动作／姿态、请求清零、能量小数保留及完整返回。P1 为 source7／`8400`，AI9 父体为 `9000`。R2 主子槽空、R1 VX 的方向、Aux48 最高 byte、VX 高 word 等上下文均通过；各成功帧与 Root 的发现记录一致。

本阶段另有 108 个 IRQ span、16 个声音消费标记，CPU 64 KiB 未解释变化为 0。完整夹具 SHA-256 为 `3230d5276627aca9a3edcf6b1cb56b7566540b69db3e64dc130a0c3738e6a6f9`，gzip SHA-256 为 `5cb4024e31563db2e18b919a0d890313425bcec9b213bd9adad7504c3b995dd3`。明细在 `engine/testdata/fighter7-attacks-special-natural-manifest.json`；独立追加绑定在 `fighter7-attacks-special-evidence-freeze.json`。

## 重复执行

采集只读 ROM，写入指定工作树输出目录：

```sh
PYTHONDONTWRITEBYTECODE=1 TMPDIR=$PWD/.local/md-yuyuhakusho/tmp \
python3 scripts/yuyuhakusho/capture_fighter7_attacks.py \
  --out .local/md-yuyuhakusho/fighter7-attacks-replay \
  --inputs docs/research/md-yuyuhakusho/source7-next-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter7-attacks-reference-frames.json \
  --natural-entry-limit 3600
```

追加的特殊动作采集命令：

```sh
PYTHONDONTWRITEBYTECODE=1 TMPDIR=$PWD/.local/md-yuyuhakusho/tmp \
python3 scripts/yuyuhakusho/capture_fighter7_special_attacks.py \
  --out .local/md-yuyuhakusho/fighter7-attacks-special-replay \
  --reference-manifest games/yuyuhakusho/engine/testdata/fighter7-attacks-special-reference-manifest.json
```

正式包级测试：

```sh
env -i PATH="$PATH" GOCACHE=$PWD/.local/go-cache GOMODCACHE=$PWD/.local/go-mod \
GOTMPDIR=$PWD/.local/md-yuyuhakusho/tmp TMPDIR=$PWD/.local/md-yuyuhakusho/tmp \
go test -race ./games/yuyuhakusho/engine -run '^TestFighter7Attacks' -count=1 -v
```

Wasm 使用同一正式包、最小环境和短匹配式，避免 Go 的 8 KiB 启动参数限制：

```sh
env -i PATH="$PATH" GOCACHE=$PWD/.local/go-cache GOMODCACHE=$PWD/.local/go-mod \
GOTMPDIR=$PWD/.local/md-yuyuhakusho/tmp TMPDIR=$PWD/.local/md-yuyuhakusho/tmp \
GOOS=js GOARCH=wasm go test ./games/yuyuhakusho/engine \
  -exec /opt/homebrew/Cellar/go/1.26.2/libexec/lib/wasm/go_js_wasm_exec \
  -run '^TestFighter7Attacks' -count=1 -v
```

追加特殊动作证据后，正式包级 native race 与 Wasm 均通过，分别用时 62.930 秒、15.041 秒，没有屏蔽其他新测试文件。最终日志为 `engine/testdata/fighter7-attacks-special-native-validation.log` 与 `fighter7-attacks-special-wasm-validation.log`，SHA 绑定在 `fighter7-attacks-validation.json`。总计 16066 次原 CPU 调用均通过。旧冻结的 14 个来源／夹具／测试文件及新增 7 项证据绑定保持原 SHA，生产实现未改。

已清理本范围原始 JSONL 中间文件和与 canonical 夹具相同的 `.local` 副本：普通阶段约 457 MB，特殊动作追加阶段约 74 MB。保留压缩夹具、manifest、own-symbol red 和最终验证日志；没有清理其他范围或用户原始素材。

## 状态所有权与限制

API 只增加嵌入 `ChuAttackState` 的临时 `AlternateVelocity int32`，对应唯一 `WorldObject.Aux48`。R1 清 Aux48 最高 byte，R3 写最高 byte，低 24 位保留；R2 只写 `Body.VelocityX` 高 word。没有重复 variant、counter 或 FE 存储。能量的有符号高 word 与低 word 分开观察，最终运行代码不依赖 ROM、原 core 或 CPU VM。

普通自然输入只观察到普通攻击成功；追加的特殊动作输入已覆盖全部 14 个玩家可达根。R4 没有玩家命令绑定，继续仅有私有证据。selector 成功不等于子回调、回血、继续动作、碰撞或完整世界已验收，这些范围由各自记录验证。
