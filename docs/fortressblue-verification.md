# 疯狂坦克（fortressblue）验收记录

日期：2026-10-20。实现来源：用户要求重做「疯狂坦克」，使用《Fortress 2 BLUE》原版素材、地图与玩法。
素材取自官方网页客户端公开资源包，经用户明确授权仅在本私有平台使用；原始媒体不进入 Git。
解码方法、来源哈希与许可见 `games/fortressblue/assets/README.md`、`sources.json` 和 `manifest.json`。

## 范围

- 2／4／6／8 人，红蓝两队各半；最多 4v4。
- 17 张原版地图（前景层碰撞、背景层、缩略图、原版音乐）。
- 14 种原版坦克（官方大厅属性：HP、防御、速度、主炮/特殊弹攻击、最大角度、射程）。
- 位图可破坏地形、风力、角度/力度、朝向、特殊弹、坠落、突然死亡、电脑/托管、观战、恢复。
- PC 鼠标瞄准/滚轮力度/点击开火、方向键移动与瞄准、空格开火；手机滑杆和按钮。
- 与服务器一致的落点预览、2.4× 坦克绘制、1200 世界像素取景、屏幕边缘敌人指示和小地图。
- 地图选择按 stage 排序；等待房间地图/规则修改必须保存，未保存时禁止开始。
- 原版坦克、炮弹、28 帧爆炸、地图与音效通过平台 `BoardProps.audio` 播放；CDN 优先、
  同源 `/fortressblue/v2/` 回退。

## 自动化检查

```sh
go test -count=1 ./games/fortressblue/...
go test -race -count=1 -timeout=10m ./games/fortressblue/...
go vet ./games/fortressblue/...
npm --prefix web run build
npm --prefix web test -- fortressblue
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/fortressblue/fetch-assets.py
python3 -B scripts/fortressblue/verify-assets.py --dist --oss
python3 -B scripts/fortressblue/check-browser.py
```

结果：

- 服务端规则：原版目录 17 张地图/14 种坦克、4×4 碰撞格、炮弹扫掠碰撞与地形停止、
  爆炸伤害/弹坑/掩码变化、红蓝顺序、选角超时、回合推进/超时、毒/火/爆炸/雷特殊弹、
  同时全灭、时间到同生命和局、坏存档/非法弹坑/错误队伍拒绝、电脑合法动作、快照后弹坑保持。
- 模块 E2E：2／4／6／8 人全电脑对局完成；额外对 **17 张地图逐一进行 8 人全电脑对局**，
  全部自然结束（队灭或时间到判定），无死循环。
- HTTP/SQLite E2E：新游戏默认上架；8 人房 + 7 名电脑、选坦克、移动、开火、错误席位/
  epoch/turn/角度/力度拒绝、服务重启保持对局；2v2 随机地图；团队结果、历史记录。
- 前端：8 项疯狂坦克目录/瞄准/弹道预览/碰撞位图镜像/素材 URL 测试通过；全部 459 项前端测试通过；
  构建通过；24 款游戏边界检查通过。
- 素材：178 个原版 WebP/MP3 文件通过字节数、SHA-256、WebP/MP3 头、时长校验；本地、
  `web/dist` 和 `https://cdn.ichenj.com/bga/v20261002-1/fortressblue/v2/` 三处字节一致。
- 竞态：`go test -race` 通过（服务端 + 模块/HTTP E2E）。

## 真实浏览器验收

`python3 -B scripts/fortressblue/check-browser.py` 启动临时生产式服务，通过共享「创建房间」
弹窗选择「疯狂坦克」和 stage-4「钢铁工厂」，设置 8 席后创建房间；在等待房间修改地图，
验证未保存时不能开始、保存后服务器收到新地图；随后添加 7 名电脑、开始游戏、选择原版坦克，
并用键盘、鼠标瞄准、滚轮力度和点击战场完成开火。本地回退运行：
`.local/fortressblue/browser/941e868b/result.json`（50/50 通过）。CDN 运行：
`CHECK_ASSETS_BASE=https://cdn.ichenj.com/bga/v20261002-1 python3 -B scripts/fortressblue/check-browser.py`，
结果 `.local/fortressblue/browser/181e568a/result.json`（50/50 通过）。

覆盖：

- 共享「创建房间」两步弹窗真实创建房间；地图下拉按 stage 1～17 排序，选择「随机地图」时
  只显示「随机地图」和 17 张可选说明，不冒充某张固定地图；选择 stage-4 后预览、
  等待房间和开局 `data-map-id` 都是 `stage-4`。
- 等待房间修改地图为 stage-2 时显示「地图或规则尚未保存」，开始按钮禁用；保存后服务器
  options 变为 stage-2；改回 stage-4 并保存后开局实际使用 stage-4。
