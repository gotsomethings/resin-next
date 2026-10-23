import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Button } from "../../components/ui/Button";
import { Select } from "../../components/ui/Select";
import { Switch } from "../../components/ui/Switch";
import { Textarea } from "../../components/ui/Textarea";
import { Input } from "../../components/ui/Input";
import { useI18n } from "../../i18n";
import { formatApiErrorMessage } from "../../lib/error-message";
import { listPlatformEgress, setPlatformEgressBlacklist, updatePlatform } from "./api";
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
  rehab_enabled: boolean;
  blacklist_threshold: number;
  rehab_interval: string;
  rehab_success_threshold: number;
  latency_top_n: number;
  latency_max_ms: number;
  latency_p95_max_ms: number;
  min_alive_streak: number;
  min_probe_success_rate: number;
  probe_rate_window_minutes: number;
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
    probe_interval: positiveDurationOrDefault(p.probe_interval, "30m0s"),
    probe_scope: p.probe_scope ?? "untested,failed,blacklisted",
    verdict_enabled: p.verdict_enabled ?? false,
    rehab_enabled: p.rehab_enabled ?? false,
    blacklist_threshold: atLeast(p.blacklist_threshold, 1, 3),
    rehab_interval: positiveDurationOrDefault(p.rehab_interval, "6h0m0s"),
    rehab_success_threshold: atLeast(p.rehab_success_threshold, 1, 3),
    latency_top_n: atLeast(p.latency_top_n, 0, 0),
    latency_max_ms: atLeast(p.latency_max_ms, 0, 0),
    latency_p95_max_ms: atLeast(p.latency_p95_max_ms, 0, 0),
    min_alive_streak: atLeast(p.min_alive_streak, 0, 0),
    min_probe_success_rate: withinRange(p.min_probe_success_rate, 0, 1, 0),
    probe_rate_window_minutes: durationMinutesOrDefault(p.probe_rate_window, 1440),
    max_last_probe_age: p.max_last_probe_age ?? "0s",
    min_node_age: p.min_node_age ?? "0s",
    egress_ip_version: p.egress_ip_version ?? "",
    egress_country_filters: (p.egress_country_filters ?? []).join(","),
    asn_filters: (p.asn_filters ?? []).join(","),
    ip_type_filters: (p.ip_type_filters ?? []).join(","),
    egress_ip_filters: (p.egress_ip_filters ?? []).join(","),
    failover_enabled: p.failover_enabled ?? false,
    failover_max_switches: withinRange(p.failover_max_switches, 1, 10, 3),
    failover_deadline: positiveDurationOrDefault(p.failover_deadline, "15s"),
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

// parseGoDuration parses a Go-style duration ("30m", "6h", "1h30m", "15s")
// into milliseconds, or null when the text is not a valid duration.
function parseGoDuration(raw: string): number | null {
  const text = raw.trim();
  if (!text) return null;
  if (text === "0") return 0;
  const re = /(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/g;
  let total = 0;
  let consumed = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    if (m.index !== consumed) return null;
    consumed = m.index + m[0].length;
    const v = Number.parseFloat(m[1]);
    const unit = m[2];
    const factor =
      unit === "h" ? 3_600_000 :
      unit === "m" ? 60_000 :
      unit === "s" ? 1_000 :
      unit === "ms" ? 1 :
      unit === "us" || unit === "µs" ? 0.001 : 0.000001;
    total += v * factor;
  }
  return consumed === text.length ? total : null;
}

// A constrained number below its minimum is almost always "unset" (0). Render
// the documented default instead of an invalid value the backend will reject.
function atLeast(v: number | undefined, min: number, fallback: number): number {
  if (typeof v !== "number" || !Number.isFinite(v) || v < min) return fallback;
  return v;
}

function withinRange(v: number | undefined, min: number, max: number, fallback: number): number {
  if (typeof v !== "number" || !Number.isFinite(v)) return fallback;
  const n = Math.trunc(v);
  if (n < min || n > max) return fallback;
  return n;
}

function durationMinutesOrDefault(v: string | undefined, fallback: number): number {
  if (!v) return fallback;
  const ms = parseGoDuration(v);
  if (ms === null || ms <= 0) return fallback;
  const minutes = Math.round(ms / 60000);
  return minutes < 10 ? fallback : minutes;
}

function positiveDurationOrDefault(v: string | undefined, fallback: string): string {
  if (!v) return fallback;
  const ms = parseGoDuration(v);
  if (ms === null || ms <= 0) return fallback;
  return v;
}

