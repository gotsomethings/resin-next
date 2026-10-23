import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "../../components/ui/Button";
import { Select } from "../../components/ui/Select";
import { Switch } from "../../components/ui/Switch";
import { Textarea } from "../../components/ui/Textarea";
import { Input } from "../../components/ui/Input";
import { useI18n } from "../../i18n";
import { formatApiErrorMessage } from "../../lib/error-message";
import { listPlatformEgress, updatePlatform } from "./api";
import { getGeoIPStatus } from "../geoip/api";
import type { EgressIPRow, Platform, PlatformExtras, PlatformUpdateInput } from "./types";

type Props = { platform: Platform };

type FormState = {
  probe_enabled: boolean;
  probe_url: string;
  probe_method: string;
  probe_headers: string;
  probe_body: string;
  probe_pass_status: string;
  probe_follow_redirects: boolean;
  probe_interval: string;
  probe_scope: string;
  verdict_enabled: boolean;
  blacklist_threshold: number;
  rehab_interval: string;
  rehab_success_threshold: number;
  latency_top_n: number;
  latency_max_ms: number;
  latency_p95_max_ms: number;
  min_alive_streak: number;
  min_probe_success_rate: number;
  max_last_probe_age: string;
  min_node_age: string;
  egress_ip_version: string;
  egress_country_filters: string;
  asn_filters: string;
  ip_type_filters: string;
  egress_ip_filters: string;
  failover_enabled: boolean;
  failover_max_switches: number;
  failover_deadline: string;
  failover_stage: string;
};

function toForm(platform: Platform): FormState {
  const p = platform as Platform & Partial<PlatformExtras>;
  return {
    probe_enabled: p.probe_enabled ?? false,
    probe_url: p.probe_url ?? "",
    probe_method: p.probe_method ?? "POST",
    probe_headers: p.probe_headers ?? "",
    probe_body: p.probe_body ?? "",
    probe_pass_status: (p.probe_pass_status ?? [200]).join(","),
    probe_follow_redirects: p.probe_follow_redirects ?? true,
    probe_interval: p.probe_interval ?? "30m0s",
    probe_scope: p.probe_scope ?? "untested,failed,blacklisted",
    verdict_enabled: p.verdict_enabled ?? false,
    blacklist_threshold: p.blacklist_threshold ?? 3,
    rehab_interval: p.rehab_interval ?? "6h0m0s",
    rehab_success_threshold: p.rehab_success_threshold ?? 3,
    latency_top_n: p.latency_top_n ?? 0,
    latency_max_ms: p.latency_max_ms ?? 0,
    latency_p95_max_ms: p.latency_p95_max_ms ?? 0,
    min_alive_streak: p.min_alive_streak ?? 0,
    min_probe_success_rate: p.min_probe_success_rate ?? 0,
    max_last_probe_age: p.max_last_probe_age ?? "0s",
    min_node_age: p.min_node_age ?? "0s",
    egress_ip_version: p.egress_ip_version ?? "",
    egress_country_filters: (p.egress_country_filters ?? []).join(","),
    asn_filters: (p.asn_filters ?? []).join(","),
    ip_type_filters: (p.ip_type_filters ?? []).join(","),
    egress_ip_filters: (p.egress_ip_filters ?? []).join(","),
    failover_enabled: p.failover_enabled ?? false,
    failover_max_switches: p.failover_max_switches ?? 3,
    failover_deadline: p.failover_deadline ?? "15s",
    failover_stage: p.failover_stage ?? "connect",
  };
}

