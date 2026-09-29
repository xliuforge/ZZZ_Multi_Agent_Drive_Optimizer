# 配装结果的职业面板

核对日期：2026-09-29。面板字段根据职业选取，数值使用配装后的最终属性；资料站中的“基础”属性和用户示例截图用于确认字段，不直接覆盖配装结果。

## 命破闪能

| 角色 | 基础闪能自动累积 | 依据 |
| --- | --- | --- |
| 星徽·比利 | 2 | 用户本次提供的属性截图 |
| 仪玄 | 2 | [Gachabase 正式服](https://zzz.gachabase.net/agents/1371/yixuan/release?lang=chs) |
| 伊德海莉 | 2 | [Gachabase 正式服](https://zzz.gachabase.net/agents/1051/yidhari/release?lang=chs) |
| 般岳 | 2 | [Gachabase 正式服](https://zzz.gachabase.net/agents/1471/banyue/release?lang=chs) |
| 真斗 | 0 | [Gachabase 正式服](https://zzz.gachabase.net/agents/1441/manato/release?lang=chs) |

数值储存在 `baseAdrenalineRegen`，通过 `ADRENALINE_REGEN` 传递。最终结果的 `panelAdrenalineRegen` 始终序列化，保留真斗的明确零值。普通 `ENERGY_REGEN` 不参与闪能计算。当前仅显示常驻自动累积，角色动作、影画或队伍触发的闪能恢复不在此栏中累计。

命破显示十项核心面板：生命、攻击、防御、冲击、双暴、异常掌控、异常精通、贯穿力、闪能。贯穿伤害不使用穿透率/穿透值，结果主面板不列这两项。已有的静态伤害加成额外列出。

`panelSheerForce` 用初始攻击、生命及静态贯穿力加成计算；`sheerForce` 保留原有实战计算和评分用途。两项分开，不能将音擎技能或四件套触发效果写入初始贯穿力。

## 锋御与普通职业

锋御以锐暴伤害替代攻击、锐能自动累积替代普通能量回复。克拉蕾数值沿用 [角色来源记录](CLARET_SOURCES.md)。其他职业显示普通的生命、攻击、防御、冲击、双暴、异常掌控、异常精通、穿透率和能量自动回复。适用时补充穿透值与已有的静态伤害加成。

单角色、多角色实际分配和接近目标候选复用同一面板组件。所有核心属性保留零值；旧结果缺少新字段时显示“—”，不回退到实战值或虚构数值。触发收益在实战参考和属性加成明细中单独显示。