- 共享玩家栏 8 席；14 张原版坦克卡；原版立绘实际加载。
- 选坦克卡/标题/说明与战场 HUD/开火按钮对比度 ≥ 4.5:1；桌面操作区宽度不超出棋盘。
- 原版地图前景、背景、爆炸图集、坦克 idle 精灵由本地或 CDN 实际加载；
  Canvas 像素多样性检查证明不是空白或纯矢量占位。
- 服务器倒计时按 `serverNow + performance.now() - receivedAt` 计算，不归零。
- 键盘 ↑ 改变角度、→ 使服务器位置增加；方向键在游戏回合内被正确接管。
- 鼠标移动到坦克右上方后角度约 58°、朝向为右，落点预览有 123 个采样点；
  移到左下方后朝向变左、角度约 6°；滚轮增加力度且页面不滚动；点击战场由服务器返回 shot id。
- 动画结束后 `data-craters > 0`，证明爆炸真实擦除了原版前景地形；刷新后
  `animating=0`，不重播旧炮弹。
- 坦克按 2.4× 原画绘制，桌面取景宽度 1200 世界像素，当前坦克与最近敌人同时可见；
  屏幕边缘显示其他敌人方向，小地图标记全部存活坦克。
- 320×844：无横向溢出，Canvas 实际宽度 ≥280 px（16:10 高 178.75 px），8 席玩家栏存在；
  滚动到操作区后开火按钮完整位于视口内（高 48 px），坦克仍可辨认。
- 截图：`.local/fortressblue/browser/941e868b/desktop-select.png`、`desktop-board.png`、
  `desktop-after-shot.png`、`mobile-320.png`、`mobile-controls.png`；CDN 运行截图见
  `181e568a/`。

## 已知边界

- 队伍模型固定为红/蓝两队且人数相等；原版的 A–H 多队自由组合不在本期范围。2/4/6/8 人的
  1v1、2v2、3v3、4v4 均由平台席位自动分队。
- 原版道具拾取、商店、账号成长、语音和观战聊天不重做；保留核心炮战与特殊弹。
- 共享玩家栏在手机上占据较高区域，战场与控制面板需向下滚动；未用游戏 CSS 隐藏或重排公共界面。
- 结果浮层使用平台共享组件；棋盘保留最终局面，游戏不插入自定义横条。
- 原版媒体不提交 Git；本记录中的 CDN 对象为不可变版本，修改素材必须创建新前缀。

## 发布与部署

- 发布提交：`dd0914c6884f22c280a26305540ebebbbfb23621`，tag `v1.22.0`。
- main 检查：Actions run [39356191736](https://github.com/k0ngk0ng/wire-board/actions/runs/39356191736)
  成功，覆盖模块计划、前端、共享与 `Test and Linux amd64 binary` 构建。
- tag 发布：Actions run [39357127144](https://github.com/k0ngk0ng/wire-board/actions/runs/39357127144)
  成功，复用同一提交已验证的 Linux amd64 产物；Release：
  <https://github.com/k0ngk0ng/wire-board/releases/tag/v1.22.0>。
- 部署镜像：`ghcr.io/k0ngk0ng/wire-board@sha256:d771c87af2cb4aabb297fb1dfe84ba2c9a4c28de7ff0e6e3329a36ba09bbb6d5`
  （本地镜像 ID `sha256:a041090f…`），容器 `WIRE_BOARD_VERSION=v1.22.0`，健康状态 `healthy`。
- 生产检查：`/healthz` 返回 `{"ok":true}`；同源回退素材
  `/fortressblue/v2/maps/stage-1-fg.webp` 与 `/fortressblue/v2/tanks/1-idle.webp` 均返回 200。
- 上下架设置保持原样：`fortressblue=1`、`battlecity=1`、`bubbleparty=1`、`dota=0`；其余游戏
  设置未改动。`cdn.ichenj.com` 素材基址与 OSS 配置沿用既有 `.env`。
- 更新前在线备份：`backups/data-20261020T020910Z-2750517.tar.gz`；解包数据库
  14,512,128 字节，`PRAGMA quick_check` 为 `ok`（13 个用户、4 个房间）。保留紧邻上一份
  `backups/data-20261019T144149Z.tar.gz`，同样通过 `quick_check`；删除更早的
  `data-20261019T142744Z-1220702.tar.gz`。
- 镜像清理：`update.sh clean` 删除历史 `v1.21.1`，保留当前 `v1.22.0`、回滚 `v1.21.2`
  及 `wire-tv` 等其他服务镜像。清理后服务健康；磁盘 40G 中已用 25G（67%），备份目录 3.5M。
