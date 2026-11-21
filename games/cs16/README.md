# 反恐精英

CS 1.6 的独立网页复刻，游戏 ID 为 `cs16`。首版支持 2 至 10 人，默认 10 席、T／CT 两队各最多 5 人；接入平台的房间、准备、电脑、主动托管、观战和结算。

## 内容

| 类型 | 范围 |
| --- | --- |
| 地图 | de_dust2 沙漠 2、de_inferno 炼狱小镇、de_aztec 阿兹特克、cs_assault 仓库突击、fy_iceworld 冰雪世界、cs_bloodstrike 血战回廊 |
| 玩法 | 三张爆破地图、仓库人质营救，以及冰雪世界与血战回廊的回合歼灭 |
| 手枪 | Glock 18、USP、P228、Desert Eagle、Dual Elites、Five-SeveN |
| 霰弹枪 | M3、XM1014 |
| 冲锋枪 | MP5 Navy、TMP、P90、MAC-10、UMP45 |
| 步枪 | Galil、FAMAS、AK-47、M4A1、SG552、AUG |
| 狙击枪 | Scout、AWP、G3SG1、SG550 |
| 机枪 | M249 |
| 装备 | 小刀、HE、闪光、烟雾、C4、护甲、头盔、拆弹器和弹药 |

原图 BSP、WAD 贴图和光照以及原人物／武器 MDL 经离线转换供 Three.js 使用。服务器只保留计算必需的碰撞、实体、导航和模型规则数据；客户端模型、地图绘制数据、声音和 WASM 保持为外部资源。

房主可在创建或等待房间时设置「误伤队友」，默认关闭，开局后固定。开启后枪械和小刀队伤为 35%，队杀扣 1 frag 和 3300 资金；资金最低为 0。关闭误伤时队友仍参与碰撞和挡弹。

## 操作

PC 点击战场锁定鼠标，WASD 移动，左键开火，右键使用武器特殊功能；Space 跳跃，Ctrl 蹲下，Shift 静步，R 装弹，E 使用／拆弹／营救，G 丢枪。1～5 选择武器栏，Q 切回上一把，B 购买，Tab 查看战绩。公共控制器支持改键，方向键也可转向。

手机使用左侧摇杆移动，拖动右侧区域转向，配合开火、跳跃、蹲下、装弹和使用按钮；武器栏与购买入口可直接点按。释放、失焦或切到后台会清除瞬时输入。

## 同步

Go 引擎以 100 Hz 候选规则步推进，网络投影与显示刷新分别调度。浏览器使用同源 Go 编译的 WASM 预测本人运动，服务器确认后重放未确认输入；伤害、经济、目标和胜负由服务器决定。动态门和活人碰撞参与运动，历史命中补偿具有上限。

私有存档格式 2 将完整历史保存为有长度和校验约束的 gzip 冷检查点，规则推进时再展开，兼容格式 1 的旧 JSON 历史。普通输入只更新当前席位，不重复处理历史正文；冷检查点不进入公开投影或恢复网络包。

客户端只发送操作。服务器按每个连接的可见范围和基线发送变化字段、实体新增／移除及新增事件；首次和恢复时发送完整公开视图。固定地图、规则和选项不随常规帧重复发送，HTTP 快照不替换 WebSocket 解码基线。

游戏采用明确单位的定点整数，渲染变换使用浮点数。原版 GoldSrc 使用浮点运动，当前实现须以已保存的轨迹和参数对照说明差异，不声称逐位等价。

## 原版样本与证据

研究文件位于 `.local/cs16/research/`。原始资源来自固定提交的第三方公开归档，逐文件保存 SHA-256 和 Git blob 校验；官方真实性尚未建立。ReGameDLL_CS 参考源码另行固定提交。

REA 对候选 `mp.dll` 完成 PE32 身份与资源清点。`cs.so` 的符号和 DWARF 另通过本机工具读取。当前缺少已配置的深度反编译提供者，没有运行或反编译原版，因此这些静态证据不能证明运行行为完全一致。完整来源、命令和边界见 `.local/cs16/research/FINDINGS.md`。

## 检查

从固定来源恢复原始素材并重新生成资源：

```sh
python3 -B scripts/cs16/fetch_sources.py
python3 -B scripts/cs16/fetch_sources.py --check --reference
python3 -B scripts/cs16/build_assets.py
node games/cs16/build/runtime-delivery.mjs
```

`--check` 完全离线。参考源码尚未恢复时，先显式运行 `fetch_sources.py --reference`。普通 CI 只使用已经校验的资源缓存，不自动执行下载入口。发布资源按内容摘要登记与复用，上传过程使用 HEAD／PUT；最终应用构建另行执行本地 CDN 回读。社区小图来源与增量转换见[原图接入记录](../../docs/research/cs16/community-maps.md)。

先显式创建本游戏的缓存和临时目录，再运行工具。测试结果保存在 `.local/cs16/verification/`；具体通过范围以本次 JSON 报告为准。

```sh
mkdir -p .local/cs16/go-cache .local/cs16/go-mod .local/cs16/tmp .local/cs16/verification
GOCACHE="$PWD/.local/cs16/go-cache" GOMODCACHE="$PWD/.local/cs16/go-mod" GOTMPDIR="$PWD/.local/cs16/tmp" TMPDIR="$PWD/.local/cs16/tmp" GOTOOLCHAIN=local GOTELEMETRY=off go test -json -count=1 -timeout=20m ./games/cs16/engine ./games/cs16/server ./games/cs16/tests
node --experimental-strip-types --test games/cs16/tests/*.test.mjs
node games/cs16/build/lint.mjs
node games/cs16/build/runtime-delivery.mjs
python3 -B games/cs16/tests/accept_browser.py
```

浏览器验收使用生产 HTTP／SQLite／WebSocket 房间与正式前端，2 个真人页面和 8 个电脑组成 10 席。触屏通过 Chromium 原生模拟，不代表某款实体手机或手柄已经实测。规则测试、原生／WASM 运动一致、正式页面与负载是不同的验收边界，不能相互替代。

本次检查范围、结果与交付边界见[本地验收记录](../../docs/research/cs16/verification.md)。
