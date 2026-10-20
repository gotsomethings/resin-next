# Resin 魔改方案 V3 · 终稿（平台策略 / 平台策略探测与并发治理）

> 状态：已定稿，P1/P2 已实现，P3 进行中。本文件是唯一权威规格，实现与验收以本文为准。

总则：所有“默认值”只是初始值，全部可在面板或 API 修改。

## 一、目标

Resin 是多节点代理池，客户端（opencode2api 等）经它轮换出口 IP 访问上游。上游按出口 IP 封人，而 Resin 目前只按标签/国家/延迟挑节点，不知道哪个 IP 已被封、也不会躲。本方案补上：自主探测 → 判决 → 拉黑 → 躲开 → 自动洗白；加 11 个筛选维度、手动 IP 过滤、拨号失败换节点、资源自适应并发、面板。

明确不做：反代接法、外部日志桥(sidecar)、手动钉 IP、存量告警、探测模板预设、单 IP 限速、egress_dup_max、系统级黑名单。

## 二、探测配置（每平台一套，全部可改）

| 字段 | 默认 | 说明 |
|---|---|---|
| `probe_enabled` | false | 总开关 |
| `probe_url` | opencode 示例 | 任意 URL，仅示例不绑厂商 |
| `probe_method` | POST | GET/POST/PUT/PATCH/DELETE/HEAD |
| `probe_headers` | 内置示例 | 一行一个“名: 值” |
| `probe_body` | 内置示例 | 请求体模板，必须可自定义 |
| `probe_pass_status` | [200] | 这些码算通过，其余全算失败 |
| `probe_follow_redirects` | true | |
| `probe_interval` | 30m | 勿过勤，猛探会把好 IP 探黑 |
| `probe_scope` | 未探测过∪失败过∪黑名单中 | 绝不整池扫 |

要点：探测由 resin 自己发起、出口自己指定，结果天然归属该出口 IP。

## 三、判决与黑名单（每平台每 IP 独立）

状态机（**已定稿语义**）：

- `未知 --通过码--> 正常`（首探通过即 normal，不放宽：单次通过即正常，与“连续 3 次失败才拉黑”对称）
- `正常 --连续失败 N 次--> 黑名单`
- `黑名单 --连续 M 次通过--> 正常`（洗白不放宽，中间任何一次失败清零重计）

参数：`verdict_enabled`(false)、`blacklist_threshold`(N=3)、`rehab_interval`(X=6h)、`rehab_success_threshold`(M=3)。

规则：

- 通过码=过，其余一律算失败（403/429/超时/5xx）。
- 黑名单生效瞬间：①路由剔除该 IP ②踢掉粘在该 IP 上的所有账号租约。
- 平台隔离：同 IP 在 A 平台黑、B 平台照常。
- 新 IP（aimili 换绑）= 全新状态，自动可用不背旧账。
- 台账 GC：IP 超过 `verdict_gc_retention`（默认 **72h**，可配）未在任何节点出现则删行；只删消失的 IP，活着的行不动，重出现按 unknown 重新入账。由 governor 循环每 6h 触发。

黑名单严格生效：**不做任何兜底放行**。当平台可路由 IP 全部被拉黑时，请求直接失败（沿用现有 empty state 的 502），不临时放行“可能也不行”的 IP。

## 四、筛选维度

原 6 条保留；新增全部 AND 叠加、不配不生效、`preview-filter` 可预演。

1. `latency_top_n` — 按最近延迟取前 N（**全局排序，需全量重建**）
2. `latency_max_ms` — 延迟上限
3. `latency_p95_max_ms` — P95 上限（真实样本环，非 EWMA 近似）
4. `min_alive_streak` — 最近 N 次系统探测全活
5. `min_probe_success_rate` — **近 24 小时探测通过率**：24 个 1 小时桶的滚动窗口，rate = succ/total（0–1），作为下限。**无样本（total==0）时该维度不生效（放行）**；同理 `min_alive_streak` / `max_last_probe_age` / `latency_p95_max_ms` 在节点从未探测过（或 P95 无样本）时也不生效，避免重启后内存态清空导致平台空池 502。
6. `max_last_probe_age` — 距上次成功探测上限
7. `min_node_age` — 入池最短时长
8. `egress_ip_version` — v4 / v6
9. `egress_country_filters` — 按**实测出口 IP** 真实归属国（GeoIP）
10. `asn_filters` — ASN 白/黑（需 GeoLite2-ASN，缺库 fail-closed）
11. `ip_type_filters` — 住宅/机房（ASN 组织名启发式）
12. `egress_ip_filters` — 手动白/黑，支持 `!` 排除，IP 与 CIDR

若平台配置了上述任一探测类维度（`min_probe_success_rate` / `min_alive_streak` / `max_last_probe_age` / `latency_p95_max_ms`）但 `probe_enabled=false`，启动时记录 WARN（平台名 + 不生效的维度），且这些维度不会生效；系统不会自动开启探测。

## 五、换路（P4）

参数：`failover_enabled`(false)、`failover_max_switches`(1–10，默认 3)、`failover_deadline`(1s–1m，默认 15s)、`failover_stage`(默认 `connect`)。

设计约束：

