# 两角色世界组合：实现前验收范围

在已验证的酎世界和来源 10 的四选择器／五持续动作之上，增加完整来源 10 人物组合，并保持酎旧 API 的行为。公共输入、动作前置／尾段、AI、动画提交、投影、HUD 与世界调度各只维护一份。来源 10 的原版 HUD 显示「戸愚呂（兄）」；平台显示使用「户愚吕兄」。人物来源 ID、原角色槽和平台席位继续分开。

先用真实菜单输入 `fighter10-inputs.json` 取得来源 10 人类对来源 9 电脑的完整原版对照，再写共享组合与世界新对局测试，保存缺实现／尚不支持的真实失败，最后实现。原输入只缩短第 1,350 帧左方向结束时间，不注入人物内存。自然来源 10 AI 与特殊招式覆盖按实际记录分别报告。

失败场景：共享组合仍固定来源 9；AI 使用错误角色策略或遗漏对手子槽攻击值；暂停／慢动作时仍采样或推进动作；重复动画时钟／RNG；把来源 10 的字节计数另存成第二份 `+48`，或覆盖其余 24 bits；尾段缓存一次阶段掩码导致后续选择顺序错误；共享投技偷用酎参数；完整人物调用在投影／HUD 之前提前结束；效果池、投技、HUD、记录流或动画缺失时泄露半步状态；私有 CPU 栈覆盖统计／输入终止标记；每步回灌原中间态；世界按人物重置共享状态；创建配置仍固定人物 9 或偷偷接受未实现角色；混合角色调用顺序／碰撞／判胜错误；读取参考世界伪装全新对局；图形／HUD 缓存丢失、保存恢复改变结果；原生／Wasm 规则或像素不一致；改动酎默认对局轨迹；浏览器显示了户愚吕兄但实际仍用酎规则。

目标验证分为原 CPU 完整 input／action／active actor 返回、真实自然世界逐轮和单初态连续对照、全新双角色对局与原生／Wasm 呈现矩阵、真实浏览器角色选择和输入。每层保留可复跑产物，不用一个层次的绿灯替代其他层次。

## 当前组合结果

共享动作、输入和人物驱动分别为 `StepFighterAction`、`StepFighterInput`、`StepFighterActor`；原酎 API 保留来源 9 包装。来源 10 的选择器、五个持续动作及共享尾段使用同一份动画时钟、形状提交与世界 RNG，没有复制整个驱动。`AlternateVelocity` 继续保存唯一 `+48` long，传入选择器时只建立最高 byte 的临时视图，并保留其余 24 bits。AI 可见事实补齐对手子对象的攻击值 `+65`。

来源 10 的原 CPU 完整组合说明见 [FIGHTER10_COMPOSITION_VERIFICATION.md](FIGHTER10_COMPOSITION_VERIFICATION.md)。新真实世界记录有 2,789 轮、36,871 次回调，视频标签 1,800..4,590；逐轮原始字节及单初态连续到清场均通过。只更新声明的外部控制器样本，未回灌人物、RNG、生命、形状或结果。

新输入的第一个捕获轮次已经在同一视频标签调用一次 `3a2e`，所以首条 `driver_entries=[1800]`；旧酎输入在视频标签之间跨越该入口。首条快照已经包含该前置阶段，不再补一次 begin。后续每轮仍要求恰好一个已观察的 driver 入口。原 64 KiB 内存、视频及 5,500 视频步的双内核基线完全一致。

| 全新配置 | 逻辑 ticks | 原赢家码 |
| --- | ---: | ---: |
| 户愚吕兄／户愚吕兄电脑 | 3,058 | 4 |
| 户愚吕兄／酎电脑 | 3,351 | 1 |
| 酎／户愚吕兄电脑 | 2,807 | 4 |
| 三人混合电脑 | 3,966 | 4 |
| 四人混合电脑 | 4,151 | 4 |
| 四人混合组队电脑 | 5,074 | 1 |
| 户愚吕兄真人静止输入／酎电脑 | 2,432 | 4 |

以上七组完整规则对局和其中四组完整逐帧呈现对局，在原生／Wasm 下的完整状态、最终状态与适用的像素轨迹 SHA 一致。各组实际发生伤害、KO、退场、结果与清场；中途保存恢复同样继续比较状态和事件。旧酎五组配置在两个运行时的十份报告与 `149cc02d` 结果逐字节相同。

浏览器读取实际人物 tag、输入通道和动作，确认选择户愚吕兄后执行 `Action=0x11/Pose=0x0c` 后退，而不是只更换图片。原生键盘、三指触屏／取消、后台释放和桌面／320px 截图保存在 `verification/fighter10-local-player`。

## 复跑

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_chu_world.py \
  --inputs games/yuyuhakusho/engine/testdata/fighter10-inputs.json \
  --out .local/md-yuyuhakusho/fighter10-world-replay \
  --start 1800 --end 5500 --allow-open-end --name chu-world-fighter10
sh scripts/yuyuhakusho/verify_fighter_sessions.sh both
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/check_fighter_player.py
```

完整来源与集成记录由 [fighter10-session.json](../verification/fighter10-session.json) 绑定。新增人物的自然 AI 仍没有录制，相关真实 AI 已在原 CPU 私有完整 input／actor 调用和独立全新对局验证；不将其称作自然证据。其他九个角色、七个场地、声音、平台网络与托管尚未完成。
