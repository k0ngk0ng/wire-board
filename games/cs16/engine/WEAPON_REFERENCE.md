# 枪械来源与量化边界

对照源为 ReGameDLL_CS 固定提交 `1f16a7f25c14225931f8df1abc51f68d18ded839`，该项目是公开重实现；以下源码观察不等同原版运行对照。基础数值选择非 `REGAMEDLL_FIXES` 分支。每秒 100 规则步，开火周期使用微秒累计期限，使不整步的射速保留平均周期，实际发射最多量化至下一规则步。

| 枪械 | 伤害 | 弹匣 | 价格 | 装填规则步 | 开火周期 µs | 源码入口 |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| glock18 | 25 | 20 | 400 | 220 | 150000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_glock18.cpp#L114) |
| usp | 34 | 12 | 500 | 270 | 150000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_usp.cpp#L115) |
| p228 | 32 | 13 | 600 | 270 | 150000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_p228.cpp#L73) |
| deagle | 54 | 7 | 650 | 220 | 225000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_deagle.cpp#L74) |
| elite | 36 | 30 | 800 | 450 | 75000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_elite.cpp#L74) |
| fiveseven | 20 | 20 | 750 | 270 | 150000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_fiveseven.cpp#L73) |
| m3 | 20 | 8 | 1700 | 45 | 875000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_m3.cpp#L72) |
| xm1014 | 20 | 7 | 3000 | 30 | 250000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_xm1014.cpp#L70) |
| mp5navy | 26 | 30 | 1500 | 263 | 75000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_mp5navy.cpp#L68) |
| tmp | 20 | 30 | 1250 | 212 | 70000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_tmp.cpp#L67) |
| p90 | 21 | 50 | 2350 | 340 | 66000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_p90.cpp#L71) |
| mac10 | 29 | 30 | 1400 | 315 | 70000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_mac10.cpp#L67) |
| ump45 | 30 | 25 | 1700 | 350 | 100000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_ump45.cpp#L71) |
| ak47 | 36 | 30 | 2500 | 245 | 95500 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_ak47.cpp#L73) |
| m4a1 | 32 | 30 | 3100 | 305 | 87500 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_m4a1.cpp#L98) |
| galil | 30 | 35 | 2000 | 245 | 87500 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_galil.cpp#L71) |
| famas | 30 | 25 | 2250 | 330 | 82500 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_famas.cpp#L91) |
| aug | 32 | 30 | 3500 | 330 | 82500 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_aug.cpp#L88) |
| sg552 | 33 | 30 | 3500 | 300 | 82500 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_sg552.cpp#L87) |
| scout | 75 | 10 | 2750 | 200 | 1250000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_scout.cpp#L94) |
| awp | 115 | 10 | 4750 | 250 | 1450000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_awp.cpp#L95) |
| g3sg1 | 80 | 20 | 5000 | 350 | 250000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_g3sg1.cpp#L89) |
| sg550 | 70 | 30 | 4200 | 335 | 250000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_sg550.cpp#L96) |
| m249 | 32 | 100 | 5750 | 470 | 100000 | [PrimaryAttack](https://github.com/rehlds/ReGameDLL_CS/blob/1f16a7f25c14225931f8df1abc51f68d18ded839/regamedll/dlls/wpn_shared/wpn_m249.cpp#L70) |

装填字段中 M3/XM1014 表示逐发装入的后续间隔；起手分别为 55+45 和 55+30 步，其余枪械表示完整弹匣装填时长。价格、弹匣与备弹上限来自 `weapontype.h`；伤害、速度、距离衰减与常规装填来自 `weapons.h`。

## 动态规则来源

- 手枪实际周期包含各自 Fire 中的扣减：USP `0.225−0.075`、Glock/P228/FiveSeven `0.2−0.05`、Deagle `0.3−0.075`；Dual Elite 使用 legacy `0.2−0.125`，修复分支的 `0.122` 不混入此配置。
- 自动枪散布沿用各枪 PrimaryAttack 的站立、移动、空中分支及枪内 accuracy 递推；legacy 的整数除法与 MP5 的 double 除法分开实现。FAMAS 自动模式额外增加 0.01，三连发保留初次 spread。
- 各自动枪 KickBack 的四姿态七参数从固定源码逐项恢复，包含原版移动优先或空中优先的分支差异；后坐恢复对照 `pm_shared.cpp:PM_DropPunchAngle`。随机序列采用独立可重放整数生成器，不声明与原版 SharedRandom 逐种子相同。
- FAMAS 三连发首发伤害 34，后两发伤害 30；首发后 0.05 秒、再过 0.1 秒继续。Glock 后两发使用固定 0.05 散布、0.9 距离衰减，不重复累加首发 KickBack。
- USP 消音器操作为 legacy 3 秒，M4A1 为 2 秒；开镜、点射、消音状态随库存和掉落保存。
- 生命穿甲比例由 `player.cpp:1196` 的 `ARMOR_RATIO=0.5` 乘枪械因子得到，Glock 为 0.525，SG550 为 0.725，M249 为 0.75。
- 跌落伤害对照 `player.h:61` 与 `multiplay_gamerules.cpp:3691`：安全速度 500，致死参考 1100，乘 1.25；定点后按整数舍入。

- CS 原始站立眼高为人物中心 +17，蹲伏 +12，来源 `pm_shared.h:67`、`util.h:81`；没有沿用 Half-Life 的 +28。
- Deploy 对照公共 `DefaultDeploy` 的 0.75 秒，AWP 为 1.45 秒、Scout 为 1.25 秒；切换武器清除旧开镜。Scout/G3SG1/SG550 第二档 FOV 为 15，AWP 为 10，开镜最大移速按原枪械常量处理。

- 平台冻结准备期允许切枪、丢弃、拾取、装填和武器模式操作，仍禁止射击、三连发续射、拉雷、投掷、爆炸与伤害；回合结束期维持关闭。此门槛通过有序输入与实际规则推进验证。

## 队友伤害选项

`friendlyFire` 默认为 `false`。关闭时队友枪械、小刀和 HE 不扣血或护甲，身体仍按原命中盒阻挡子弹并消耗穿透；开启时枪械和小刀先按命中部位计算，再取整数 35% 队伤后应用护甲。HE 沿用完整现有爆炸伤害，不套用枪械的 35% 系数。敌人、自伤、世界伤害、跌落和 C4 沿用既有规则。

队杀扣 1 frag 与 3300 资金，资金最低为 0；同一生命只结算一次。敌人击杀继续使用 +1 frag 和现有 300 奖励，上限 16000。依据为固定源码 `player.cpp:963` 的爆炸分支与 `player.cpp:1187` 的 legacy 队伤分支，击杀结算见 `multiplay_gamerules.cpp:3994`、`gamerules.h:158`。

`friendly_fire_combat_e2e_test.go` 覆盖默认值、实际枪械、小刀、HE 引信、护甲、队杀幂等、资金边界、自伤／C4、低穿透身体挡弹及可穿透子弹减伤。其 5 组测试与既有 23 组战斗测试合计 28 组 race 检查通过；房间设置与电脑开火保护由其他集成测试另行验证。

## 已知对照范围

人物使用原始 8 角色 MDL 的 21 个命中盒和原骨骼矩阵，按实际武器的 DefaultDeploy 动作扩展选择 11 类站立／蹲伏持枪姿态的首帧中央 blend，再按玩家朝向变换；未持盾时排除 group8。原始人质使用其 19 个命中盒、21 骨骼与脚底原点，枪击和爆炸接入人质生命与处罚。完整动画逐帧命中盒、动态 pitch/blend、原始浮点边界及原版随机种子一致性尚未通过原版运行对照。负备弹、无界事件等历史异常不作为平台行为沿用。

命中回溯有界恢复目标位置、朝向、武器姿态与生命代次；当前墙与动态门仍按当前权威碰撞检查，不声明历史整场景回溯。材质来自原 BSP face 与 materials.txt，穿透按命中面多边形定位并应用源材质系数；距离指数衰减使用整数分段近似。

参数与行为的复跑入口为 `go test ./games/cs16/engine -run 'TestReference|TestAllTwentyFour|TestBurst|TestOriginalModel'`；必须使用 `.local/cs16/` 缓存与临时目录，结果以 `.local/cs16/verification/` 对应源码版本产物为准。
