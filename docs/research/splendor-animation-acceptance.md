# 宝石滚动动画验收

2026-10-11。本阶段关闭拿取宝石、归还宝石、市场预留牌在滚动场景下的页面验收项；未修改生产动画代码。

## 覆盖范围与结果

通过本地真实 HTTP 注册四位玩家、建房和开局，用合法操作推进到超过十枚宝石后的归还阶段。浏览器通过实时连接接收状态，动画期间横向滚动玩家栏并纵向滚动页面；不注入牌局快照、不暂停或延长 CSS 动画。

| 配置 | 视口 | 结果 |
| --- | --- | --- |
| 基础版 | 390 × 844 | 通过，15 次操作 |
| 东方、城市、贸易站、要塞同开 | 320 × 844 | 通过，14 次操作 |
| 同上 | 1440 × 844 | 通过，17 次操作 |

每帧检查飞行元素的来源及目的地坐标是否对应实际宝石堆、市场卡位和正确玩家头像，允许误差 1 像素。滚动后的三个同步帧不计入坐标断言；后续帧持续验证。超出可视区域或被玩家栏裁切的头像不能成为可见飞行动画的目的地。页面无横向溢出，无 JavaScript 异常。

320 像素扩展页面的一级卡位可能位于屏幕下方。早期探针预留该卡时没有可见飞行轨迹，属于来源不可见的预期行为；正式探针选取屏幕内的三级卡位。对坐标及隐藏状态的断言保持不变。随机先手可能使其他玩家先预留满三张，脚本之后改用合法取宝石动作继续推进。

这是桌面 Chromium 模拟视口的布局与动画坐标验收，不作为真实手机帧率测试。购买预留牌、贵族到访、东方连锁免费卡和其他扩展专属交互仍待页面验收；不宣称全部宝石扩展已完成整版验收。

## 复跑

先构建并在独立的本地测试数据库上启动服务，素材地址从私有环境配置传入，不写入脚本：

```sh
source scripts/env.sh
npm --prefix web run build
go build -o .local/splendor-accept .
DATA_DIR="$PWD/.local/splendor-accept-data" \
ADDR=127.0.0.1:18099 INVITE_CODE=page-accept-code \
ASSETS_BASE_URL="${ASSETS_BASE_URL:?请配置实际素材基地址}" \
.local/splendor-accept
```

在另一个终端从仓库目录执行：

```sh
source scripts/env.sh
python3 scripts/accept_splendor_animation.py --width 390
python3 scripts/accept_splendor_animation.py --expanded --width 320
python3 scripts/accept_splendor_animation.py --expanded --width 1440
```

脚本只接受本地服务地址，使用独立且静音的浏览器会话。每次产物位于 `.local/splendor-animation-<宽度>-<运行编号>/`，包含 `opening.png` 和 `result.json`。失败也保存结果；退出时关闭测试浏览器并清理该次浏览器配置目录。测试账号只存在于指定的临时数据库中。

本轮产物目录：

- `.local/splendor-animation-390-5e326aa2/`
- `.local/splendor-animation-320-1ad9ab0f/`
- `.local/splendor-animation-1440-0c00e396/`

验收后停止本地服务，清理测试数据库和临时二进制，保留结果及截图作为证据。