1. **只对拨号阶段失败换路**（`DialContext` 返回错误）。隧道建立后的传输错误不重试。
2. **双预算**：换路次数 ≤ `failover_max_switches`，总耗时 ≤ `failover_deadline`，任一耗尽立即停。
3. **成功后必须重钉租约**：`Router.RebindLease` 选一个非故障、仍在可路由视图里的节点，改写该账号的 sticky lease 并更新 IP 负载计数；下一次请求即命中新节点。
4. **全败**：`DeleteLease` 释放该账号租约（不留悬挂），并返回**原错误**（`connect_dial` + 原始 error）。
5. **`failover_stage` 只实现 `connect`**。`first_byte` / `any` 为保留值，写入时**直接拒绝**（400 INVALID_ARGUMENT），不静默降级。

测试：`TestConnectFailover_RebindsLeaseToNewNode`、`TestConnectFailover_BudgetExhaustedReleasesLease`。

## 六、并发治理（P3）

现状：`probe_concurrency` 是只读 env（默认 1000，上限 10000），启动建固定 worker，改值需重启。改为动态限额调度：worker 池按上限预建，派发任务前读原子限额，超限排队。

参数：

- `probe_concurrency` 设定值 1–10000（默认沿用 env）
- `probe_concurrency_mode` = `fixed` | `auto`（默认 fixed）

自动模式采样（每 30s）：

- 内存紧张 = 宿主 `MemAvailable / MemTotal` < 10% **或** 容器 `memory.current / memory.max` ≥ 85%（**不看 swap**：resin 只用内存）
- CPU 紧张 = 1 分钟负载 > 核数 × 0.9 **或** `/proc/stat` 差分 CPU 利用率 > 85%

调节：

- 降档：**任一 30s 采样判定紧张 → 立即 × 0.7**（小内存机器一次紧张就卡）；持续紧张每 60s 再降 30%
- 恢复：连续 3 次宽松 → 生效值 + 设定值 × 30%；连续 3 次宽松（约 90s）一档，回到设定值为止；迟滞带内不计入宽松并清零恢复计数
- 迟滞：压力线 85%、恢复线 70%
- 下限 = 设定值 × 10%，永不归零

落点：config 迁 runtime（env 仅首次默认）；`PATCH /api/v1/system/config` 立即生效；`probe/manager.go` 改动态限额；新增 `probe/governor.go`；系统配置页加并发区。

面板显示：设定值 / 当前生效值 / 模式 / 最近调整原因与时间。

## 七、接口

- `GET /api/v1/platforms/{id}/egress` — IP 台账：状态/失败数/最近结果/节点数
- `PATCH /api/v1/platforms/{id}` — 全部新参数
- `POST /api/v1/platforms/preview-filter` — 试算命中，不动配置
- `GET/PATCH /api/v1/system/config` — 并发设定/模式/生效值

## 八、代码落点

| 文件 | 内容 |
|---|---|
| `internal/platform/platform.go` | 字段 + `evaluateNode` 11 维 + 黑名单判断 |
| `internal/platform/filters.go` | 11 维校验/匹配 |
| `internal/platform/model_codec.go` | 校验/编解码 |
| `internal/model/models.go` | 平台配置 + IP 判决结构 |
| `internal/state/migrations/000010_*.sql` `000011_*.sql` | 迁移 |
| `internal/state/repo_state.go` | 读写 |
| `internal/service/control_plane_platform.go` | 透传 + patch 白名单 |
| `internal/service/platform_verdict.go` | 状态机 |
| `internal/api/{server,handler_platform}.go` | 新路由 |
| `internal/probe/platform_probe.go` | 平台探测器 |
| `internal/probe/governor.go` | 资源治理 |
| `internal/probe/manager.go` | 动态限额 |
| `internal/proxy/{forward,tunnel}.go` | 换路 + 驱逐钩子 |
| `webui/src/features/{platforms,systemConfig}/*` | 面板 |

## 九、分期与验收

| 期 | 内容 | 验收 |
|---|---|---|
| P1 | 筛选面（~700 行） | 配 `latency_top_n=100` + 出口国筛选，preview 命中数 = 实际路由 |
| P2 | 判决面（~1100 行） | 被封 IP 连 3 次拉黑 + 租约被踢；连续 3 次通过自动放回 |
| P3 | 并发治理（~400 行） | auto 模式压内存，生效值按 30% 递减至设定值 10%；压力解除后每 60s +30% 爬回 |
| P4 | 换路面（~400 行） | 坏节点在单请求内被跳过 |
| P5 | 面板（~550 行 TS） | 平台页四块 + 系统页并发区 |

## 十、上线与回滚

- 分支 `feature/platform-policy`；镜像 `resin:policy-v1`
- 上线前备份 `state.db`；迁移只 ADD COLUMN / CREATE TABLE
- 回滚：改回 `ghcr.io/resinat/resin:v1.2.0` + `up -d`

## 十一、红线

1. 探测不扫全量、默认 30 分钟、黑名单 6 小时复探（防自造黑 IP）
2. 非通过码一律计失败
3. 黑名单仅本平台
4. 并发自动模式下限 = 设定值 10%，永不归零
5. 不做反代、不做 sidecar、不做那 5 条附加功能
