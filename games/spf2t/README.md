# 街霸方块

2 人实时落宝石对战：同色宝石不会自动消除，需要用同色碎裂宝石接触触发；强化宝石、钻石、反击宝石和连锁构成主要攻防。默认三局两胜，也可创建单局决胜。支持电脑玩家、真人托管、观战和断线恢复。

## 规则与操作

- 6×12 棋盘；双方各自从顶部中央落下一对宝石。
- 碎裂宝石与同色普通/碎裂/强化宝石相邻时触发，清除同色连通组。
- 同色普通宝石形成 2×2 或更大实心矩形会合并为强化宝石；整体下落，整体被同色碎裂宝石清除。
- 每第 25 对出现钻石；落在空地上无效，落在宝石上清除全盘该颜色，攻击减半，反击宝石计时 3。
- 消除会把反击宝石送入对手待落队列；对手每落下一对计时 -1，归零变普通宝石。消除可抵消自己的待落反击宝石。
- 8 名角色各有独立的 12×6 反击宝石图案；开局前双方选角，可选同一角色。
- 三局两胜先赢 2 回合；单局决胜先赢 1 回合；一方无法生成下一对即输掉该回合。攻击数值为平台平衡近似。
- 键盘：←／→ 移动，↑／X 顺时针，Z／Ctrl 逆时针，↓ 软降，空格硬降。
- 手机：棋盘下方按钮；按住左右移动或软降会连续触发。棋盘支持鼠标按钮操作。
- 对局上限：单局 4 分钟，三局两胜 8 分钟（从第一回合开始时计算）。超时比较已完成回合胜场，平局使用平台显式和局。

## 服务端

`games/spf2t/server` 直接实现 `gameapi.Module`：`Realtime: true`，`OnTimer` 每 100ms 推进重力/锁定/结算/回合；`SystemDeadline` 驱动实时帧。`BotAction`/`Reclaim` 与真人使用同一个 `select`/`inputs` 协议。`View` 公开双方棋盘、活动对子、下一对、角色与回合胜场；RNG 和待落宝石颜色不公开。

选角阶段不设置 `Deadline`/`TimeoutActors`，服务端在 15 秒后自动补齐，避免把选角超时变成整局托管。结算期间该席位暂时移出 `Actors`，对手继续操作。

## 测试与验收

```sh
. scripts/env.sh
go test -count=1 ./games/spf2t/server ./games/spf2t/tests
npm --prefix web test -- spf2t
python3 -B scripts/spf2t/verify-assets.py --dist
python3 -B scripts/spf2t/check-browser.py
python3 -B scripts/ci/scope.py run --game spf2t --backend
```

浏览器验收需要已构建的 `web/dist` 和本机 `agent-browser`；结果保存在 `.local/spf2t/browser/`，包含 1280／390／320px 截图、计算样式对比度、选角、键盘与原生触屏、托管、刷新、结果浮层和重赛检查。规则与完整电脑对局证据在 `.local/spf2t/module/`，HTTP 证据在 `.local/poker/spf2t/http/`，参考复刻证据在 `.local/spf2t/reference/`。

## 素材

街机素材来自用户提供的 `.local/spf2t`，派生后上传 OSS 不可变目录 `bga/v20261002-1/spf2t/v1/`，通过 `https://cdn.ichenj.com/bga/v20261002-1/spf2t/v1/` 访问；商业媒体不提交 Git。前端优先使用 CDN，失败时尝试本地 `/spf2t/v1/...`，两者都不可用时使用内置矢量/渐变宝石与首字头像回退。

封面是 `assets/art.ts` 中的原创手绘式 SVG；不使用原版盒装封面、扫描图、Capcom Logo 或大段标题字。素材清单、来源、生成方式与 SHA-256 见 `assets/`；授权状态以 `assets/sources.json` 和 `docs/THIRD_PARTY.md` 为准。

## 上架

新游戏注册后默认上架；管理员可在「管理后台 → 游戏上架」随时下架，下架后服务端拒绝创建。含电脑席位或 `Unrated` 的对局由平台不计分；真人托管仍按真人计分。
