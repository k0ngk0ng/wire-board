# 全电脑冷配置长运行

## 实现前失败场景与固定范围

- 新建独立测试，默认纽约球场、teams0/1、角色与装备不变，固定seed0x12345678，四席ControllerAssignments均为-1。保留原1人闲置＋3电脑cold6000和视频时钟测试不变。
- 从NewOriginalMatch及原开球阶段构造状态；必须核实四席实际Controls均为128，不能只修改测试名称或绕过真人输入。
- 目标固定6000逻辑更新，完整初始化后独立重复两次并比较每步状态累积摘要。逻辑更新不能称为完整比赛时间。
- 任一原AI未恢复分支必须在首差处失败，保留更新编号与完整前态；不换seed、截短目标、跳过动作或添加自创策略。
- 物理、接触及辅助对象动作的首差交对应负责者，避免重复恢复或根据结果猜行为。
- 原函数隔离对照和自然原运行证据分别报告；此cold测试本身不是自然原机轨迹对照，不把可持续运行等同于完整复刻证明。

## 首差与恢复

首次执行在update519的actor0输入失败；诊断确认mode28 A95F。恢复后同一阶段actor1暴露配对mode29 AA2C。完整前态及失败输出保留在`.local/basketball/rea/all-computer-cold/first-failure.json`、`first-mode.log`和`after-mode28.log`，没有删除失败记录或换种子。

mode28／29分别按原函数和辅助函数恢复，通过15360／9176组原程序隔离矩阵。既有自然18,000帧参考两个入口均未命中，明确不计自然对局覆盖。没有接管辅助物理或prop动作13，也没有以AI直接修改球权、链接、分数或碰撞结果。

## 当前完成结果

新测试`TestAllComputerColdWorldSixThousandUpdates`固定四席Assignments=-1，并断言实际Controls=[128,128,128,128]。原生与Wasm各独立重复两次6000逻辑更新，四次轨迹摘要均为`80f86c66d2535152a3be34a720b8a1f25e57a33f248f0214970ae962a31d2cdd`，最终scores=[6,9]。

原mixed测试源码与目标没有改动，回归仍是两次6000，scores=[12,55]及原摘要`4a81c896e779e4bdec6ca7d8bd461c0fb92cdc20bc18a8e3ec580b4794afcef8`。原1998次连续更新、57次主动更新轨迹也保留，原生竞态与Wasm检查通过。默认视频长测没有修改。

可复跑结果独立保存在`.local/basketball/rea/all-computer-cold/{native-result.json,wasm-result.json}`和相邻JSONL；并行回归日志为`regression-race.json`、`regression-wasm.log`。命令的默认模式仍验证原mixed配置，只有显式`--all-computer`才选择新测试和独立输出目录。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_cold_runtime.py --all-computer
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_cold_runtime.py --all-computer --wasm
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestAITeamSupport|TestAIRiderApproach|TestAllComputerCold|TestColdConfigured|TestWorldLoop' -count=1
```

此项只覆盖固定纽约默认球队的全电脑冷配置6000逻辑更新；不等于视频完整比赛、全部控制器模式或整个游戏已经复刻完成。
