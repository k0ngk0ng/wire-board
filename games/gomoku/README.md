# 五子棋

独立两人棋类模块，使用平台统一创建、等待、玩家栏、托管、计时、历史和结算界面。默认上架，管理员可随时下架。

## 规则

- 15×15 交叉点，开局随机分配黑白，黑先。
- `freestyle`：无禁手，连续五子或以上获胜。
- `exact-five`：无禁手，恰好五子获胜，长连不胜。普通开局，不是带 Swap2 开局的国际竞赛预设，也不是有黑方禁手的连珠。
- 满盘无人获胜为和棋。第二手后可以在自己的回合提议和棋，每手一次；不暂停或重置行动时限，继续落子会撤回提议。对方同意才结束。
- 双方可以认输。真人和棋不加减积分，也不计胜场，保存和棋历史。
- 每步 30～300 秒，默认 120 秒。超时转电脑托管，取消托管和重启都保留局面；不能借取消托管延长对手的时限。

## 电脑

默认不依赖外部模型或服务。快速回退优先完成自己的五子、阻挡对方的立即获胜点，再按威胁评分选择邻近候选点。后台搜索使用有预算的迭代加深和 Alpha-Beta，入门与标准分别限制深度、候选数、节点和时间；没有标定 Elo。

搜索通过平台 `BotSearch` 在锁外执行，结果仍由本模块校验回合、对局标识、手数和落点。平台负责丢弃旧版本、取消托管后的旧任务，以及计算失败时的合法回退。`server/` 不依赖其他游戏规则。

## 显示与输入

棋盘保持正方形，PC 不压扁高度；手机交互棋盘至少 480px，可拖动查看。棋子外轮廓至少 24px，小棋盘用于概览和连续拖动定位。鼠标滚轮直接缩放，左键拖动平移；方向键移动焦点，回车选择。选点后需确认才向服务器落子。

图片源为 `assets/art.ts` 中的原创 SVG，封面突出连续五子，和其他棋类分别构图。落子动画和声音只响应新的服务端事件，刷新／重连不重播历史；支持减少动态效果与平台统一静音。

## 素材

4 张 WebP：棋盘底纹、黑棋、白棋、封面。4 个 WAV：落子、获胜、和棋、提示。落子声来自 Kenney Boardgame Pack 的 `Bonus/chipsCollide1.ogg`，CC0；许可文本与包摘要保存在 `assets/licenses/`、`assets/sources.json`。其余旋律使用平台原创 PCM 生成器。

正式地址为部署 `ASSETS_BASE_URL` 后的 `gomoku/v1/images/` 和 `gomoku/v1/audio/`；保持不可变版本目录。本地回退位于 `assets/public/gomoku/v1/`，通过 Vite 合入 `web/dist/gomoku/v1/`。只通过 `BoardProps.audio` 播放，图片不可用时仍保留代码绘制的棋盘和棋子。

生成依赖 Node、Python、`cwebp`、`ffmpeg`，所有工具缓存写入工作区：

```sh
. scripts/env.sh
npm install --prefix .local/chess-family/assets-tools --no-save --ignore-scripts @resvg/resvg-js@2.6.2
node --experimental-strip-types scripts/gomoku/prepare-assets.mjs
python3 -B scripts/gomoku/verify-assets.py --decode
```

`prepare-assets.mjs` 固定检查原包 SHA-256，保留许可，导出无损 WebP、单声道 PCM WAV 和成品摘要。修改已上传素材须提升目录版本，不覆盖不可变对象。当前 8 个文件已上传现有 OSS 基地址，并完成 CDN 内容／MIME 回读；证据位于 `.local/gomoku/assets/cdn.json`。

## 验收

```sh
. scripts/env.sh
python3 -B scripts/ci/scope.py run --game gomoku --backend
WIRE_BOARD_GOMOKU_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 \
  python3 -B games/gomoku/tests/accept_ui.py
python3 -B scripts/gomoku/verify-assets.py --dist --decode
```

规则／HTTP 用例先于实现。`tests/http_test.go` 区分自然开局与明确声明的中局夹具：四向五连走真实落子流程，长连和满盘使用固定局面，不能将后者计为自然完整局。

`TestGomokuBotCompleteGamesHTTP` 在两种规则、两种难度下让生产策略通过真实 HTTP 走完 4 局；该用例使用真人会话转交策略行动，不能冒充原生电脑调度。`TestGomokuAutoPlayCompleteHTTP` 另以真实电脑席位＋托管真人，使用生产计时器走完两种规则各 1 局，中途重启，不加速调度、不直接改局面或胜负。

浏览器用标准空棋盘，仅固定黑白抽签以便重放坐标。验收覆盖创建、中文规则、原生鼠标／滚轮／触屏、选点确认、手机棋子可见性、减少动态效果、刷新、静音、完整连五及公共结算。结果以 `.local/gomoku/browser/result.json` 为准；HTTP 和完整对局轨迹在 `.local/gomoku/http/`，范围检查在 `.local/ci/gomoku/`。
