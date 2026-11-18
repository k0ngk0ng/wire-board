# 京城支线与交易验收

本批依据当前 Windows 简体原始数据、[京城蝶舞攻略](https://gamegene.cn/wiki/362)及[扬州擒贼的渡口段落](https://gamegene.cn/wiki/361)核对。缓存为 `capital-text-guide.md`、`yangzhou-text-guide.md`，不观看视频。

## 实现前失败场景

1. 情书对话确认前保存丢失选择，恢复重发情书，或把站岗守卫删除。普通菜单确认、提示、取得后和后续交谈分别验证。
2. 花园天仙玉露从错误道路接近，或把可见瓶子当成宝箱；民居银针被邻近 NPC 对话挡住却直接补发。按原版入口与搜索范围行走，保留药品、守卫和房间原始位置。
3. 资助把余额判断中的临时扣还误当成再次给钱，或把低于 30000 的条件和三次上限删掉。分别验证首次领取、高余额不再领取、低余额拒绝／接受、三次后不再发放及各次续档。
4. 为降低余额直接改钱、丢道具或使用测试旁路。须在原版商店实际购买，逐件核对商品、价格、库存及钱包，并保存商品选择、确认与成交后状态。
5. 糖葫芦按物品标价 20 而非原脚本售价 30 扣款，或酒不是每壶 100 文；拒绝购买仍扣钱／给货，恢复时重扣或重发。按实际买卖脚本逐笔验收。
6. 将合法的新一次购买当成重复奖励，强制商贩只卖一次；或把两次购买算成两个隐藏事件。原版可重复交易保留，恢复本身不得产生新的成交，同一商贩事件只计一次。
7. 交易中退出菜单绕过扣款，存档后返回了错误场景、队伍或选中项。通过正常输入操作，保留原版玩家、角色顺序和菜单上下文。
8. 不同报告和复验覆盖旧档或重复增加进度，或把旧点构建身份换成新核心。使用独立前缀，逐项核对旧进度和冻结产物，只追加本批已验证事件。
9. 云姨按原自动脚本移动后离开可视区，测试仍在资源表的初始坐标搜索。不可见时沿原道路走到北侧停留点 `672,1264`，进入可视区后使用公开人物坐标；重复站位须及时失败并保留现场。
10. 商贩拒绝或成交后直接返回场景，测试仅以对白计数判断完成，又重复打开同一菜单并等待。交易完成须以本次已提交选择和正常返回场景为依据，另行核对钱包与库存；选择提交后的等待须有上限。

## 当前范围

计划核对情书、花园天仙玉露、民居银针、云姨资助、糖葫芦购买及酒购买 6 个候选。情书的完整递送链和古董商家布包返还另列后续，不以收到情书冒充整条支线结束。

## 已通过范围

6 个事件通过实际领取或交易、原始脚本与原生状态只读核对，共 93 次全新运行器保存恢复。一次性三点每点通过领取前、提示中、领取后和 3 次实际重复搜索后的恢复；情书另保留原确认菜单，守卫不消失，天仙玉露按原指令消失，银针保持隐藏。

云姨资助保留首次直接给钱、后两次低于 30000 文时的拒绝／接受、高余额不再给、三次后低余额也不再给。三笔共 90000 文，原武器店按每把 12000 文购得金童剑 7 把，共 84000 文；最终余额为 20499 文。每次商品选中、原确认选项、成交、库存及资助提示分别保存恢复。

糖葫芦按每个 30 文、酒按每壶 100 文，各执行「拒绝、购买、购买、拒绝」。库存分别增加 2，钱包分别减少 60／200 文；交易选择前后续档不自动重新扣款或发货。原版重复购买能力保留，不计多点。

云姨寻人和商贩完成判定的改动仅在测试运行器中；原核心、事件表、脚本、位置、碰撞和数值未改。普通三点共 13 次恢复，资助及实购 56 次，两商贩各 12 次。旧进度与 449 个不可变产物／主线报告按摘要复核，本批使用独立前缀；上批蛤蟆山归档保持。

## 复现与产物

资源与已登记根档准备完毕后，在仓库根目录执行。三个区域分别将 `PAL_HIDDEN_REGION` 设为 `101`、`109`、`138`：

```sh
. scripts/env.sh
PAL_CAMPAIGN_VARIANT=audio PAL_DEVELOPMENT_HEALTH=9999 PAL_TEST_AUTHORITY=1 PAL_HIDDEN_REGION=101 go test -v ./games/pal/tests -run '^TestOriginalRegionalHiddenRewardsAndSavedNoDuplication$' -count=1 -timeout=6m
PAL_CAMPAIGN_VARIANT=audio PAL_DEVELOPMENT_HEALTH=9999 PAL_TEST_AUTHORITY=1 PAL_CAPITAL_ECONOMY=1 go test -v ./games/pal/tests -run '^TestOriginal(CapitalThreeFundingGrantsWithRealPurchasesAndSave|SugarAndWineRepeatPurchasesCancelAndSave)$' -count=1 -timeout=12m
python3 -B games/pal/tests/capital-side-original-review.py
python3 -B games/pal/tests/hidden-progress.py
python3 -B games/pal/tests/capital-side-evidence-e2e.py
```

实际日志为 `regional-capital-side-<scene>-final-run.txt`、`capital-economy-final-run.txt`，逐次存档及 proof 位于 `.local/pal/verification/runtime-candidates/audio/development-health/hp-9999/`。原生只读报告为 `.local/pal/verification/capital-side-original-review.json`；汇总与冻结清单为同目录的 `capital-side-hidden-stage.json`，检查旧点、旧证据和完整本批账目，当前累计已处理 336/732。

原生只读核对校验真实核心摘要、完整内存摘要及当前事件表锚点，排除 SSS 文件副本；核对云姨入口变更、瓶子消失指令、隐藏点状态、选中菜单、钱包和重复交易入口。全程无原生编辑、无资源上传。本批不代表完整情书／布包支线、性能或发布验收通过。
