# 泡泡堂（bubbleparty）

围桌内的实时水泡对战还原。游戏名称为「泡泡堂」，模块 ID 继续使用 `bubbleparty`，已有房间、战绩和管理设置不迁移。

## 玩法

- 2／4／6／8 人分红蓝两队。方向键或 WASD 连续移动，空格或 J 放水泡；手机使用可拖动方向盘和独立放泡按钮。
- 水泡放下后可以离开，离开后不能穿回；水柱沿十字传播，破坏砖块／木箱并触发连爆，持续期间仍会命中后来进入的角色。
- 水柱命中先困泡；队友触碰救援，敌人触碰击破，困泡到期仍未获救则淘汰。救援后有短暂保护。
- 道具提高速度、水泡容量和射程；护盾抵挡一次命中。基础策略与所有行动均由服务端规则约束。
- 新局不缩圈。一队全灭时另一队获胜；时限结束双方仍有队员则为平台显式和局，真人和局积分不变，含电脑对局沿用平台不计分规则。
- 统一支持电脑玩家、托管／取消托管、观战、刷新、断线重连和存档恢复。正常短暂停手不会立即进入托管。

当前水泡引信 2.2 秒、困泡 5 秒、救援保护 1 秒是本项目参数；尚未取得原版同版本运行证据，不能把它们称为原版精确数值。针、踢泡、投泡、坐骑及角色差异化属性不在本次基础团队对战范围内。

## 地图与资源

默认「小区10」按官方历史截图重建 15×13 住宅区、中央道路、树篱、双色砖和木箱；出生安全调整和未确认的原版碰撞语义见 [契约](CONTRACT.md)。水乐园、森林公园、沙丘营地保留为自定义地图，可选择随机地图。

角色、水泡、水柱图集来自经过本地运行和 REA 静态分析的早期 HTML5 还原项目；音乐和音效来自 REA 解包后的 2002 年客户端索引数据，未重新合成。文件使用独立 `bubbleparty/v2/` 路径，同源回退可独立运行；封面是原创手绘 SVG。来源、SHA-256、原包资源偏移及许可状态见 [资源说明](assets/README.md) 和 [原版参考核查](../../docs/research/bubbleparty/reference-findings.md)。本项目与官方运营服务无关联。

## 兼容

新局使用 `rulesVersion=2`。无版本字段的旧存档保留原有缩圈、人数判定及时间戳；刷新或接管不会升级一局正在进行的规则。内部地图存档继续兼容字节数组，公开视图使用数字数组，避免前端收到 Base64 而不画障碍。

## 验证

仅运行本游戏的规则、HTTP、电脑完整对局和前端检查：

```sh
. scripts/env.sh
go test -count=1 ./games/bubbleparty/...
go test -race -count=1 ./games/bubbleparty/...
npm --prefix web test -- bubbleparty
python3 -B scripts/bubbleparty/check-assets.py
npm --prefix web run build
python3 -B scripts/bubbleparty/check-refactor.py
```

浏览器验收每次创建独立 `.local/bubbleparty/refactor-browser-*` 目录，保存截图、真实输入及公开状态证据和 `result.json`，结束后清理本次数据库、profile、socket 和临时文件。旧 `check-browser.py` 入口转发到同一检查，不再以输入序号增加代表移动成功。

实现前失败场景分别在 `tests/REFACTOR_SCENARIOS.md`、`tests/BROWSER_REFACTOR_SCENARIOS.md` 和 `tests/SPRITE_SCENARIOS.md`。旧版验收记录只表示历史执行，不能替代本轮重构验收。

本次结果、截图路径与清理范围见 [重构验收记录](../../docs/bubbleparty-refactor-verification.md)。
