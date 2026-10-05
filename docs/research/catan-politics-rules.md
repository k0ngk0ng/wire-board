# 城市与骑士政治牌补充规则依据

核对日期：2026-10-05。主要效果仍以 `docs/board-expansion-rule-sources.json` 中2025版为准，旧版规则和当前官网FAQ只补充新版省略的细节。

## 官方来源

- [2025 Cities & Knights 规则书](https://www.catan.com/sites/default/files/2025-03/CN3087%20CATAN%E2%80%93Cities%26Knights_%20Rulebook.pdf)，第15–16页：八种主动政治牌及宪法胜利点。
- [官方 Cities & Knights FAQ](https://www.catan.com/faq/cities-knights)，问题83：叛变取得的骑士若本来活跃，可以立即执行骑士行动；问题84：三级骑士无需先拥有要塞；问题85–87：对手决定移除哪名骑士，即使出牌者放不了，仍移除。
- 同一FAQ问题88：外交搬移期间若最终最长路线不比原来短，原持有者不暂时失去奖励；问题89–90：对手在道路中间建村/放骑士不让封闭路线开放，不能拆掉连接己方骑士的必要道路；问题91：可以让建筑失去其最后一条道路；问题94指向下述航海家FAQ。
- [官方 Seafarers FAQ](https://www.catan.com/node/120)，问题18：两个己方建筑或骑士之间的路线封闭，即使敌方在中间阻断也不开放。单一锚点的回环，只允许移动紧邻该锚点的两端；无锚点圆环的每段都开放。附图说明连接圆环的主干A不能移动，而环上的其他段可以。
- [官网规则下载目录](https://www.catan.com/understand-catan/game-rules)列出的[2020 C&K 规则书](https://www.catan.com/sites/default/files/2021-06/catan_c_k_2020_rule_book_200708.pdf)，第17页 Saboteur 解释段明确为“Each of the other players”，排除出牌者；卡面与2025正文同样使用“as many or more”措辞。因此按对手公开分数不低于出牌者筛选，不让出牌者弃自己的牌。城墙与七点的手牌门槛不适用。

FAQ仍使用旧名 Deserter、Diplomat、Saboteur；实现中对应2025的 Treason、Diplomacy、Sabotage，不把其他旧版差异一并套入。

## 获取校验

- 2020 PDF SHA256：`ae335149a56b60418b5561cf9f45dea07cdd00be01362a1e51802f7bf2f024ac`，6310759 字节。
- 2026-10-05 Cities & Knights FAQ 原始HTML SHA256：`e4293106d0b7557222be37735ff7ed1120da94d25630e514384c51ede87d4f54`，152085 字节。

临时下载文件在提取上述依据后清理，正式2025规则书继续使用已固定哈希的研究源。
