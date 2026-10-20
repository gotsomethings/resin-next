import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "../../components/ui/Button";
import { Card } from "../../components/ui/Card";
import { Input } from "../../components/ui/Input";
import { useI18n } from "../../i18n";
import { formatApiErrorMessage } from "../../lib/error-message";
import { listPlatformEgress, updatePlatform } from "./api";
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

export function PlatformPolicyPanel({ platform }: Props) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<FormState>(() => toForm(platform));
  const [notice, setNotice] = useState<string>("");

  useEffect(() => {
    setForm(toForm(platform));
  }, [platform]);

  const egressQuery = useQuery({
    queryKey: ["platform-egress", platform.id],
    queryFn: () => listPlatformEgress(platform.id),
    enabled: Boolean(platform.id),
    refetchInterval: 15000,
  });

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

  return (
    <Card className="platform-policy-panel">
      <div className="card-header">
        <h3>{t("探测与策略")}</h3>
        <Button variant="primary" size="sm" onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
          {saveMutation.isPending ? t("保存中...") : t("保存策略")}
        </Button>
      </div>
      {notice ? <p className="muted">{notice}</p> : null}

      <section className="policy-section">
        <h4>{t("探测配置")}</h4>
        <label className="field">
          <span>{t("启用探测")}</span>
          <input
            type="checkbox"
            checked={form.probe_enabled}
            onChange={(e) => setForm({ ...form, probe_enabled: e.target.checked })}
          />
        </label>
        <label className="field">
          <span>{t("探测 URL")}</span>
          <Input value={form.probe_url} onChange={(e) => setForm({ ...form, probe_url: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("方法")}</span>
          <Input value={form.probe_method} onChange={(e) => setForm({ ...form, probe_method: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("请求头（一行一个）")}</span>
          <textarea
            value={form.probe_headers}
            onChange={(e) => setForm({ ...form, probe_headers: e.target.value })}
            rows={3}
          />
        </label>
        <label className="field">
          <span>{t("请求体")}</span>
          <textarea
            value={form.probe_body}
            onChange={(e) => setForm({ ...form, probe_body: e.target.value })}
            rows={4}
          />
        </label>
        <label className="field">
          <span>{t("通过状态码（逗号分隔）")}</span>
          <Input
            value={form.probe_pass_status}
            onChange={(e) => setForm({ ...form, probe_pass_status: e.target.value })}
          />
        </label>
        <label className="field">
          <span>{t("跟随重定向")}</span>
          <input
            type="checkbox"
            checked={form.probe_follow_redirects}
            onChange={(e) => setForm({ ...form, probe_follow_redirects: e.target.checked })}
          />
        </label>
        <label className="field">
          <span>{t("探测间隔")}</span>
          <Input value={form.probe_interval} onChange={(e) => setForm({ ...form, probe_interval: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("探测范围")}</span>
          <Input value={form.probe_scope} onChange={(e) => setForm({ ...form, probe_scope: e.target.value })} />
        </label>
      </section>

      <section className="policy-section">
        <h4>{t("判决与黑名单")}</h4>
        <label className="field">
          <span>{t("启用判决")}</span>
          <input
            type="checkbox"
            checked={form.verdict_enabled}
            onChange={(e) => setForm({ ...form, verdict_enabled: e.target.checked })}
          />
        </label>
        <label className="field">
          <span>{t("连续失败拉黑阈值")}</span>
          <Input
            value={String(form.blacklist_threshold)}
            onChange={(e) => setForm({ ...form, blacklist_threshold: num(e.target.value) })}
          />
        </label>
        <label className="field">
          <span>{t("黑名单复探间隔")}</span>
          <Input value={form.rehab_interval} onChange={(e) => setForm({ ...form, rehab_interval: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("洗白所需连续通过")}</span>
          <Input
            value={String(form.rehab_success_threshold)}
            onChange={(e) => setForm({ ...form, rehab_success_threshold: num(e.target.value) })}
          />
        </label>
      </section>

      <section className="policy-section">
        <h4>{t("筛选增强")}</h4>
        <label className="field">
          <span>{t("延迟 Top N（0=不限）")}</span>
          <Input value={String(form.latency_top_n)} onChange={(e) => setForm({ ...form, latency_top_n: num(e.target.value) })} />
        </label>
        <label className="field">
          <span>{t("延迟上限 ms（0=不限）")}</span>
          <Input value={String(form.latency_max_ms)} onChange={(e) => setForm({ ...form, latency_max_ms: num(e.target.value) })} />
        </label>
        <label className="field">
          <span>{t("P95 上限 ms（0=不限）")}</span>
          <Input value={String(form.latency_p95_max_ms)} onChange={(e) => setForm({ ...form, latency_p95_max_ms: num(e.target.value) })} />
        </label>
        <label className="field">
          <span>{t("最小存活连击（0=不限）")}</span>
          <Input value={String(form.min_alive_streak)} onChange={(e) => setForm({ ...form, min_alive_streak: num(e.target.value) })} />
        </label>
        <label className="field">
          <span>{t("24h 成功率下限（0–1）")}</span>
          <Input value={String(form.min_probe_success_rate)} onChange={(e) => setForm({ ...form, min_probe_success_rate: rate(e.target.value) })} />
        </label>
        <label className="field">
          <span>{t("最近成功探测最大年龄")}</span>
          <Input value={form.max_last_probe_age} onChange={(e) => setForm({ ...form, max_last_probe_age: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("最短入池时长")}</span>
          <Input value={form.min_node_age} onChange={(e) => setForm({ ...form, min_node_age: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("出口 IP 版本（v4/v6/空）")}</span>
          <Input value={form.egress_ip_version} onChange={(e) => setForm({ ...form, egress_ip_version: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("出口国家（逗号分隔，! 排除）")}</span>
          <Input value={form.egress_country_filters} onChange={(e) => setForm({ ...form, egress_country_filters: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("ASN（as123 / !as123）")}</span>
          <Input value={form.asn_filters} onChange={(e) => setForm({ ...form, asn_filters: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("IP 类型（residential/hosting）")}</span>
          <Input value={form.ip_type_filters} onChange={(e) => setForm({ ...form, ip_type_filters: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("手动 IP 黑白（IP/CIDR，! 排除）")}</span>
          <Input value={form.egress_ip_filters} onChange={(e) => setForm({ ...form, egress_ip_filters: e.target.value })} />
        </label>
      </section>

      <section className="policy-section">
        <h4>{t("换路设置")}</h4>
        <label className="field">
          <span>{t("启用换路")}</span>
          <input
            type="checkbox"
            checked={form.failover_enabled}
            onChange={(e) => setForm({ ...form, failover_enabled: e.target.checked })}
          />
        </label>
        <label className="field">
          <span>{t("最大换路次数")}</span>
          <Input
            value={String(form.failover_max_switches)}
            onChange={(e) => setForm({ ...form, failover_max_switches: num(e.target.value) })}
          />
        </label>
        <label className="field">
          <span>{t("总截止时间")}</span>
          <Input value={form.failover_deadline} onChange={(e) => setForm({ ...form, failover_deadline: e.target.value })} />
        </label>
        <label className="field">
          <span>{t("换路阶段（当前仅 connect）")}</span>
          <Input value={form.failover_stage} onChange={(e) => setForm({ ...form, failover_stage: e.target.value })} />
        </label>
      </section>

      <section className="policy-section">
        <h4>{t("出口 IP 台账")}</h4>
        {egressQuery.isLoading ? <p className="muted">{t("加载中...")}</p> : null}
        {egressQuery.isError ? (
          <p className="muted">{formatApiErrorMessage(egressQuery.error, t)}</p>
        ) : null}
        {!egressQuery.isLoading && !egressQuery.isError && !(egressQuery.data?.items ?? []).length ? (
          <p className="muted">{t("暂无台账数据")}</p>
        ) : null}
        {(egressQuery.data?.items ?? []).length ? (
          <div className="table-wrap">
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
                {(egressQuery.data?.items ?? []).map((row: EgressIPRow) => (
                  <tr key={row.ip}>
                    <td>{row.ip}</td>
                    <td>
                      <span className={stateBadge(row.state)}>{row.state || "unknown"}</span>
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
    </Card>
  );
}
