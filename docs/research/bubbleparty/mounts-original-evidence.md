# 泡泡堂坐骑来源与边界

实现前失败场景：图标与骑乘模型配错；四向骑姿错位；静态层随红蓝队色变化；图集解码越界或来源摘要不符；覆盖旧不可变素材；坐骑覆盖基础速度导致脱落后升级丢失；已骑乘、困泡或死亡者重复拾取；非法坐骑存档；同一道水柱先脱落又困泡；后续永久免疫；飞行失骑卡进障碍；机器人与客户端预测速度不一致；素材失败导致人物消失或新增数字身份角标。

来源为保留的 2002-03-13 客户端 `Fx.idd`。原始资源 SHA-256、字节区间、ObjectTable 动画与 ItemAttr 字段保存于 `games/bubbleparty/assets/manifest-mounts-v4.json` 和 `mounts-v4.json`。提取入口：

```sh
python3 -B scripts/bubbleparty/prepare-mounts.py --client-dir .local/bubbleparty-original/client-2002
python3 -B scripts/bubbleparty/check-mount-assets.py
. scripts/env.sh
go test ./games/bubbleparty/server -run TestMount -count=1
```

REA 静态证据由经典地图调查共用的 `.local/bubble-classic-maps/source-evidence/rea-native-map-tile-evidence.json` 保留。`FUN_004489c0` 从 ItemAttr 初始化 Ship，末尾第一个 int 为移动量。`FUN_00442bb0` 骑乘时改用此字段；40 原始像素一格，30 ms 逻辑 tick。普通乌龟（Ship02）为 2 像素/tick，即 600 ms/格；快乌龟（Ship03）为 9，即约 133 ms/格；猫头鹰（Ship01）为 5，即 240 ms/格。角色骑姿与坐骑的四向动画由 ObjectTable 引用，保留原始帧序列。

`FUN_004420b0` 限制角色存活、未困泡且尚未骑乘。`FUN_00443cd0` 开始 50 tick 的受击脱落过渡。当前实现立即显示脱落，并保持 1500 ms 水柱保护；这是简化过渡的实现选择，不宣称逐帧恢复原客户端。基础移速单独保存，骑乘期间的鞋子升级在脱落后生效。所有乘骑仍不能穿越硬障碍或泡泡。飞碟与故障飞碟可穿箱子，但不能拾取任何道具；ItemAttr effects 5/9 在原版 debug 字符串中明确对应 m_fAbPassBox / m_fAbNotEatItem，0x43ffa0 的拾取分支检查后者。

8 种坐骑均恢复素材与属性：飞碟为 10 px/tick；故障飞碟、普通章鱼、矿车为 2；快章鱼为 9。未发现章鱼附加水域能力，不添加臆测机制。矿车在 TileInfo.fieldsRaw[8] 非零地面上改用 fields[8]=10 px/tick；原始地面表只有 Tile08_Ground/Rail00..06（IDs 424..430）此值非零，0x449750 将该字段复制为 ground+0x70，0x442bb0 再读取并覆盖速度。具体地图掉落由父任务整合。8 种坐骑共 120 个 PNG（含 10 个角色红蓝骑姿）位于不可变 `bubbleparty/v4/mounts/`。原版素材再分发许可未核验，与此前原版素材采用同样的来源声明。

没有运行原版联网对局。以上是静态代码和原始资源证据，不能替代原客户端动态验收。

补充 REA 证据：`.local/bubble-mounts/rea-mount-evidence.json`，13 条记录。主要 evidence：`ev_5ca7652548d5b3e273ce15e4123c3cdee3c5a2612529b43177570ee5a514dce8`（能力字段debug标签）；`ev_0b6870d0625d6a6e51687b313263686a1f926cdb9e74fc4da26e628c81d50515`（飞碟禁吃道具）；`ev_69aeea8194ea1672d97536e1ed7fb45bbb94184439b018005939305f23ab1a80`（轨道地面字段映射）；`ev_8b78c5911dfc173ca479748b898f661addc4e01f0863b906bbe7cc2a75be71d6`（骑乘速度与地面覆盖）；`ev_5e15421988bbe652b2a0496f2ede16a642dc508e07d6fe79e5438f51890df113`（脱落50tick）。完整水柱受击调用链由地图证据中的 `ev_01b8bfd3376033a31fd7c07e4c37967ec108645c91deefa5c0928ab7ef01527f` 证明：覆盖达到2/3时未骑乘困泡、骑乘脱落，负脱落计时期间不重复受击。
