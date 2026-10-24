# 持篮筐共享动作验证

实现前失败场景：31／33 走动和刹车不能保留原动作及动画；37／38 在动画游标 1 丢失原篮筐选择、朝向和独立方向字段；第六个篮筐及未匹配所有者的原缺省选择错误；载人／附着分支与地面收尾顺序不一致；动作 7 抛出后下一物体阶段仍拒绝运行；篮筐方向被错误地从朝向推断；首次进入与持续飞行重复初始化；完整动作阶段或连续释放到落地与原指令执行不同。

使用原 ROM bank6／fixed 指令的受控执行生成逐字段对照，保留 ROM、初始 RAM、夹具摘要与脚本。隔离状态及动作阶段链不声称为自然整场比赛；不得跳过原版物理或重置状态来通过。

## 对照结果

- 1,632 次单对象调用：1,504 次角色走动、刹车及站立／空中抛筐动作，另 128 次 B117 辅助物减速。后者是载人落地的直接衔接，覆盖两物体、首次／已进入、左右朝向及不同速度符号与过零。
- 440 次完整动作阶段：432 次篮筐动作 7 的各槽位、首次／已进入、朝向、九种独立方向和空中／地面组合，另 8 次纽约球场附着角色的生产快路径。
- 24 条受控初始状态下的原指令连续动作链：站立和空中抛筐、左右、六个篮筐；每条连续执行 19 或 24 次动作阶段，直到篮筐落地静置。每轮的 processed 位通过原 bank5:8469..8498 重置，再运行 fixed:D345..D348；不手工修复中途状态。
- 所有用例比较完整语义状态、对象访问顺序和声音事件。纽约附着用例进一步揭示并修复了旧 `positionCarriedHoop` 对掉落／骑乘姿势的拒绝，现复用已恢复的完整 ECF5／ED65 偏移表；载人落地接续 B117 的动作 4。

孤立篮筐阶段将无关球与其他篮筐设为合法静置状态，避免把移走所有者后的无效旧持筐状态混入测试。初始 RAM 的这些选择记录在生成脚本中，不声称其为自然比赛。实际线上存档到整场结算由另一组事故回归覆盖。

篮筐方向使用 `HoopHeadings` 保存原 RAM 的独立字段。旧存档缺字段时沿用 Go 的零值，不改变既有恢复要求。当前测试中的原始篮筐附着字节均为 16，动作 7 的原写入仍是 16；未把未知附着值宣称为已覆盖。

## 复现

以下命令从仓库根目录执行，原指令夹具生成需要已有的原 ROM 与参考 RAM，测试本身只使用提交的夹具。

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/verify_hoop_shared.py
go test ./games/basketball/engine -run '^TestHoop(SharedOriginalCalls|LobOriginalPhasesAndChains)$' -count=1
go test ./games/basketball/engine -run '^(Test.*Hoop.*|Test.*Held.*Pose.*|TestWorldActionsOriginal.*|TestWorldActionsEntireCapturedCorpus|TestOriginalAuxiliaryLobLaunchAndFlight)$' -count=1
```

两组测试均已通过，记录保存在 `.local/basketball-hoop-shared/green.log` 和 `regression.log`。原版来源与夹具 SHA-256 在 `engine/testdata/hoop-shared-source.json`。REA 指令证据为 `ev_1ad622c6f6d8c5c1804e844fa9f245547533fd4359788aab647902a76e77679a`，完整副本在 `.local/basketball-hoop-shared/rea-handlers.json`；其中地址为文件偏移，加载地址为 0x8000。