// Field-level error, rendered directly under the control it belongs to.
function FieldError({ message }: { message?: string }) {
  if (!message) return null;
  return <p style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4, color: "var(--danger)" }}>{message}</p>;
}

// Turn a backend "<field>: <msg>" validation error into a field key plus a
// localized message, so it can be shown under that field.
function mapApiFieldError(
  error: unknown,
  t: (text: string, options?: Record<string, unknown>) => string,
): { field?: string; message: string } {
  const raw = error instanceof Error ? error.message : "";
  const m = /^([a-z_]+)\[\d+\]\s*:\s*(.*)$/i.exec(raw) ?? /^([a-z_]+)\s*:\s*(.*)$/i.exec(raw);
  if (!m) {
    return { message: formatApiErrorMessage(error, t) };
  }
  return { field: m[1], message: t(m[2]) };
}

function stateBadge(state: string): string {
  if (state === "blacklisted") return "badge badge-danger";
  if (state === "failed") return "badge badge-warning";
  if (state === "normal") return "badge badge-success";
  return "badge";
}

function stateText(t: (key: string) => string, state: string): string {
  if (state === "normal") return t("正常");
  if (state === "failed") return t("失败");
  if (state === "blacklisted") return t("拉黑");
  return t("未测");
}

// RFC3339Nano -> local "YYYY-MM-DD HH:mm:ss" (- when unknown).
function formatTimeToSecond(raw: string | undefined): string {
  if (!raw) return "-";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

function formatRate(rate: number | null | undefined): string {
  if (typeof rate !== "number" || !Number.isFinite(rate)) return "-";
  return `${(rate * 100).toFixed(1)}%`;
}

function ipTypeText(t: (key: string) => string, type: string | undefined): string {
  if (type === "residential") return t("住宅");
  if (type === "hosting") return t("机房");
  if (type === "mobile") return t("移动");
  return t("其他");
}

const IP_FILTER_ITEM_STYLE = { display: "flex", flexDirection: "column" as const, gap: "0.25rem" };
const IP_FILTER_LABEL_STYLE = { fontSize: "0.75rem", color: "var(--text-secondary)" };
const IP_FILTER_CONTROL_STYLE = { minWidth: 150 };

// Manual blacklist toggle: off = neutral track with the knob left, on = the
// knob slides right over a red track.
function BlacklistToggle({
  on,
  disabled,
  onToggle,
}: {
  on: boolean;
  disabled?: boolean;
  onToggle: (next: boolean) => void;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      disabled={disabled}
      onClick={() => onToggle(!on)}
      style={{
        position: "relative",
        width: 44,
        height: 24,
        padding: 0,
        border: "none",
        borderRadius: 12,
        cursor: disabled ? "not-allowed" : "pointer",
        background: on ? "var(--danger)" : "var(--border)",
        transition: "background 0.18s ease",
        opacity: disabled ? 0.6 : 1,
      }}
    >
      <span
        style={{
          position: "absolute",
          top: 3,
          left: on ? 23 : 3,
          width: 18,
          height: 18,
          borderRadius: "50%",
          background: "#fff",
          transition: "left 0.18s ease",
        }}
      />
    </button>
  );
}

export function PlatformPolicyPanel({ platform }: Props) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const [form, setForm] = useState<FormState>(() => toForm(platform));
  const [notice, setNotice] = useState<string>("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [egressState, setEgressState] = useState<"all" | "unknown" | "normal" | "failed" | "blacklisted">(
    "all",
  );
  const [egressType, setEgressType] = useState<"all" | "residential" | "hosting" | "mobile" | "unknown">("all");
  const [egressKeyword, setEgressKeyword] = useState("");
  const [egressCountry, setEgressCountry] = useState("");
  const [egressSort, setEgressSort] = useState<"failure_desc" | "failure_asc" | "probe_desc" | "probe_asc" | "ip_asc">(
    "ip_asc",
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
        rehab_enabled: form.rehab_enabled,
        blacklist_threshold: form.blacklist_threshold,
        rehab_interval: form.rehab_interval,
        rehab_success_threshold: form.rehab_success_threshold,
        latency_top_n: form.latency_top_n,
        latency_max_ms: form.latency_max_ms,
        latency_p95_max_ms: form.latency_p95_max_ms,
        min_alive_streak: form.min_alive_streak,
        min_probe_success_rate: form.min_probe_success_rate,
        probe_rate_window: `${form.probe_rate_window_minutes}m`,
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
    onError: (error) => {
      const mapped = mapApiFieldError(error, t);
      if (mapped.field) {
        setErrors((prev) => ({ ...prev, [mapped.field as string]: mapped.message }));
        setNotice("");
        return;
      }
      setNotice(mapped.message);
    },
  });

  const blacklistMutation = useMutation({
    mutationFn: (vars: { ip: string; blacklisted: boolean }) =>
      setPlatformEgressBlacklist(platform.id, vars.ip, vars.blacklisted),
    onSuccess: () => {
      setNotice("");
      void queryClient.invalidateQueries({ queryKey: ["platform-egress", platform.id] });
      void queryClient.invalidateQueries({ queryKey: ["platform", platform.id] });
    },
    onError: (error) => {
      setNotice(formatApiErrorMessage(error, t));
    },
  });

  const num = (v: string) => {
    const n = Number.parseInt(v, 10);
    return Number.isFinite(n) ? n : 0;
  };
  const rate = (v: string) => {
    const n = Number.parseFloat(v);
    return Number.isFinite(n) ? n : 0;
  };

  // Client-side mirror of the backend constraints: catching them here keeps the
  // message localized and puts it under the offending field.
  const validateForm = (f: FormState): Record<string, string> => {
    const out: Record<string, string> = {};
    const intMin = (key: string, value: number, min: number) => {
      if (!Number.isFinite(value) || value < min) {
        out[key] = t("必须是 ≥ {{min}} 的整数", { min });
      }
    };
    const dur = (key: string, raw: string, minMs: number) => {
      const ms = parseGoDuration(raw);
      if (ms === null) {
        out[key] = t("必须填写有效时长，如 30m、6h、15s");
      } else if (ms < minMs) {
        out[key] = t("必须大于 0");
      }
    };

    intMin("blacklist_threshold", f.blacklist_threshold, 1);
    if (f.rehab_enabled) {
      intMin("rehab_success_threshold", f.rehab_success_threshold, 1);
    }
    intMin("latency_top_n", f.latency_top_n, 0);
    intMin("latency_max_ms", f.latency_max_ms, 0);
    intMin("latency_p95_max_ms", f.latency_p95_max_ms, 0);
    intMin("min_alive_streak", f.min_alive_streak, 0);

    if (f.failover_max_switches < 1 || f.failover_max_switches > 10) {
      out.failover_max_switches = t("必须是 {{min}}–{{max}} 之间的整数", { min: 1, max: 10 });
    }
    if (f.probe_rate_window_minutes < 10) {
      out.probe_rate_window_minutes = t("必须是 ≥ {{min}} 的整数", { min: 10 });
    }
    if (!(f.min_probe_success_rate >= 0 && f.min_probe_success_rate <= 1)) {
      out.min_probe_success_rate = t("必须是 {{min}}–{{max}} 之间的数值", { min: 0, max: 1 });
    }

    dur("probe_interval", f.probe_interval, 1);
    if (f.rehab_enabled) {
      dur("rehab_interval", f.rehab_interval, 1);
    }
    dur("failover_deadline", f.failover_deadline, 1);
    dur("max_last_probe_age", f.max_last_probe_age, 0);
    dur("min_node_age", f.min_node_age, 0);

    const codes = splitList(f.probe_pass_status).map((v) => Number.parseInt(v, 10));
    if (!codes.length || codes.some((n) => !Number.isFinite(n) || n < 100 || n > 599)) {
      out.probe_pass_status = t("必须填写有效状态码（100–599），逗号分隔");
    }
    return out;
  };
  const allEgressRows: EgressIPRow[] = egressQuery.data?.items ?? [];
  const visibleEgressRows = allEgressRows
    .filter((row) => egressState === "all" || (row.state || "unknown") === egressState)
    .filter((row) => egressType === "all" || (row.ip_type || "unknown") === egressType)
    .filter((row) => {
      const kw = egressKeyword.trim().toLowerCase();
      return !kw || row.ip.toLowerCase().includes(kw);
    })
    .filter((row) => {
      const want = egressCountry.trim().toLowerCase();
      return !want || (row.country || "").toLowerCase() === want;
    })
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
            <FieldError message={errors.probe_pass_status} />
          </div>


          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("探测间隔")}</label>
            <Input value={form.probe_interval} onChange={(e) => setForm({ ...form, probe_interval: e.target.value })} />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 30m，勿过勤")}</p>
            <FieldError message={errors.probe_interval} />
          </div>

        </div>

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
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("黑名单复用")}</label>
            <div style={{ display: "flex", alignItems: "center", height: 36 }}>
              <Switch
                checked={form.rehab_enabled}
                onChange={(e) => setForm({ ...form, rehab_enabled: e.target.checked })}
              />
            </div>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("关闭时不重探拉黑 IP")}</p>
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
            <FieldError message={errors.blacklist_threshold} />
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("黑名单复探间隔")}</label>
            <Input
              value={form.rehab_interval}
              disabled={!form.rehab_enabled}
              onChange={(e) => setForm({ ...form, rehab_interval: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 6h")}</p>
            <FieldError message={errors.rehab_interval} />
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("复用连续通过探测次数")}</label>
            <Input
              type="number"
              min={1}
              value={String(form.rehab_success_threshold)}
              disabled={!form.rehab_enabled}
              onChange={(e) => setForm({ ...form, rehab_success_threshold: num(e.target.value) })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 3 次，中间失败即重计")}</p>
            <FieldError message={errors.rehab_success_threshold} />
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
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("探测通过率")}</label>
            <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
              <span className="muted" style={{ fontSize: 12 }}>{t("近")}</span>
              <Input
                type="number"
                min={10}
                style={{ width: 96 }}
                value={String(form.probe_rate_window_minutes)}
                onChange={(e) => setForm({ ...form, probe_rate_window_minutes: num(e.target.value) })}
              />
              <span className="muted" style={{ fontSize: 12 }}>{t("分钟通过率大于")}</span>
              <Input
                type="number"
                min={0}
                max={1}
                step={0.05}
                style={{ width: 96 }}
                value={String(form.min_probe_success_rate)}
                onChange={(e) => setForm({ ...form, min_probe_success_rate: rate(e.target.value) })}
              />
            </div>
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("窗口最小 10 分钟；通过率 0–1，0=不限")}</p>
            <FieldError message={errors.probe_rate_window_minutes ?? errors.min_probe_success_rate} />
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
        <h4>{t("高级路由设置")}</h4>
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
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("应用高级路由")}</label>
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
            <FieldError message={errors.failover_max_switches} />
          </div>

          <div>
            <label className="field-label" style={{ display: "block", marginBottom: 6 }}>{t("总截止时间")}</label>
            <Input
              value={form.failover_deadline}
              onChange={(e) => setForm({ ...form, failover_deadline: e.target.value })}
            />
            <p className="muted" style={{ marginTop: 4, fontSize: 12, lineHeight: 1.4 }}>{t("默认 15s，范围 1s–1m")}</p>
            <FieldError message={errors.failover_deadline} />
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
        <h4>{t("IP 列表")}</h4>
        <p className="muted">
          {t("展示平台配置与高级设置共同过滤后的全部出口 IP；未测与拉黑仍在列。")}
        </p>

        <div
          className="nodes-inline-filters"
          style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem", alignItems: "flex-end", marginTop: 12 }}
        >
          <div style={IP_FILTER_ITEM_STYLE}>
            <label style={IP_FILTER_LABEL_STYLE}>{t("IP")}</label>
            <Input
              value={egressKeyword}
              onChange={(e) => setEgressKeyword(e.target.value)}
              placeholder={t("模糊搜索")}
              style={IP_FILTER_CONTROL_STYLE}
            />
          </div>
          <div style={IP_FILTER_ITEM_STYLE}>
            <label style={IP_FILTER_LABEL_STYLE}>{t("国家")}</label>
            <Input
              value={egressCountry}
              onChange={(e) => setEgressCountry(e.target.value)}
              placeholder={t("如 jp")}
              style={IP_FILTER_CONTROL_STYLE}
            />
          </div>
          <div style={IP_FILTER_ITEM_STYLE}>
            <label style={IP_FILTER_LABEL_STYLE}>{t("类型")}</label>
            <Select value={egressType} onChange={(e) => setEgressType(e.target.value as typeof egressType)}>
              <option value="all">{t("全部")}</option>
              <option value="residential">{t("住宅")}</option>
              <option value="hosting">{t("机房")}</option>
              <option value="mobile">{t("移动")}</option>
              <option value="unknown">{t("其他")}</option>
            </Select>
          </div>
          <div style={IP_FILTER_ITEM_STYLE}>
            <label style={IP_FILTER_LABEL_STYLE}>{t("状态")}</label>
            <Select value={egressState} onChange={(e) => setEgressState(e.target.value as typeof egressState)}>
              <option value="all">{t("全部")}</option>
              <option value="unknown">{t("未测")}</option>
              <option value="normal">{t("正常")}</option>
              <option value="failed">{t("失败")}</option>
              <option value="blacklisted">{t("拉黑")}</option>
            </Select>
          </div>
          <div style={IP_FILTER_ITEM_STYLE}>
            <label style={IP_FILTER_LABEL_STYLE}>{t("排序")}</label>
            <Select value={egressSort} onChange={(e) => setEgressSort(e.target.value as typeof egressSort)}>
              <option value="ip_asc">{t("IP 字典序")}</option>
              <option value="failure_desc">{t("连续失败 ↓")}</option>
              <option value="failure_asc">{t("连续失败 ↑")}</option>
              <option value="probe_desc">{t("最近探测时间 ↓")}</option>
              <option value="probe_asc">{t("最近探测时间 ↑")}</option>
            </Select>
          </div>
          <Button variant="secondary" size="sm" onClick={() => void egressQuery.refetch()} disabled={egressQuery.isFetching}>
            {t("刷新")}
          </Button>
          <span className="muted">{visibleEgressRows.length} / {allEgressRows.length}</span>
        </div>

        {egressQuery.isLoading ? <p className="muted" style={{ marginTop: 8, fontSize: 12 }}>{t("加载中...")}</p> : null}
        {egressQuery.isError ? (
          <p className="muted" style={{ marginTop: 8, fontSize: 12 }}>{formatApiErrorMessage(egressQuery.error, t)}</p>
        ) : null}
        {!egressQuery.isLoading && !egressQuery.isError && !allEgressRows.length ? (
          <p className="muted" style={{ marginTop: 8, fontSize: 12 }}>{t("暂无台账数据")}</p>
        ) : null}

        {visibleEgressRows.length ? (
          <div className="table-wrap" style={{ marginTop: 12 }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>{t("IP")}</th>
                  <th>{t("国家")}</th>
                  <th>{t("类型")}</th>
                  <th>{t("总节点")}</th>
                  <th>{t("状态")}</th>
                  <th>{t("黑名单连续成功")}</th>
                  <th>{t("状态码")}</th>
                  <th>{t("时间")}</th>
                  <th>{t("近 {{min}} 分钟通过率", { min: form.probe_rate_window_minutes })}</th>
                  <th>{t("手动拉黑")}</th>
                </tr>
              </thead>
              <tbody>
                {visibleEgressRows.map((row: EgressIPRow) => {
                  const blacklisted = row.state === "blacklisted" || Boolean(row.manual_blacklist);
                  return (
                    <tr key={row.ip}>
                      <td>{row.ip}</td>
                      <td>{row.country ? row.country.toUpperCase() : "-"}</td>
                      <td>{ipTypeText(t, row.ip_type)}</td>
                      <td>{row.node_count}</td>
                      <td>
                        <span className={stateBadge(row.state)}>{stateText(t, row.state || "unknown")}</span>
                      </td>
                      <td>{row.state === "blacklisted" ? row.success_streak : "-"}</td>
                      <td>{row.last_status_code > 0 ? row.last_status_code : "-"}</td>
                      <td style={{ whiteSpace: "nowrap", lineHeight: 1.5 }}>
                        <div>
                          <span className="muted" style={{ fontSize: 12 }}>{t("加入")} </span>
                          {formatTimeToSecond(row.joined_at)}
                        </div>
                        <div>
                          <span className="muted" style={{ fontSize: 12 }}>{t("探测")} </span>
                          {formatTimeToSecond(row.last_probe_at)}
                        </div>
                      </td>
                      <td>{formatRate(row.success_rate)}</td>
                      <td>
                        <BlacklistToggle
                          on={blacklisted}
                          disabled={blacklistMutation.isPending}
                          onToggle={(next) => blacklistMutation.mutate({ ip: row.ip, blacklisted: next })}
                        />
                      </td>
                    </tr>
                  );
                })}
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
        <Button
          variant="primary"
          onClick={() => {
            const next = validateForm(form);
            setErrors(next);
            if (Object.keys(next).length > 0) {
              setNotice("");
              return;
            }
            saveMutation.mutate();
          }}
          disabled={saveMutation.isPending}
        >
          {saveMutation.isPending ? t("保存中...") : t("保存探测策略")}
        </Button>
      </div>
    </div>
  );
}