function splitList(raw: string): string[] {
  return raw
    .split(/[,\n]/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function splitStatuses(raw: string): number[] {
  const out = splitList(raw)
    .map((s) => Number.parseInt(s, 10))
    .filter((n) => Number.isFinite(n) && n >= 100 && n <= 599);
  return out.length ? out : [200];
}

function stateBadge(state: string): string {
  if (state === "blacklisted") return "badge badge-danger";
  if (state === "normal") return "badge badge-success";
  return "badge";
}

function stateText(t: (key: string) => string, state: string): string {
  if (state === "normal") return t("正常");
  if (state === "blacklisted") return t("已拉黑");
  return t("未知");
}

export function PlatformPolicyPanel({ platform }: Props) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<FormState>(() => toForm(platform));
  const [notice, setNotice] = useState<string>("");
  const [egressState, setEgressState] = useState<"all" | "unknown" | "normal" | "blacklisted">("all");
  const [egressSort, setEgressSort] = useState<"failure_desc" | "failure_asc" | "probe_desc" | "probe_asc" | "ip_asc">(
    "failure_desc",
  );

  useEffect(() => {
    setForm(toForm(platform));
  }, [platform]);

  const egressQuery = useQuery({
    queryKey: ["platform-egress", platform.id],
    queryFn: () => listPlatformEgress(platform.id),
    enabled: Boolean(platform.id),
    refetchInterval: 15000,
  });

  // ASN / IP-type dimensions only work when the backend has an ASN database.
  const geoipStatusQuery = useQuery({
    queryKey: ["geoip-status"],
    queryFn: getGeoIPStatus,
    staleTime: 60000,
  });
  const asnAvailable = geoipStatusQuery.data?.asn_available ?? false;

  const saveMutation = useMutation({
    mutationFn: async () => {
      const patch: PlatformUpdateInput = {
        probe_enabled: form.probe_enabled,
        probe_url: form.probe_url,
        probe_method: form.probe_method,
        probe_headers: form.probe_headers,
        probe_body: form.probe_body,
        probe_pass_status: splitStatuses(form.probe_pass_status),
        probe_follow_redirects: form.probe_follow_redirects,
        probe_interval: form.probe_interval,
        probe_scope: form.probe_scope,
        verdict_enabled: form.verdict_enabled,
        blacklist_threshold: form.blacklist_threshold,
        rehab_interval: form.rehab_interval,
        rehab_success_threshold: form.rehab_success_threshold,
        latency_top_n: form.latency_top_n,
        latency_max_ms: form.latency_max_ms,
        latency_p95_max_ms: form.latency_p95_max_ms,
        min_alive_streak: form.min_alive_streak,
        min_probe_success_rate: form.min_probe_success_rate,
        max_last_probe_age: form.max_last_probe_age,
        min_node_age: form.min_node_age,
        egress_ip_version: form.egress_ip_version,
        egress_country_filters: splitList(form.egress_country_filters),
        asn_filters: splitList(form.asn_filters),
        ip_type_filters: splitList(form.ip_type_filters),
        egress_ip_filters: splitList(form.egress_ip_filters),
        failover_enabled: form.failover_enabled,
        failover_max_switches: form.failover_max_switches,
        failover_deadline: form.failover_deadline,
        failover_stage: form.failover_stage,
      } as PlatformUpdateInput;
      return updatePlatform(platform.id, patch);
    },
    onSuccess: () => {
      setNotice(t("已保存"));
      void queryClient.invalidateQueries({ queryKey: ["platform", platform.id] });
      void queryClient.invalidateQueries({ queryKey: ["platform-egress", platform.id] });
    },
    onError: (error) => setNotice(formatApiErrorMessage(error, t)),
  });

  const num = (v: string) => {
    const n = Number.parseInt(v, 10);
    return Number.isFinite(n) ? n : 0;
  };
  const rate = (v: string) => {
    const n = Number.parseFloat(v);
    return Number.isFinite(n) ? n : 0;
  };
  const allEgressRows: EgressIPRow[] = egressQuery.data?.items ?? [];
  const visibleEgressRows = allEgressRows
    .filter((row) => egressState === "all" || (row.state || "unknown") === egressState)
    .slice()
    .sort((a, b) => {
      switch (egressSort) {
        case "failure_asc":
          return a.failure_count - b.failure_count || a.ip.localeCompare(b.ip);
        case "probe_asc":
          return (a.last_probe_at || "").localeCompare(b.last_probe_at || "") || a.ip.localeCompare(b.ip);
        case "probe_desc":
          return (b.last_probe_at || "").localeCompare(a.last_probe_at || "") || a.ip.localeCompare(b.ip);
        case "ip_asc":
          return a.ip.localeCompare(b.ip);
        default:
          return b.failure_count - a.failure_count || a.ip.localeCompare(b.ip);
      }
    });

  return (
    <div className="platform-detail-tabpanel platform-ops-tabpanel">
      {notice ? (
        <p className="muted" style={{ marginTop: 0, fontSize: 12, lineHeight: 1.4 }}>{notice}</p>
      ) : null}

      <section className="syscfg-section">
        <h4>{t("探测配置")}</h4>
        <p className="muted">{t("用该平台自己的请求模板探测出口 IP，结果归属该 IP。")}</p>

        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
            gap: "16px 24px",
            marginTop: 16,
          }}
        >
          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("应用探测")}</label>
            <div style={{ display: "flex", alignItems: "center", height: 36 }}>
              <Switch
                checked={form.probe_enabled}
                onChange={(e) => setForm({ ...form, probe_enabled: e.target.checked })}
              />
            </div>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("关闭时不产生任何探测样本")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("探测方法")}</label>
            <Select
              value={form.probe_method}
              onChange={(e) => setForm({ ...form, probe_method: e.target.value })}
            >
              {["POST", "GET", "PUT", "PATCH", "DELETE", "HEAD"].map((m) => (
                <option key={m} value={m}>{m}</option>
              ))}
            </Select>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 POST")}</p>
          </div>

          <div style={{ gridColumn: "1 / -1" }}>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("探测 URL")}</label>
            <Input value={form.probe_url} onChange={(e) => setForm({ ...form, probe_url: e.target.value })} />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("任意 URL，示例为 opencode 匿名请求地址")}</p>
          </div>

          <div style={{ gridColumn: "1 / -1" }}>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("请求头")}</label>
            <Textarea
              rows={4}
              value={form.probe_headers}
              onChange={(e) => setForm({ ...form, probe_headers: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("每行一条：名称: 值")}</p>
          </div>

          <div style={{ gridColumn: "1 / -1" }}>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("请求体")}</label>
            <Textarea
              rows={5}
              value={form.probe_body}
              onChange={(e) => setForm({ ...form, probe_body: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("请求体模板，可自由改写")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("通过状态码")}</label>
            <Input
              value={form.probe_pass_status}
              onChange={(e) => setForm({ ...form, probe_pass_status: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("逗号分隔，默认 200；其余一律计失败")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("跟随重定向")}</label>
            <div style={{ display: "flex", alignItems: "center", height: 36 }}>
              <Switch
                checked={form.probe_follow_redirects}
                onChange={(e) => setForm({ ...form, probe_follow_redirects: e.target.checked })}
              />
            </div>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认开启")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("探测间隔")}</label>
            <Input value={form.probe_interval} onChange={(e) => setForm({ ...form, probe_interval: e.target.value })} />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 30m，勿过勤")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("探测范围")}</label>
            <Select value={form.probe_scope} onChange={(e) => setForm({ ...form, probe_scope: e.target.value })}>
              <option value="untested,failed,blacklisted">{t("未探测 / 失败过 / 黑名单中")}</option>
              <option value="untested">{t("仅未探测")}</option>
              <option value="failed">{t("仅失败过")}</option>
              <option value="blacklisted">{t("仅黑名单")}</option>
            </Select>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("绝不整池扫")}</p>
          </div>
        </div>
      </section>

      <section className="syscfg-section">
        <h4>{t("判决与黑名单")}</h4>
        <p className="muted">{t("连续失败即拉黑该出口 IP，并在本平台路由中剔除。")}</p>

        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
            gap: "16px 24px",
            marginTop: 16,
          }}
        >
          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("应用判决")}</label>
            <div style={{ display: "flex", alignItems: "center", height: 36 }}>
              <Switch
                checked={form.verdict_enabled}
                onChange={(e) => setForm({ ...form, verdict_enabled: e.target.checked })}
              />
            </div>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("关闭时只记录台账，不剔除路由")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("连续失败拉黑阈值")}</label>
            <Input
              type="number"
              min={1}
              value={String(form.blacklist_threshold)}
              onChange={(e) => setForm({ ...form, blacklist_threshold: num(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 3 次")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("黑名单复探间隔")}</label>
            <Input value={form.rehab_interval} onChange={(e) => setForm({ ...form, rehab_interval: e.target.value })} />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 6h")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("洗白所需连续通过")}</label>
            <Input
              type="number"
              min={1}
              value={String(form.rehab_success_threshold)}
              onChange={(e) => setForm({ ...form, rehab_success_threshold: num(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 3 次，中间失败即重计")}</p>
          </div>
        </div>
      </section>

      <section className="syscfg-section">
        <h4>{t("筛选增强")}</h4>
        <p className="muted">{t("全部 AND 叠加；不填即不限制。")}</p>

        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
            gap: "16px 24px",
            marginTop: 16,
          }}
        >
          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("延迟 Top N")}</label>
            <Input
              type="number"
              min={0}
              value={String(form.latency_top_n)}
              onChange={(e) => setForm({ ...form, latency_top_n: num(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("0=不限")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("延迟上限 ms")}</label>
            <Input
              type="number"
              min={0}
              value={String(form.latency_max_ms)}
              onChange={(e) => setForm({ ...form, latency_max_ms: num(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("0=不限")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("P95 上限 ms")}</label>
            <Input
              type="number"
              min={0}
              value={String(form.latency_p95_max_ms)}
              onChange={(e) => setForm({ ...form, latency_p95_max_ms: num(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("0=不限；无样本时不生效")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("最小存活连击")}</label>
            <Input
              type="number"
              min={0}
              value={String(form.min_alive_streak)}
              onChange={(e) => setForm({ ...form, min_alive_streak: num(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("0=不限；未探测节点不受限")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("近 24 小时探测通过率")}</label>
            <Input
              type="number"
              min={0}
              max={1}
              step={0.05}
              value={String(form.min_probe_success_rate)}
              onChange={(e) => setForm({ ...form, min_probe_success_rate: rate(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("按小时桶滚动，无样本时不限制；0=不限")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("最近成功探测最大年龄")}</label>
            <Input
              value={form.max_last_probe_age}
              onChange={(e) => setForm({ ...form, max_last_probe_age: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("如 2h；0s=不限")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("最短入池时长")}</label>
            <Input value={form.min_node_age} onChange={(e) => setForm({ ...form, min_node_age: e.target.value })} />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("如 10m；0s=不限")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("出口 IP 版本")}</label>
            <Select
              value={form.egress_ip_version}
              onChange={(e) => setForm({ ...form, egress_ip_version: e.target.value })}
            >
              <option value="">{t("不限")}</option>
              <option value="v4">IPv4</option>
              <option value="v6">IPv6</option>
            </Select>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("按实测出口 IP 判定")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("IP 类型")}</label>
            <Select
              value={form.ip_type_filters}
              onChange={(e) => setForm({ ...form, ip_type_filters: e.target.value })}
            >
              <option value="">{t("不限")}</option>
              <option value="residential">{t("住宅")}</option>
              <option value="hosting">{t("机房")}</option>
              <option value="mobile">{t("移动")}</option>
              <option value="unknown">{t("其他")}</option>
              <option value="!hosting">{t("非机房")}</option>
              <option value="!residential">{t("非住宅")}</option>
              <option value="!unknown">{t("非其他")}</option>
            </Select>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("按 ASN 组织名启发式判定")}</p>
            {!asnAvailable && (
              <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>
                {t("需 ASN 库；当前未安装，此维度不生效")}
              </p>
            )}
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("ASN")}</label>
            <Input value={form.asn_filters} onChange={(e) => setForm({ ...form, asn_filters: e.target.value })} />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("如 as123,!as456；需 ASN 库")}</p>
            {!asnAvailable && (
              <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>
                {t("需 ASN 库；当前未安装，此维度不生效")}
              </p>
            )}
          </div>

          <div style={{ gridColumn: "1 / -1" }}>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("出口国家")}</label>
            <Input
              style={{ width: "100%" }}
              value={form.egress_country_filters}
              onChange={(e) => setForm({ ...form, egress_country_filters: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("逗号分隔，如 jp,us；! 前缀排除")}</p>
          </div>

          <div style={{ gridColumn: "1 / -1" }}>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("手动 IP 黑白")}</label>
            <Input
              value={form.egress_ip_filters}
              onChange={(e) => setForm({ ...form, egress_ip_filters: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("IP 或 CIDR，逗号分隔；! 前缀排除")}</p>
          </div>
        </div>

        {!form.probe_enabled &&
        (form.latency_p95_max_ms > 0 ||
          form.min_alive_streak > 0 ||
          form.min_probe_success_rate > 0 ||
          (form.max_last_probe_age !== "" &&
            form.max_last_probe_age !== "0s" &&
            form.max_last_probe_age !== "0")) ? (
          <p className="muted" style={{ marginTop: 12 }}>
            {t("未开启探测，这些维度不会生效")}
          </p>
        ) : null}
      </section>

      <section className="syscfg-section">
        <h4>{t("换路设置")}</h4>
        <p className="muted">{t("仅连接阶段失败时换节点；已建立的传输不重试。")}</p>

        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
            gap: "16px 24px",
            marginTop: 16,
          }}
        >
          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("应用换路")}</label>
            <div style={{ display: "flex", alignItems: "center", height: 36 }}>
              <Switch
                checked={form.failover_enabled}
                onChange={(e) => setForm({ ...form, failover_enabled: e.target.checked })}
              />
            </div>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("关闭时拨号失败直接返回错误")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("最大换路次数")}</label>
            <Input
              type="number"
              min={1}
              max={10}
              value={String(form.failover_max_switches)}
              onChange={(e) => setForm({ ...form, failover_max_switches: num(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 3，范围 1–10")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("总截止时间")}</label>
            <Input
              value={form.failover_deadline}
              onChange={(e) => setForm({ ...form, failover_deadline: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 15s，范围 1s–1m")}</p>
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("换路阶段")}</label>
            <Select
              value={form.failover_stage}
              onChange={(e) => setForm({ ...form, failover_stage: e.target.value })}
            >
              <option value="connect">{t("连接阶段")}</option>
              <option value="first_byte" disabled>{t("首字节（未实现）")}</option>
              <option value="any" disabled>{t("任意阶段（未实现）")}</option>
            </Select>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("当前仅实现 connect")}</p>
          </div>
        </div>
      </section>

      <section className="syscfg-section">
        <h4>{t("出口 IP 台账")}</h4>
        <p className="muted">{t("每个出口 IP 的状态、失败计数与洗白进度。")}</p>

        <div className="policy-toolbar" style={{ display: "flex", gap: 16, alignItems: "flex-end", flexWrap: "wrap", marginTop: 12 }}>
          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("状态")}</label>
            <Select value={egressState} onChange={(e) => setEgressState(e.target.value as typeof egressState)}>
              <option value="all">{t("全部")}</option>
              <option value="unknown">{t("未知")}</option>
              <option value="normal">{t("正常")}</option>
              <option value="blacklisted">{t("已拉黑")}</option>
            </Select>
          </div>
          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("排序")}</label>
            <Select value={egressSort} onChange={(e) => setEgressSort(e.target.value as typeof egressSort)}>
              <option value="failure_desc">{t("连续失败 ↓")}</option>
              <option value="failure_asc">{t("连续失败 ↑")}</option>
              <option value="probe_desc">{t("最近探测时间 ↓")}</option>
              <option value="probe_asc">{t("最近探测时间 ↑")}</option>
              <option value="ip_asc">{t("IP 字典序")}</option>
            </Select>
          </div>
          <Button variant="secondary" size="sm" onClick={() => void egressQuery.refetch()} disabled={egressQuery.isFetching}>
            {t("刷新")}
          </Button>
          <span className="muted">{visibleEgressRows.length} / {allEgressRows.length}</span>
        </div>

        {egressQuery.isLoading ? <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("加载中...")}</p> : null}
        {egressQuery.isError ? <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{formatApiErrorMessage(egressQuery.error, t)}</p> : null}
        {!egressQuery.isLoading && !egressQuery.isError && !allEgressRows.length ? (
          <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("暂无台账数据")}</p>
        ) : null}
        {visibleEgressRows.length ? (
          <div className="table-wrap" style={{ marginTop: 12 }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>{t("IP")}</th>
                  <th>{t("状态")}</th>
                  <th>{t("连续失败")}</th>
                  <th>{t("洗白进度")}</th>
                  <th>{t("最近结果码")}</th>
                  <th>{t("最近探测时间")}</th>
                  <th>{t("节点数")}</th>
                </tr>
              </thead>
              <tbody>
                {visibleEgressRows.map((row: EgressIPRow) => (
                  <tr key={row.ip}>
                    <td>{row.ip}</td>
                    <td>
                      <span className={stateBadge(row.state)}>{stateText(t, row.state || "unknown")}</span>
                    </td>
                    <td>{row.failure_count}</td>
                    <td>{row.success_streak}/{form.rehab_success_threshold}</td>
                    <td>{row.last_result || "-"}</td>
                    <td>{row.last_probe_at || "-"}</td>
                    <td>{row.node_count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

      <div
        style={{
          position: "sticky",
          bottom: 16,
          display: "flex",
          justifyContent: "flex-end",
          gap: 12,
        }}
      >
        <Button variant="primary" onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
          {saveMutation.isPending ? t("保存中...") : t("保存探测策略")}
        </Button>
      </div>
    </div>
  );
}
