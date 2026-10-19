# 强手棋

经典 Monopoly 独立模块，规则、电脑策略、界面、素材清单及验收脚本均位于本目录。后端只依赖 `internal/gameapi`，通过 `games/registry.go` 注册；前端自动发现 `web/module.tsx`。

**默认上架。** 注册后玩家大厅和创建列表直接显示；管理员可在「管理后台 → 游戏上架 → 强手棋」下架，下架后服务端拒绝创建。重启保留管理后台的选择。

## 玩法与界面

- 2～6 人，随机先手，服务器维护骰子与私有牌堆。
- 40 格英国版地图；掷骰、双骰、起点、税款、监狱及三种出狱方式。
- 买地、自动收租、公开拍卖；报价保留资金，截止时间独立于玩家托管。
- 均衡建设及拆除、32 座房屋和 12 座旅馆库存、建筑短缺拍卖。
- 抵押、赎回、欠款筹资、向玩家或银行破产、遗产移交及银行地产拍卖。
- 私有双边交易：现金、地产、免狱卡；报价修改、撤销、拒绝、到期和接受时重核资产。
- 16 张机会卡和 16 张 2021 年后公共基金卡；卡牌连锁移动、逐人收付款和破产续接。
- 电脑玩家与托管、只读观战、刷新与重启恢复、出局离桌、结果及再来一局。
- 使用统一创建弹窗、等待房间、玩家栏和回合提示；提供桌面与手机棋盘、缩放、地产证、资产管理、交易和卡牌面板。
- 掷骰与逐格移动使用服务器公开动作记录，处理跨起点、反向移动和直接入狱；重载、漏帧恢复和减少动态效果模式直接对齐当前状态。棋子为桌面 34px／手机 26px，独立编号层保证同格玩家编号不被遮挡。操作资格由服务端视图提供；无竞价能力者不会被要求回应。

明确采用的线上约定：自动收租；单笔抵押利息向上取整（抵押值 175 对应利息 18、赎回价 193）；银行不足 4 座房屋时可按半价整组出售建筑；卡牌逐人收付款从抽牌者下一席开始；仅剩一人时先转移破产者资产，再直接结算，不继续要求赢家处理受让费用或拍卖。免费停车不发奖金，不提供玩家贷款或免租协议。

## 图片与声音

素材使用既有 `ASSETS_BASE_URL` 下的 `monopoly/v1/`，共 20 张 WebP 和 11 个 MP3。已上传至阿里云 OSS，并按实际 CDN 地址逐个回读、核对 SHA-256、Content-Type 及解码结果。

大厅、创建选项和等待房间统一使用本模块绘制的 SVG 插画封面。棋盘标志、房屋、旅馆和格子图标从固定版本的 Hasbro C1009 官方说明书提取；旧封面文件保留在版本化素材目录供旧客户端使用。六种棋子使用单独标注来源的经典风格插画。机会卡与公共基金卡使用类别图标配实时中文文本，并非 32 张完整原版卡面扫描。音效包括掷骰、购买／建设、收付资金、抽牌、入狱、通知、破产和胜利；本游戏只维护事件到音效的映射，播放与队列统一交给平台。遵循全局静音设置，首次载入不重播历史事件，媒体失败不影响游戏操作。

[来源清单](assets/sources.json) 记录来源和核验范围，[媒体清单](assets/media-manifest.json) 记录尺寸、时长和成品摘要。地产数值来自固定版本的经典地产表，已与官方说明书展示的浅蓝色地产证、车站、公用事业及税款交叉核对；未将未展示的地产证宣称为逐张官方核验。规则与研究过程见 [接入分析](../../docs/research/monopoly-classic.md)。

## 复现验收

在仓库根目录执行；缓存、临时数据库、浏览器配置、日志和截图都写入仓库内。

```sh
. scripts/env.sh
mkdir -p .local/monopoly/acceptance/final
go test -json -count=1 -timeout=3m ./games/monopoly/tests \
  > .local/monopoly/acceptance/final/server.jsonl
go test -json -count=1 -timeout=2m \
  -run 'Test(NewModuleDefaultsToPublished|ProductionMonopolyDefaultsPublished|ModuleSystemEvent|ModuleLastHumanDeparture|AssetURLValidationAndCSP)' \
  ./internal/server > .local/monopoly/acceptance/final/host.jsonl
python3 -B scripts/check_game_boundaries.py
WIRE_BOARD_MONOPOLY_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 \
  python3 -B games/monopoly/tests/accept_ui.py
```

浏览器验收使用 `agent-browser` 和实际生产前端，经过后台上架、创建牌桌、买地、建设、交易、拍卖、抵押筹资、破产、观战、手机布局、结果和刷新。媒体检查通过实际 HTML 音频播放事件验证播放、静音和失败恢复；浏览器本身静音，不把自动化结果称作人工试听。最后一局从标准开局使用服务器正常电脑定时节奏自然结束。`--skip-full` 仅跳过最后的长对局，适合界面局部回归；完整结果位于 `.local/monopoly/acceptance/browser/result.json`。本轮通过 21 个浏览器检查点，最后一局用时 436.7 秒；六款旧游戏及模块接入回归另通过 16 个检查点。

HTTP E2E 明确区分自然开局与中局夹具。2／4／6 人完整局从每人 1500、全部地产无主、完整牌堆开始，覆盖建设、交易和破产，并在中途和终局重启。初始私有测试快照、动作轨迹和统计保存在 `.local/monopoly/acceptance/full-games/players-{2,4,6}.json`；生产客户端无法指定随机源或测试场景。

## 复现素材

使用 Python 环境中的 Pillow、NumPy、PyMuPDF，以及 `ffmpeg`／`ffprobe`。按清单中的固定 URL 下载源文件并保留原文件名；脚本会在生成前核对源文件摘要。

```sh
.local/monopoly/venv/bin/python -B games/monopoly/scripts/prepare_assets.py \
  --sources .local/monopoly/sources \
  --rules .local/monopoly-research/c1009.pdf \
  --output .local/monopoly/media
.local/monopoly/venv/bin/python -B games/monopoly/scripts/verify_assets.py \
  --bundle .local/monopoly/media \
  --base https://cdn.ichenj.com/bga/v20261002-1 \
  --output .local/monopoly/acceptance/assets/cdn
```

上传工具为 `scripts/upload_assets.py`，音频需显式传入 `--include-audio`。不要修改公共素材基地址；后续修改成品内容应提升素材版本，避免覆盖不可变缓存。凭据只从忽略的 `config/secrets/` 读取，不进入素材包或前端。
