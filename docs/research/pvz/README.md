# 植物大战僵尸 PC 初代取证

本次以 PC 1.0.0.1051 作为原版基准，下载样本后仅进行静态读取、REA/Ghidra 反编译和资源解码，未执行 Windows 程序。玩家控制僵尸、1v1／2v2、独立钱包、限时与攻守换边属于新增对战规则。

完整接入阶段已读取原始 53 项植物表、33 项僵尸表、143 项重动画表，并补充全部特殊机制、弹道、种植层叠、碰撞和场景函数的原版证据。运行资源使用 `classic-v2`：完整原图层、动作、受损阶段和 6 个场景，另有全部原版 OGG 音效与场景 FLAC 音乐。早期 `classic-v1` 保留为历史产物，当前完整接入使用 v2。

## 完整目录与参数账本

| 产物 | 范围 |
| --- | --- |
| [full-catalog.json](evidence/full-catalog.json) | 53 植物／33 僵尸原始 ID、费用、冷却、动作类型和动画映射；常规种子为前 49 项 |
| [reanimation-catalog.json](evidence/reanimation-catalog.json) | 原程序实际 143 项动画映射，避免历史参考源码数组次序不同导致错绑 |
| [plant-mechanism-profile.json](evidence/full/plant-mechanism-profile.json) | 全植物机制参数、分支、证据和明确未知项 |
| [zombie-health-profile.json](evidence/full/zombie-health-profile.json) | 全 33 类身体／头盔／护盾／飞行生命和条件修正 |
| [speed-profile.json](evidence/full/speed-profile.json) | 逐阶段 `_ground`／直接速度分类、原始轨道、减速差异和小丑引信公式 |
| [animation-durations.json](evidence/full/animation-durations.json) | 全单位源动作帧范围；声明 FPS 与运行时覆盖速率分开 |
| [index.json](evidence/index.json) | 完整保留的原版 REA Evidence 索引 |

完整植物表原始内存证据为 `ev_b794f08ae804c64ed0c1b9adbb176f350a31bba38f7a3d267d2474b25446a9c8`，僵尸表为 `ev_5d906ea59064c18eb9742862bae44019532615fe2e027260fc8585ed0adb398d`，动画表为 `ev_71accd825ab69bd5414e44fb65d9c276298c7faca105700bc570fa3d2b91c1c8`。各机制函数的完整输出保留在 `evidence/full/mechanisms/`。

`BossBungeeAttack` 的机器码存在于 `0x5350C0`，默认分析未建立函数。后续使用[明确的分析入口种子](boss-seeds.tsv)恢复，证据为 `ev_764e4d538a1aabc89a99f9691406f3bc13a31266002cc3d451ea0d5397367833`；该分析配置包含人工指定入口，不伪装成原始符号恢复。

## 原画完整性与音乐

早期截图中「射手只有头」的根因已通过原始轨道确认：`anim_shooting` 仅含头／嘴，身体在独立 `anim_idle` 循环中。v2 按原始 stem attachment 的当前矩阵与基准矩阵逆合成完整头身。先行失败测试、75 个真实攻击帧的茎叶语义验收和[前后近图](evidence/shooter-body-before-after.png)均保留；机枪、三线和裂荚的多头也经过同类检查。

原画验证同时检查受损坚果／高坚果／大蒜／南瓜、雪橇整队前后车壳与四乘员、泳池水面、屋顶坡度，以及所有 native 图层引用。见[完整姿态近图](evidence/native-complete-poses.png)、[native 验收](evidence/native-validation.json)和[全部音效验收](evidence/audio-validation-v2.json)。这验证实际素材与呈现数据，不替代平台真人交互 E2E。

菜单、选卡、草地、夜晚、泳池、浓雾、屋顶和 Boss 的原始音乐已恢复为场景 FLAC，附带精确前奏／循环采样帧、原分轨及夜晚 76／77 高潮入口。原曲分段、官方工作区解码工具、无损 PCM 校验和范围说明见[音乐取证](MUSIC.md)。

## 样本身份

