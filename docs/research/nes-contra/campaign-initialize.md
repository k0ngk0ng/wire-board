`CC5F` 关卡介绍初始化和 `CC7D` 战斗准备已恢复为独立规则阶段。实现位于 `games/contra/engine/campaign_initialize.go`，复用现有 `ActiveWorld`、`Campaign`、`LevelResources` 和 `CampaignSceneResult`。两个阶段均不推进 `LevelRoutine`，由 `Campaign.AfterPresentation` 完成边界推进。

原版来源为 `Contra (J).nes`，SHA-256 为 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。用户 ZIP 只读，分析切片和模拟器配置均位于仓库内部。

| 原版入口 | 已恢复行为 | 关键保留项 |
| --- | --- | --- |
| `CC5F` | 设置显示遮罩 `1E`；重置音频；选择介绍图形页 10；即时清空背景；排入文字模板 6 | 玩家、敌人、积分、生命和世界状态；图形页只更新 Desired |
| `CC7D` / `C328` | 清空 `0300..05FF` 对应的角色显示、弹池和敌人槽位 | 玩家零页状态、坐标小数、高字节、武器、生命、碰撞区和未加载的地图半区 |
| `A816` | 加载当前关卡的场景、碰撞、调色板、滚动限制与动画初值；重置雪粒子表 | 雪粒子与雪显示控制分别使用既有 owner；不建立第二份状态 |
| `DAD3` | 横向关从列 0 开始加载 48 列；基地加载 32 列；瀑布从第 29 行开始 | 瀑布不覆盖无关的横向计数器和属性地址 |
| `CC7D` 后段 | 加载 32 色；设置该关滚动偏移；非 demo 时播放关卡音乐；切换矩形图形队列 | 基地此前已执行一次 16 色加载，淡入计数实际递减两次 |

完整 REA Evidence 保存于 [campaign-initialize-rea.json.gz](evidence/campaign-initialize-rea.json.gz)，共 16 份函数记录。`CC5F` 的 Evidence ID 为 `ev_a96806739c2d251a16ce2d2a1b0602af3ac32b38ddeb7f4aacfb5939637c62b1`；`CC7D` 为 `ev_6637e7af2582413c0aacb969b57b1f9048848467d9c9144bfa1d529ab8a45abf`。Ghidra 部分函数边界包含相邻落空分支，因此同时保留 `C314`、`C912`、`DAFA` 的独立分析，以及实际执行的入口／返回快照；未把静态反编译当作运行结果。

原版采集共 20 场：8 关 × 单人／双人，以及 4 组非零保留状态探针。所有场景从开机开始，使用真实 Select／Start 输入。第 2～8 关在首次 `CC5F` 入口仅修改关卡编号；单人组在 `CC7D` 入口预置脏对象页；保留状态探针额外预置生命、武器、坐标小数、地图区和游标，具体写入逐项保留在场景脚本中。两组 demo 探针验证音乐抑制。没有写入函数返回状态，也没有将这些受控入口声称为自然通关。

每场保留初始化、瞬态清理、关卡头加载、地图初始化、战斗准备的完整入口／返回，共 100 条记录，包含 2 KB RAM、4 KB 名称表与音频调用顺序。测试比较其中 40 个完整初始化阶段的全部已建模 owner、图形队列、即时背景和声音。4 场独立重录覆盖横向关、基地、瀑布和 demo，全部记录逐字节一致。

先列失败场景并写测试，再写实现。首次 red 中包含缺失的两个初始化函数，以及另一并行场景正在编写的 API；没有移动或屏蔽红测试。最终限定 race 检查共 60 个测试条目通过，包括 40 个原版对照子例、18 个失败原子性子例及 2 个父测试。测试期间完整 engine 源码和资源摘要保持一致；结果、原版素材摘要及重复采集摘要见 [campaign-initialize-verification.json](evidence/campaign-initialize-verification.json)。

复验命令：

```sh
source scripts/env.sh
go test -race -json ./games/contra/engine -run '^TestCampaignInitialize' -count=1
```

需要重做原版记录时，使用已有 Pillow 环境运行 `scripts/contra/campaign_initialize_capture.py`，为每次录制指定新的 `--name`；`--verify-against` 指定已有原版证据文件名即可逐条比对。REA 切片和种子由 `campaign_initialize_prepare.py` 生成，然后分别运行 `node scripts/contra/static_rea.mjs .local/contra-campaign-initialize/{fixed,header,helpers}-spec.json`。独立验收脚本为 `campaign_initialize_accept.py`。

World 集成须消费 `display-mask=1E` 等显示效果，并保证阶段音效只入队一次。`AfterPresentation` 仍保留已有的音频边界元数据，不能再次把同一 reset／音乐追加为第二条播放命令。当前证据限定于两个初始化阶段；从创建房间到完整战役的连续运行、网络同步和合作积分验收由上层集成完成。
