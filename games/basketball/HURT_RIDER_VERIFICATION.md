# 骑乘人物受击动作 49

实现前列出的失败场景：

- 动作 49 被通用附着检查拦截，或简单忽略附着后继续物理，导致位置被支撑者重新吸附。
- 提前清空附着，遗漏原 `$8B67` 对 Kind12 支撑物的动作与速度写入。
- 辅助目标先进入动作 11，随后支撑物处理可能覆盖动作；合并命令打乱先后顺序。
- 支撑物已附着、空中或地面时的分支混同；修改普通球员支撑者。
- 球权、掉落速度、特殊持有者、搭档链接直到物理后才提交，导致运动阶段读取旧值。
- 已进入的动作重复解绑或重复初始化；非 49 受击动作因拆分而变更。
- 准备命令在物理后重放，覆盖场地处理的真实结果。
- 原生运行可推进，但 JSON 恢复或 Wasm 结果不同。

真实失败：固定配置场地 1、球队 5／6、球员 2／3／0／1，在 video 11546 前态继续时，人物 0 的动作 49、附着 c1 被拒绝。保留原始前态，不清理或替换其输入。

## 原行为和实现边界

复用已有 REA / Ghidra 证据 `ev_1ee104565b42b16bfedb576d51ca0d8c0d9d1deb05e7f4680df600642e3cf3fb`，读取 `$9673`、`$AA2B`、`$8B67` 的实际反编译输出。选定函数另存于 `.local/basketball/rea/hurt-rider/reused-rea.json`，没有重复导入相同二进制。

动作 49 的初入顺序为：初始化、根据 X 速度设置朝向、辅助目标转动作 11、处理支撑物、清自身附着、处理搭档或所持物、掉球或掉篮筐，最后执行动画和运动。普通球员支撑者保持原状。Kind12 支撑物已经附着时不变；未附着且在空中时改为动作 12；未附着且在地面时改为动作 4，并接收受击人物的速度和重力。

`PrepareHurtPlayer` 输出有序命令，`world_hurt.go` 在运动前按顺序提交。支撑物操作复用已经对照原 `$8B67` 的 `detachWorldJumpSupport`。`CompleteHurtPlayer` 只处理动画、运动和落地尾部，不重放准备命令。随机数在准备阶段提交；完成阶段不把准备前的共享随机数或球状态写回世界。

兼容的独立 `StepHurtPlayer` 仍拒绝缺少真实支撑物的附着入口，避免脱离世界信息调用时伪造支撑。世界入口使用完整准备／完成协议，通用附着限制仅针对已恢复的动作 49 放开。

## 验证范围

- 160 次原 `$9673` 完整调用，覆盖四名人物、初入／已进入、普通球员／Kind12 地面／Kind12 空中／已附着 Kind12／Kind14、持球／无球、辅助目标有／无。
- 对比完整准备边界和最终世界状态，并在 Go 的真实运动加载点检查准备写入已经生效。
- 旧受击语料的 805 次原调用和已有掉球速度／恢复测试保持通过。
- 夹具初版未携带可变辅助物 Kind，导致 Kind14 被默认解码为 Kind12，错误已在源解码器修复；保留 `components-after.log`，没有修改游戏规则适配缺字段。

- 160 条完整动作经公共人物分派入口验证通过；运动边界加载时，球权、支撑物、附着及随机数已匹配原准备状态。
- 原 video 11546 失败前态继续 180 视频帧，每帧 JSON 恢复与直接推进一致。
- 集成后的以上定向测试与旧 805 次受击语料，race 4.227 秒、Wasm 1.192 秒通过。
- 单配置冷启动 Native 和 Wasm 各双次完整两节比赛通过：均为 video 22412、logic 7442，每次恢复 84 次，轨迹摘要均为 `d1e32e5398207a18f31d34c9ecab2662ce56dc2fef584739028ada47e95df431`。固定原种子 `0x12345678`、原电脑策略、正常比赛时钟，源码前后摘要相同。报告为 `.local/basketball/case11-hurt-rider-fixed/{native,wasm}/report.json`，两种运行环境采用同一源码摘要 `895d42bde3ac97d5c04f2a20f0255663a083c6d151cc3271e1e4c23303a30a7c`。

可重复执行：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_hurt_rider.py
go test -race ./games/basketball/engine -run '^(TestOriginalHurtRider|TestRuntimeHurtRider|TestOriginalHurtPlayerTrace|TestHurtDropBallVelocityAndRecovery)' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^(TestOriginalHurtRider|TestRuntimeHurtRider|TestOriginalHurtPlayerTrace|TestHurtDropBallVelocityAndRecovery)' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
python3 scripts/basketball/diagnose_team_rosters.py --court 1 --teams 5 6 --characters 2 3 0 1 --output .local/basketball/case11-hurt-rider-repeat
```

结果只证明该恢复分支及该配置，不代表全部球队／球员组合、浏览器和所有完整复刻任务已经完成。没有改动悬空球缺陷或其他游戏。