| 样本 | 来源 | 大小 | SHA-256 |
| --- | --- | --- | --- |
| 英文 2009 试玩安装包 | [Internet Archive: PlantsVsZombies_EN](https://archive.org/details/PlantsVsZombies_EN) | 27,951,128 | `2f1249af3848a6bc02d75039e285fb0fa7ca0a53d7e5c6bdb81e38b31d5864d2` |
| 已解包 PC 原始版 ZIP | [Internet Archive: plants-vs-zombies-v-1.0.0.1051](https://archive.org/details/plants-vs-zombies-v-1.0.0.1051) | 24,684,894 | `520599455b5b112b658dbf2bc5f17c939d411442ad0d0b741a7ad01757e1397d` |
| ZIP 内 `PlantsVsZombies.exe` | 上述 ZIP | 3,002,368 | `8e8eea73a85d7af311755708f5daf2786650cf3c032188b8315fcf99858c8342` |
| ZIP 内 `main.pak` | 上述 ZIP | 25,020,446 | `2fc4e9e8e1ecf40f7c53c03df5cc5a3d553dbabc63a1a833c4656975ccb8a933` |

试玩安装包为 InstallShield 自解压文件，未执行。为直接读取资源和游戏本体，后续使用已解包 ZIP。归档作者对原始性的描述属于来源元数据；本报告将结论绑定上述实际字节摘要。EXE 的 VERSIONINFO 内 FileVersion 和 ProductVersion 均为 `1.0.0.1051`。

完整下载 URL、文件摘要与参考资料摘要见 [sources.json](evidence/sources.json)。`Patoke/re-plants-vs-zombies` 仅用于函数名和候选地址定位，参数均优先核对原版字节与 REA 结果。例如该参考源码把普通僵尸随机速度上界写成 `0.32`，实际样本反编译参数为 `0.37`，本报告保留此差异。

## REA 证据

使用 REA 6.5.0、Ghidra 12.1.4。样本为 PE32/x86，image base `0x400000`；自动分析识别 10,351 个函数和 4,082 个字符串。引用的函数名是按历史参考命名，REA 实际返回 `FUN_<地址>`，未恢复原始完整符号或类型。

| 问题 | 地址／文件 | Evidence ID | 可确认范围 |
| --- | --- | --- | --- |
| 二进制身份与布局 | [binary-overview.json](evidence/binary-overview.json) | `ev_2e0a3f547e45fcd88aca41efc878fea1a57fdfd7b48debcbc4fc79534f4fa770` | 当前样本、段布局、静态分析库存 |
| 植物定义表 | `0x69f2b0`，[原始内存](evidence/plant-definitions-memory.json) | `ev_dc71287f4a873a20c822a0bf6b9c3dea5337f41f1c93ef3b1cbdb775f3f87f9e` | 费用、冷却、发射计数，36 字节一项 |
| 植物初始化 | `0x45db60`，[伪代码](evidence/plant-initialize.json) | `ev_31ed426848787e31f620f5d629adccee34f78bb4f97dcb85ff6a63e0f391c6fa` | HP、雷准备计数、樱桃特殊计数、首次产出随机化 |
| 僵尸初始化 | `0x522580`，[伪代码](evidence/zombie-initialize.json) | `ev_93206148d78e026350ef3767eda11ca9172e4110ce39d40beb6483fc75790290` | 身体、头盔、报纸 HP 与初始状态 |
| 僵尸速度选取 | `0x524a70`，[伪代码](evidence/zombie-speed.json) | `ev_80aee52ec21666389b659ab2e1d5f96ff9480215747cbacd3a867dfd6b8d0460` | 随机速度参数，不直接等于每帧位移 |
| 啃咬 | `0x52fb40`，[伪代码](evidence/zombie-eating.json) | `ev_7aa5fbacf6f4dc210598a2609ea6fd42c928983c945e5e7c1df8eff9d4c9cbed` | 单次扣除 4 HP、特殊植物免疫和减速检查 |
| 减速计数 | `0x530950`，[伪代码](evidence/zombie-chill.json) | `ev_3eb600a810458ee89c61731cac85d0da91e2bf7b3ef961270f2503a90d9faf80` | 普通减速 1,000、冰冻陷阱 2,000 更新；取较长剩余时间 |
| 头盔受击 | `0x530fe0`，[伪代码](evidence/zombie-armor.json) | `ev_9561f276ed302d8ad18a0bbef43df4cc0e6288843b6bb7e139fdec51dd6f508c` | 先消耗头盔，返还溢出伤害 |
| 生产 | `0x45f9a0`，[伪代码](evidence/plant-production.json) | `ev_f733a4c3bea3d603db4bb3988ee4787f2dfc479d1966f667f416a152fedcd96e` | 每次产出后重设随机计数 |
| 射击 | `0x45f8a0`，[伪代码](evidence/plant-shooter.json) | `ev_f057d16c1ec19bbb42079f3876d7233f49f6d722dfdd361920387c7622d53512` | 发射计数递减、双发在计数 25 再触发 |
| 动画时基 | `0x471bc0`，[伪代码](evidence/animation-update.json) | `ev_e306e79a73dbad1b3237a9178806450ce9e7cbfc7a4abc30c7a65120ef1cf498` | `animTime += animRate * 0.01 / frameCount` |
| 啃咬调度 | `0x52f4c0`，[伪代码](evidence/zombie-bite-schedule.json) | `ev_0441681cbe4ba46281519e66142c6ef77aef91ac9456bf0f6b70062095c1263e` | 普通每 4 更新、减速时每 8 更新；每次 4 HP |
| 步态归一 | `0x52f050`，[伪代码](evidence/zombie-speed-normalization.json) | `ev_adeef476620fac389e0436b866098910458d49884ae2c542913d55fdd52d3c04` | `animRate = frameCount / groundDistance * velX * 47 / scale` |
| 地面轨道位移 | `0x4738d0`，[伪代码](evidence/animation-ground-velocity.json) | `ev_690c994f06e4605a8a97c848b549fd651dd1e23ce779737f08f8ceafb2353ba0` | 每更新读取相邻地面轨道差乘以 `0.01 * animRate` |
| 弹丸伤害表 | `0x69f1c0`，[原始内存](evidence/projectile-definitions.json) | `ev_640ad3a6b16fa442a1ac499473e5640c814e05cb8d7949bba9950824a7911ae2` | 普通豌豆、冰豌豆均为 20，12 字节一项 |

反编译对自定义 x86 调用约定的参数恢复有限。随机范围另保留[初始化反汇编](evidence/plant-initialize.asm.txt)与[生产反汇编](evidence/plant-production.asm.txt)：首次向日葵计数为 `300 + Rand(951)`；后续为 `2350 + Rand(151)`。据 0.01 秒更新时基分别为 3～12.5 秒与 23.5～25 秒。射击间隔为 `150 - Rand(15)` 更新，即 1.36～1.50 秒；双发的相邻两次发射间隔为 25 更新。

## 已核对的首批单位

| 植物 | 阳光 | 冷却更新数 | HP | 发射／生产基准更新数 |
| --- | --- | --- | --- | --- |
| 豌豆射手 | 100 | 750 | 300 | 150 |
| 向日葵 | 50 | 750 | 300 | 2,500 |
| 樱桃炸弹 | 150 | 5,000 | 300 | 引爆计数 100 |
| 坚果墙 | 50 | 3,000 | 4,000 | — |
| 土豆雷 | 25 | 3,000 | 300 | 准备计数 1,500 |
| 寒冰射手 | 175 | 750 | 300 | 150 |
| 双发射手 | 200 | 750 | 300 | 150 |
| 高坚果 | 125 | 3,000 | 8,000 | — |

普通僵尸身体 HP 为 270；路障头盔额外 370；铁桶头盔额外 1,100；橄榄头盔额外 1,400；撑杆身体为 500；报纸护盾额外 150。身体、头盔和护盾应分别保存，不能因显示「总血量」而丢失护甲脱落及报纸狂暴阶段。

普通僵尸速度参数 `velX` 为 0.23～0.37，橄榄／撑杆跳前为 0.66～0.68，报纸狂暴为 0.89～0.91；实际位置由 `_ground` 轨道驱动。根据归一式推导，周期均速约为 `velX × 47` 像素／秒，对应普通约 11～17、橄榄约 31、狂暴报纸约 42。该均速是静态公式推导，不表示已逐帧运行原版测得；用固定速度或平均速度实现时应标为步态简化。普通啃咬为 100 HP／秒，减速时为 50 HP／秒。

## 资源与重复取证

PAK 共 2,413 个成员，目录、长度、源位置和逐文件 SHA-256 见 [pak-members.json](evidence/pak-members.json)。提取器的先行失败场景见 [EXTRACTION_FAILURE_SCENARIOS.md](EXTRACTION_FAILURE_SCENARIOS.md)。美术已由原始图层和轨道生成 43 段动画、1,239 帧，见[合成预览](evidence/sprite-contact.png)与[资源验收](evidence/asset-validation.json)。生成方法见[资源说明](../../../games/pvz/assets/README.md)。

REA 命令须将 `TMPDIR` 设到本 worktree 根目录；Ghidra 不接受点前缀路径段，且 macOS Unix socket 路径长度受限。REA 自动把 Java home、缓存与数据库放进该工作区内的私有临时根，关闭后移除。示例：

```sh
TMPDIR="$PWD" GHIDRA_INSTALL_DIR="<已安装的 Ghidra 目录>" \
  JAVA_HOME="<已安装的 JDK 目录>" \
  rea decompile "$PWD/.local/pvz-research/original-1.0.0.1051/PlantsVsZombies.exe" \
  0x45db60 --provider ghidra --format json
```

本轮静态证据不证明原版 Windows 运行中的每帧结果、原始随机序列、动画插值或动态音频混音完全一致。上述 8 个单位表格是早期 v1 基线；完整阶段已覆盖所有目录与场景，实际恢复程度及剩余缺口以参数账本为准。对战经济、僵尸价格／冷却、双方初始资源、时间限制、人数归一化与电脑策略由新模块维护，不能标为原版数值。
