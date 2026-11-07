# Bot 的已呈现像素边界

BotVisualFrame 是纯视觉分类器的输入，只保留最终 256×240 RGBA、已实际公开的 HUD、席位和观察序号。它不保留 World、玩家规则坐标、敌人槽／Kind、HP、AI 计时、碰撞窗口、随机数、原始 OAM、屏外背景或未使用的调色板信息。

生产者使用 PresentedFrame，通过与真实 Canvas 相同的规范化图形规则生成最终像素。13 个普通原版帧和 7 个分屏原版帧，共 1,228,800 像素通过对照。这个结果只证明像素边界，不替代视觉分类、策略公平或全关游戏验收。

## 旧观察问题与失败矩阵

原 ObserveBot 在下一次 Session 提交前读取当前逻辑坐标、敌人槽与 CollisionWindow，会早于 Canvas。Sprite=1 又是没有图块的 blank 描述，既用于新生占位，也用于由背景绘制的门／核心，不能简单作为可见条件或统一删除。

玩家在敌人接触阶段被平台修正 Y，也可能晚于旧 Combat.Display 的冻结点。原版玩家 Y 与显示对象 Y 共用 $031A；主任务已用真实记录修正显示别名。Bot 不能通过读取更早／更晚的物理坐标掩盖呈现错误。

先行失败矩阵包括：

- 新生占位、未提交绘图、队列拒绝或前缀丢弃。
- 背景目标已显示但规则槽未激活；逻辑已销毁但旧画面仍显示。
- 闪烁空帧、每行 8 精灵上限、遮挡、背景优先级、裁切和同色图像。
- 屏外地图、未提交镜头、碰撞窗口早于可见地形。
- 隐藏槽／Kind／HP／计时／RNG 改变但画面相同时，观察必须一致。
- 调用者修改素材、HUD 或返回缓冲；恢复或重复帧改变可见事实。
- Unknown HUD 的残留数值被当作事实。

## API 与接线

    frame, err := NewBotVisualFrame(
        session.World.Active.Presentation.Snapshot(),
        publicHUD, visualCatalog, actor, observationID,
    )
    rgba := frame.Pixels()
    otherSeat, err := frame.ForActor(otherActor)

Pixels 返回调用方拥有的副本。ForActor 复用冻结像素，不重新渲染或加载素材。Snapshot 是规则层唯一允许的呈现入口；不要混入待提交的 OAM、逻辑位置或碰撞补丁。无需新增另一套 World。

BotPublicHUD 包含座位数、公开 phase，以及带 Known 的 stage／scene／choice、paused 和各席位生命／武器／存活状态。未实际显示的字段必须为 Unknown；构造器会清除 Unknown 的残留数值，不从规则补齐。

HUD 必须与人类界面使用同一份已呈现或明确公开的数据。网络 JSON 中能读到字段，不代表用户已经看见它。不能仅为 Bot 把隐藏字段重新定义为公开 HUD。

分类器只接收最终像素和固定视觉知识。目标类别、坐标、姿态、地形与时序关联由画面识别得到；在固定知识下，相同像素／HUD／席位／序号必须得到相同观察。

## 不可变素材目录

    id, err := BotVisualCatalogIdentity(manifest)
    catalog, err := NewVerifiedBotVisualCatalog(manifest, rgb, loadPage)

清单只含 RGB 与 CHR 页的长度／SHA-256，不绑定部署路径。RGB 为 192 字节；CHR 页为 4096 字节的 64×64 索引图，取值 0..3。目录校验摘要、复制输入、缓存独立数组并返回副本。一次画面只读取当前 8 槽的去重页，不遍历 128 页。

目录不访问文件、网络或 ROM。服务端若禁止回调内 IO，可在构造阶段按 runtime-stages.json 的实际可达并集预读、校验并冻结内存，再注入目录。浏览器仍只加载当前关资源。

开发素材位于 .local/contra-browser/build/public/graphics/，包括 colors.rgb 和 page-%03d.idx，摘要来自该构建的 runtime manifest。正式路径由发布构建绑定，不将研究目录用作线上回退。BuiltinLevelBundle 只有规则／关卡数据，不能代替图形目录。

## 静态模板

BuildBotVisualSpriteTemplates 只处理调用方提供的固定、有限 recipes。每项包含 Mapping、Palette、Code、Attributes 和公开视觉 Label，不读取运行中的 World。

构造复用已验证的 sprite 描述和像素生产者，以两种不同底色识别透明区域，裁剪后保留原图形锚点。code 0、blank code 1 和无可辨像素不产出目标。不遍历全部 code × palette × preset，也不按内部 Kind 自动分类。36 种姿态／翻转／后坐组合与像素生产者完整对照通过。缓存按实际有序 OAM 覆盖和解码后的 RGB 建键，不把页号、tile 号、sprite code 或 Label 当作画面差异；同实图的静态别名保留各自标签和独立输出缓冲。

标签、重复图案消歧、背景实体／地形模板和时序追踪由 BotVision 维护，须独立绑定身份并验证，不能用运行时槽补齐识别。

## 验证与成本

    source scripts/env.sh
    go test -race ./games/contra/engine -run '^(TestBotVisualFrame|TestVerifiedBotVisualCatalog|TestBotVisualSpriteTemplates)' -count=1

原版夹具只用于生产者核验，不进入游戏逻辑。M4 初步双席基准约 0.322 ms／帧、770 KB／帧分配，其中包含两个 Pixels 副本。该数字不是服务器容量验收。后续可共享识别结果或增加只读采样接口，不能退回隐藏槽换取性能。

最终像素、目录和机械模板的 10 项 race 检查及源码绑定见 [验证记录](evidence/bot-visual-verification.json)。对应 Go 临时目录已清理；只保留原版夹具、来源、red／green 和必要生成器，没有新增工作区外文件。
