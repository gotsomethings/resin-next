# Changelog

本文件是唯一的发布说明来源。GitHub Release 的正文由 `.github/workflows/release.yml`
从对应 tag 的段落抽取（`body_path`），**不使用 commit message**。

写入约定：

- 每个版本一段，标题必须是 `## <tag> — <日期>`，tag 与推送的 git tag **完全一致**。
- 段落从 `## ` 开始，到下一个 `## ` 或文件结尾为止。
- 推送 tag 前先在本地跑一遍抽取，确认非空且命中目标版本。

## v1.0.2 — 2026-10-27

### Fixed

- **新建平台默认全是关的**：`defaultPlatformConfig` 用零值 `model.Platform` 构造
  Probe / Verdict / Failover，且不设 `advanced_enabled`，所以 `POST /platforms` 建出来的平台
  探测、判决、黑名单、换路全部不生效。实测新建平台 `advanced=false / probe=false / failover=false`，
  换路因此永远不触发（同一场景 failover 关着时 502，打开后 200 并换到另一节点）。
  现提取 `platform.ApplyNewPlatformDefaults` 作为唯一来源，bootstrap 的 Default 平台与创建接口
  共用：探测恒开、总开关开、黑名单阈值 3、复用 6h / 成功 3 次解除、failover 开（3 次 / 15s / connect）。

## v1.0.1 — 2026-10-27

### Fixed

- **全新安装的高级层全是关的**：`ensureDefaultPlatform` 用裸结构体创建 Default 平台，
  `advanced_enabled` / `probe_enabled` / `verdict_enabled` / `rehab_enabled` 全为 false、
  `probe_url` 为空、阈值与间隔全为 0，结果是**平台探测从不运行**（日志里连一轮 `platform probe …`
  都没有）。现在 bootstrap 按运行时默认值播种：探测恒开（GET `https://www.gstatic.com/generate_204`，
  通过码 200/204，间隔 30m）、总开关开、黑名单阈值 3、复用 6h / 成功 3 次解除、failover 开。
- **总开关根本无法修改**：`advanced_enabled` 不在 `platformPatchAllowedFields` 里，
  `PATCH /platforms/{id}` 直接返回 400 `field "advanced_enabled" is read-only or unknown`，
  面板上的开关点下去一定失败。已补进允许字段表。

## v1.0.0 — 2026-10-27

### 中文摘要

Resin-next 的首个发布。**版本号从 v1.0.0 重新起算，与上游 Resin 的版本线无关。**
在这一版里重构了「平台策略」：把探测、判决、黑名单、高级路由和 IP 列表收敛成一套一致的口径，
并修复了平台探测重启后停摆、成功率恒为 `-`、手动拉黑 400 三个阻塞上线的缺陷。

### Platform policy (advanced routing)

- **Advanced mode switch**：新增 `advanced_enabled`（默认开启）。关闭后平台只按基础配置过滤
  （订阅启用 + 标签 + 区域 + 有效出口 IP），策略维度、探测判决、黑名单全部不生效。
- **Probe config cleanup**：探测恒为开启，移除「应用探测」开关；删除三条死配置
  （跟随重定向、探测间隔全局化、探测范围全量）。
- **Verdict & blacklist moved into probe config**：判决与黑名单并入探测配置区；
  黑名单阈值常开；新增「黑名单复用」开关控制 rehab 行为。
- **Manual IP blacklist config removed**：删除平台的「手动 IP 黑白名单」配置项，
  迁移 `000017_platform_drop_egress_ip_filter` 清空存量数据。手动拉黑改为 IP 列表上的开关。
- **Pass-rate window**：探测通过率窗口从固定 24 小时改为任意分钟数（最小 10），
  基于样本环存储，不再依赖分桶。
- **Routing settings renamed**：界面文案「换路设置」→「高级路由设置」，「应用换路」→「应用高级路由」。

### IP list

- 列表展示 **平台过滤后的 IP**（健康 + 平台自身规则），黑名单不参与筛选 ——
  被拉黑的 IP 仍留在列表里，可随时解除、也能看到恢复情况。
- 统计行按 **IP 计数**：可用 IP / 探测失败 IP / 已拉黑 IP / 待测 IP。
- 排序支持加入时间、失败次数、探测时间（升/降），默认加入时间倒序。
- 手动拉黑保留滑动开关交互，改乐观更新，不再被全局 mutation 锁死；手动拉黑不会被自动洗白。

### Probe & verdict

- 单次探测硬超时（2× 超时配置，默认 30s），超时按失败记账并告警；每轮结束输出摘要日志
  （candidates / pass / fail / elapsed）。
- 无候选时也输出诊断日志（nodes / healthy / matched）。
- 判决台账清理协程：每 2 分钟扫描，IP 离开节点池超过 5 分钟后删除对应台账行。

### Fixed

- **平台探测重启后停摆**：`BuildFromModel` 未恢复 Probe / Verdict / Failover 配置，
  重启后 `Probe.Enabled=false`、探测 URL 为空，扫描直接跳过。
- **成功率恒为 `-`**：bootstrap 构造节点时绕过 `NewNodeEntry`，`ProbeWindow` 为 nil，
  所有探测样本被静默丢弃。新增 `InitProbeWindow()` 兜底。
- **手动拉黑返回 400**：请求体被 JSON 序列化两次，后端 `DisallowUnknownFields` 解析失败。
- **同 IP 轮换误续期**：轮换（出口 IP 不变、只换物理节点）应当保留原租约过期时间，
  只有轮换落空、真正重建租约时才按 `now + StickyTTL` 重新计时。

### Upgrade notes

> [!IMPORTANT]
> 启动时自动执行数据库迁移（`state.db` 升到 v18），包含不可逆的
> `000017_platform_drop_egress_ip_filter` 与 `000018_platform_advanced_switch`。

- 升级前先备份 `<RESIN_STATE_DIR>/state.db`。
- 迁移后无法直接用之前的版本回退，只能恢复备份。
- 平台「可用节点」的统计口径改为「健康 + 基础配置」，数字会比旧版小，属于预期。
