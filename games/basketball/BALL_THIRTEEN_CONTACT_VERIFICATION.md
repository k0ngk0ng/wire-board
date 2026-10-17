# 特殊球 13 的接触游标

球队 4 原 seed 完整赛在 video 13067、logic 4340 首次暴露球 13 游标 2 的接触范围缺失。旧几何导出按相邻数组边界确定长度，只保留游标 0／1；原函数 `$8318` 按动画游标继续读取后续几何索引。

先写测试复现两个失败：原几何读取返回错误，以及保留的真实运行状态在下一帧报同一错误。然后仅为球 13 补上实际可达范围：

| 游标 | 原 shape 地址 | 几何 |
| --- | --- | --- |
| 0／1 | `$9151` | 原球框，高度 0～16 |
| 2／4 | `$9131` | 攻击与受击框均关闭 |
| 5／6 | `$9159` | 原球框，高度 1～15 |

不把原游标改为 0，不扩大其他动作范围；游标 3、7、255 继续拒绝。修改位于 `OriginalContactProfile` 的独立球分支，不改变人物攻击尾段或 variant 63。

`verify_ball13_contact.py` 执行未修改的原 bank 5 指令，保留 12 次调用（6 个游标 × 首入／已进入）、原 ROM 摘要和真实失败状态。定向 race 4.226 秒、Wasm 1.105 秒通过；真实失败状态续跑 360 视频帧，每帧保存恢复一致。原特殊球接触和人物攻击尾几何回归同时通过。

球队 4 正常两节、固定 seed `0x12345678` 的完整比赛运行两次，均为 22,373 视频帧、7,429 逻辑更新、84 次恢复；轨迹 SHA-256 均为 `1854bfd61c881ba3e752c3f0d05fd1ad42d686fddcadc713d6981fb43bb7c727`，运行前后源码摘要一致，`accepted:true`。此项不代表所有角色／球队／场地验收。

证据目录：`.local/basketball/rea/ball13-contact/` 和 `.local/basketball/team4-after-ball13-contact/native/`。原始失败目录 `.local/basketball/team4-after-handler17/native/` 保留。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ball13_contact.py
source scripts/env.sh
go test -race ./games/basketball/engine -run '^TestOriginal(Ball13Contact|AttackContactTailGeometry|SpecialBallContact)' -count=1
python3 scripts/basketball/diagnose_team_rosters.py --court 1 --teams 0 4 --characters 0 1 0 1 --output .local/basketball/team4-contact-repeat
```
