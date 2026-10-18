# 战役装备属性与换装预览

战役准备、中场和下一场准备现在显示所选球员的奔跑、跳跃、投篮、恢复、攻击等级。选择库存物品先查看「当前／换装后」及增减，确认后才提交原装备操作。双人对战继续使用原版 Mode 128，不提供装备规则或装备入口；这是用户明确确认的选择。

## 原版计算与预览范围

原 Scene 2 `8CEE` 从 C1D8 角色表读取基础等级，通过 `8FC1 → AD8F` 按角色装备槽计算，之后在 `8D26` 开始写入显示信息。`91DD` 未确认分支经 `949A` 绘制库存，再调用 `8CEE`；移动库存光标不会先把候选物品写入角色装备。

因此本次区分两种显示：

- **当前等级**：复现原菜单已经穿戴装备后的数值。
- **换装后**：平台提供的确认前比较；在副本上组合已验证的 `campaign.Equip` 和同一套 Go 属性计算，不声称原菜单光标本来就有这种比较显示。

所有计算留在 Go。前端只显示服务端数值及差值，不复制装备加成表。预览不修改库存、装备、比赛时间或 RNG；实际计时器仍按既有平台规则运行。中场读取已接受的待提交装备，不能误用上一节的旧配置。

属性以原版 0～7 档显示。原程序先按上衣、短裤、鞋子顺序运算，最后统一进行带符号截断；逐件截断会改变叠加结果。五项数值不变不等于没有特殊效果，投篮、跳跃、攻击等额外分支单独见 [装备特殊效果目录](../EQUIPMENT_EFFECTS_SOURCE.md)。

## 原指令证据

`scripts/basketball/verify_campaign_menu_grades.py` 使用既有 MMC3 Memory 和 py65，执行未修改的 bank 3 `8CEE..8D26`，包含 `8FC1/AD8F`，不跳过函数调用，在字形渲染前停止。

矩阵为两种战役人数 × 四名己方球员 × 1,584 个合法槽位组合，共 **12,672 次原函数调用**。每种槽位含空槽及对应类别的全部物品。生成器检查进度／库存／装备不变，RNG 原值不变且没有调用 F9C3；元数据保留受控基准 RAM、覆盖字段、CPU 与 mapper 校验和。它是受控函数矩阵，不是自然获得装备的历史，也不是 12,672 场比赛。

夹具：`engine/testdata/campaign-menu-grades.json.gz`。解压 JSON SHA-256：`d96e8acee3efa854529140a04f836251bc526787a3f2bc02e3f1ebaa914559fa`。重复执行 `--check` 与保存的产物完全一致。

Go 投影对照全部原调用，另外检查角色之间不串装备、非法类别／空洞库存／进度拒绝、客户端伪造显示数值被重算、中场采用当前库存，以及投影不修改权威比赛快照。

## 界面与验收

选中库存物品只产生本地预览，确认请求仍绑定当时的 epoch、period、revision 和库存索引。新版本使未确认预览失效。13 件库存的手机用例会选择最后一件，检查能力比较和确认按钮自动进入可视区；不是只验证 DOM 存在。

原密码 `400110` 重建物品 0 和 3；穿上钢铁球衣后预览闪亮球衣，实际显示奔跑 4→5、投篮 6→7、恢复 5→4。该用例验证有提升也有下降，不能把装备一律画成绿色加成。另保留无数值变化的火箭球衣用例。

- 独立 React 浏览器验收通过，包含真实服务端视图、原生鼠标／触屏、当前属性、增减、未确认不提交、版本失效、长库存手机可见性和既有弹窗检查。
- 真实生产构建页面验证预览不改变服务端库存，确认后才穿戴；同时检查双人对战实际 Runtime 为 Mode 128，装备全为空。
- 属性增减及正文的最终计算颜色通过对比度检查，保存桌面和 320px 截图。
- 六个相关顶层服务端 race 测试通过。首次 race 发现观战请求可能先因房间版本变化返回 409，而非权限返回 400；验收现允许这两类拒绝，并核对权威奖励／阶段完全未变，没有把失败请求当成成功或跳过检查。

工作树产物在 `.local/campaign-attributes/`，浏览器产物仍在 `.local/campaign-ui/` 和共享 `.local/basketball/platform-rooms/`。保留初始编译失败、无属性界面的失败、长库存比较离开可视区的失败及首次 race 记录。

复现：

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_campaign_menu_grades.py --check
go test -race ./games/basketball/server -run '^TestCampaignAttributes|^TestCampaignEntry|^TestCampaignPreparationHTTP|^TestCampaignLifecycleHTTP' -count=1 -v
python3 scripts/basketball/accept_campaign_ui.py
npm --prefix web run build
BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowserPlatformRooms$' -count=1 -v
```

本次没有修改场内装备物理、AI 或胜负。装备特殊效果来源调查发现的跳跃／落地接线缺口，以及 [赛后动画](../POSTRESULT_ANIMATION_SOURCE.md)、完整赛程和既有悬空球失败，仍须继续处理；属性矩阵不能代替这些验收。
