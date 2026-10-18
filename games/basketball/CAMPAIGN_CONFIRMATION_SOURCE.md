# 战役成员确认事务的原始夹具

本夹具为半场事务接线提供原选人／清理、角色提交／等级计算及后续四人物更新的独立证据。它复用4条已有正常菜单记录，执行未改写的原指令；没有重跑整场，也没有修改生产代码或编写替代规则。

文件为 `engine/testdata/campaign-confirmation.json.gz`（4行数组）和 `campaign-confirmation-source.json`。生成器为 `scripts/basketball/verify_campaign_confirmation.py`。

## 记录范围

| run | 选人前→自然后帧 | 提交前→自然后帧 | Mode／Period |
| --- | --- | --- | --- |
| campaign-audit-first-match-confirm | 769→770 | 1612→1613 | 0／0 |
| campaign-audit-coop-first-match | 769→770 | 1565→1566 | 1／0 |
| campaign-audit-equipped-home-match | 3410→3411 | 4194→4195 | 0／2，020010装备主场 |
| campaign-audit-loss-continue | 11009→11010 | 11086→11087 | 0／1，半场再次确认 |

所有帧编号从1开始，指已有 `ram.bin` 中的视频帧末快照。`sourceSHA256` 绑定每个完整原记录，不把它称为精确程序计数器入口的实时捕获。

## 选人和清理字段

每行的 `selectionBefore`／`selectionAfter` 包含：

- `mode`：原 `$058D`。
- `pending[4]`：待提交角色 `$03CB..$03CE`。
- `objectKind5`：菜单对象Kind槽5 `$03DB`。
- `menuScratch[9]`：`$06D7..$06DF` 的菜单工作字节。
- `random`：完整共享随机状态 `$0048..$004B`。

执行 bank3 `$896D→$8088`，包括原自动敌方选人及 `$89AC` 公共清理。输出与相邻自然后帧匹配，`selectionRandomCalls=1`。阶段单独记录为 `selectionPhaseBefore`／`selectionPhaseAfter`；后者为31。这里仍处于菜单流程，没有直接进入比赛。

## 角色提交、等级和配置

`commit` 保留以下语义字段：

- `characters[4]`：已提交的 `$03E3..$03E6`，按场上席位排序。
- `grades[4]`：每席位的 `run`、`jump`、`shot`、`recovery`、`attack`；对应 `$0615/$0619/$061D/$0621/$0625` 各4字节。
- `equipment[4][3]`：按原角色槽索引的三件装备，**不是按场上席位索引**；原255表示空槽，JSON为-1。
- `mode`、`teams[2]`、`court`、`period`、`winBits[2]`、`rewardSet`、`inventory[16]`。库存也使用有符号空槽-1。

原 `$B2EF..$B2F8` 提交角色，再由 `$B2FA` 调用一次真实 `$ACBB`。4例角色、全部20个等级字节及这些配置字段都与自然后帧一致，`commitGradeCalls=1`、`commitRandomCalls=0`。

## 核心提交的完整原RAM和写轨迹

`commitRawBefore`／`commitRawAfter` 为完整2048字节RAM的base64。核心执行从 `$B2EF` 延伸到 `$B32D`，保留上述角色／等级之外的原清理：

- 对象0～5的动作字节清零。
- 对象0～5的X速度**整数高字节**清零；低小数字节保留，不能把整个速度清零。
- 对象0～5的X坐标第三字节清零；其余坐标字节保留。
- `$06D7..$06DC` 六个场地／菜单别名字节清零。
- Scene `$0588=1`、Phase `$0589=0`。
- `$06CA..$06D1` 八字节清零。
- 球的投影Y低字节 `$0407=$F0`，不能覆盖其高字节或其他投影。

`commitWrites` 按执行顺序记录每次写入的 `pc/address/before/after`，包含同值写入；`commitWrittenAddresses` 为唯一地址集合；`commitStateWriteFields` 对已识别持久字段命名。CPU栈和工作字节仍保留在完整轨迹中，不能照搬为玩法状态。

## 四人物尾段不是纯绘制

`$B32D` 是后续四个菜单／结果人物更新的起点：`LDX #0`，然后反复 `JSR $C033`。`$C033→$FCB0→bank3 $801E→$9598` 包含动作初始化、`$95B4`动画、`$9769`运动和 `$C00C`投影／对象更新，不能跳过或称为无状态渲染。

另一个独立执行从同一 `commitRawBefore` 出发，完整运行 `$B2EF→$B337`，在末尾RTS执行前停止：

- `commitTailRawAfter`：完整核心加四人物更新后的2048字节RAM，base64。
- `commitTailStop`：`bank3:B337 before RTS`。
- `commitTailActorCalls`：严格 `[0,1,2,3]`。
- `commitTailWrites`：包括尾段的有序写入轨迹。
- `commitTailRandomCalls=0`。
- `commitTailObservedActorBytesMatched=100`：每例四人物动作、动画、坐标、速度、投影的100个字节均与自然后视频帧匹配；4例共400个字节对照。

尾段使用既有MMC3内存映射执行真实换库，不跳过C033或C00C，没有用常量补硬件读值。轨迹中的 `space="mapper-bus"` 表示MMC3寄存器总线写；该地址的CPU读值属于映射ROM，不能将其 `before` 当作硬件寄存器旧值。RAM写的 `before/after` 则是实际内存值。

核心B32D的 `commitRawAfter` 已与扩展尾段前保存的夹具逐字节核对，保持不变；新增尾段没有替换或弱化核心边界。

## RNG和组合限制

两段来自不同的真实视频时间，中间省略了原菜单等待、战术界面、介绍动作和调用者主循环。四例省略间隔分别842、795、783、76个视频帧。

- `selectionAfter.random` 是原选人输出，且与该相邻自然后帧一致。
- `commitRandomBefore`／`commitRandomAfter` 是从保留的提交前帧RAM直接启动独立B2EF探针的输入／输出，因该段不抽随机数而相等。
- `commitObservedAfterRandom` 是真实自然后视频帧的值，还包含调用者主循环影响，4例均与独立提交探针后的值不同。

因此不能断言 `selectionAfter.random == commitRandomBefore`，也不能断言 `commitRandomAfter == commitObservedAfterRandom`。上层若省略原菜单等待，可测试“当前共享RNG选人一次，提交和四人物更新不再消费RNG”的组合契约；不能声称这样的组合复现了两个自然视频帧之间全部原执行与时间。

本夹具是原源证据，不等同于半场生产事务已经完成。root负责先写事务测试、接入Campaign非空的确认路径及真实整局验证；普通Mode128路径不由这些正常战役记录授权改动。

## 再生成

在独立工作树根目录执行：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_campaign_confirmation.py
```

生成器校验固定ROM SHA、两段原CPU调用、相邻自然帧的语义结果、四人物尾段及RNG调用数量。它仅读取已有原记录，不启动整场采集。完整RAM探针使用SP255，保留其栈写入；这不是对原视频帧时CPU寄存器位置的假定。
