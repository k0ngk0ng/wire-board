# 友善强盗：规则依据与实施边界

核对日期：2026-10-05。内部规则引擎阶段；尚无房间配置/界面/战绩标识，未开放或发布。

## 官方依据

- 2025《Traders & Barbarians》第4页 **The Friendly Robber**：不能将强盗放到只有2胜利分玩家的建筑旁；没有合法地块时返回沙漠；此时只能向超过2分且邻接该沙漠的玩家偷取资源。该页组合条款列出基础游戏、其他T&B变体/剧本与航海家。
- 2025《Traders & Barbarians 5–6》引言：沿用T&B规则，并调整地图与配对回合。两份PDF固定来源和SHA256见 `../board-expansion-rule-sources.json`。
- [官方 T&B FAQ](https://www.catan.com/faq/traders-barbarians)，问题53–56。原HTML SHA256 `9443d536c318a4e629d5bcd37a7c2c72e3403f81255915cd3cf6cf9cb77a734d`，以下逐行提取文本（末尾换行）SHA256 `e75a8649fd46a28482ac9a1f9fb1ba29e1f73428db7e721f0320233f188c2af1`。临时HTML摘录后清理。

```text
> The Friendly Robber + Seafarers (in general) - Is the pirate also friendly when “Seafarers” is combined with “The Friendly Robber”? (53) Traders & Barbarians Yes.
> The Friendly Robber - Are victory points on face-down cards also taken into account when determining next to which player I may place the friendly robber? (54) Traders & Barbarians No. Only visible victory points are considered.
> The Friendly Robber - Do I have to place the friendly robber on a hex adjacent to which I have a settlement or city if any other position would affect players with less than three victory points? (55) Traders & Barbarians Yes. However, this will happen only in very rare cases.
> The Friendly Robber - If all my opponents have only two points each, may I drive away the robber without stealing a card? (56) Traders & Barbarians Yes.
```

## 已接入规则

- 依据FAQ55的“不足3分”，按公开分数小于3判定保护，包括当前玩家自己。总分扣除隐藏胜利点；手牌数量、资源种类、隐藏胜利点卡不会改变保护。中立建筑与已退出玩家不受此保护。
- 普通强盗目标不能邻接受保护玩家的村庄或城市，即使同一地块也有可偷取的高分玩家，仍不可选择该地块。自己的高分建筑也不是跳过目标的理由。
- 没有其他合法陆地时允许沙漠退路，退路可以邻接受保护者，但偷取候选仍只含公开至少3分的其他玩家。若唯一沙漠就是强盗当前位置，则保持在沙漠并完成这次处理；这是将“无合法位置返回沙漠”的例外应用于已有位置，不宣称FAQ单独说明了原地退路。
- 基础骑士牌可以移动强盗而不偷牌；升级建筑或获得公共奖励后按最新公开分数决定保护。变体不调整胜利门槛、弃牌阈值或通常回合倒计时。
- 航海家通用海盗目标根据邻接船只主人的公开分数保护，岸边只有建筑而没有船只不因此阻挡海盗。目标校验、前端合法位置和电脑使用同一函数；到外框后不偷牌。这里是通用位置规则的局部验证，不代表全部剧本组合已经支持。
- 公共视图提供受保护玩家编号，不输出隐藏卡牌；状态及版本可以保存恢复，变体动作使用原子副本拒绝非法请求。
- 内部基础构造器支持3–6人和已有固定/可变布局；内部组合测试叠加港口霸主。Helpers未核验，构造器拒绝。

## 待核验的组合边界

尚未添加航海家/城市骑士/Helpers的友善强盗组合构造器或公开选项。以下缺口保留在整体范围内：

- 无沙漠地图中所有陆地均受保护时，规则书的“返回沙漠”如何适用；特别是海盗已在外框而所有可移动海洋也受保护的情况。
- 遗忘部落只允许移到有数字的地块，其无数字沙漠与退路规则的优先级。
- 海盗群岛自动巡航/袭击及掷7任意偷取，不能直接当成普通海盗移动或套用FAQ53就宣称完成。
- 城市骑士实体骑士、征税等与友善强盗的组合，以及Helpers的直接送回沙漠效果。2025 T&B此页没有列城市骑士，尚不将遗漏当作明确禁止组合，也不擅自宣布支持。

现有未启用变体的对局不采用这些新限制。未核实的特殊情况不能用“随机选一个位置”或悄悄跳过规则来掩盖。

## 验证记录

- 定向验证公开/隐藏胜利点、当前玩家自己的建筑、中立与退出玩家、空地、不能提前退回沙漠、沙漠原地退路、退路中只偷高分玩家、骑士不偷牌仍可打出、实际付费升级后保护消失、港口奖励可见性、旧偷牌候选原子拒绝、保存恢复、观战/他人隐私以及电脑不读取对手隐藏牌。海盗局部夹具核对船主保护、岸边建筑不阻挡、隐藏VP、合法提示/电脑与外框。
- 12场基础3/4/5/6人完整电脑局，覆盖五六人固定/可变、普通友善强盗及叠加港口霸主。278–681步，20–49次强盗移动，10/11分获胜；每步检查资源、发展卡、棋子库存和受保护地块，周期完整保存恢复。该轮含定向15.915秒通过；海盗验证仅局部夹具，不是完整海图组合对局。
- 三条真实HTTP路径实际重启后分别手动移动、托管移动、超时弃牌后手动移动。核对公开保护不受隐藏胜利点影响、不同身份的合法提示、手牌隐私、错误请求完整Room不变、退路只偷高分玩家。手动/托管移动保留原45秒期限；超时弃牌按现有规则进入强盗阶段并有120秒，随后移动不再刷新期限。没有改成普通强盗选择超时就自动执行——普通行动仍沿用平台既有超时/踢人机制。三路径1.359秒、并发10.819秒通过。
- 无前端变更、浏览器、正式等待配置或完整HTTP对局验收；没有新增素材或上传。房间/界面/历史及组合验证继续待做。

相关基础/航海家/金矿与Helpers完整局、海盗移动、部落限制、布匹边界等引擎回归205.600秒通过，game/server静态检查通过。临时官网HTML及本阶段测试输出已清理；未改前端、开放入口或发布。下一阶段接入内部基础配置、保护提示、规则/历史和正式开局验收，所有特殊组合及整体扩展范围不变。
