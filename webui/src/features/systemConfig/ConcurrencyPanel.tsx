import { useQuery } from "@tanstack/react-query";
import { Badge } from "../../components/ui/Badge";
import { Card } from "../../components/ui/Card";
import { useI18n } from "../../i18n";
import { getSystemConfig } from "./api";

export function ConcurrencyPanel() {
  const { t } = useI18n();
  const query = useQuery({ queryKey: ["system-config"], queryFn: getSystemConfig, refetchInterval: 15000 });
  const cfg = query.data;

  return (
    <Card className="syscfg-form-card">
      <div className="detail-header">
        <div>
          <h3>{t("探测并发治理")}</h3>
          <p>{t("设定值与模式在运行时配置里修改；生效值由资源治理器实时调整。")}</p>
        </div>
      </div>

      {query.isError ? <p className="muted">{t("加载失败")}</p> : null}

      <div className="syscfg-grid">
        <div className="syscfg-field">
          <span className="muted">{t("设定值")}</span>
          <strong>{cfg?.probe_concurrency ?? "-"}</strong>
        </div>
        <div className="syscfg-field">
          <span className="muted">{t("当前生效值")}</span>
          <strong>{cfg?.probe_concurrency_effective ?? "-"}</strong>
        </div>
        <div className="syscfg-field">
          <span className="muted">{t("模式")}</span>
          {cfg?.probe_concurrency_mode ? (
            <Badge variant={cfg.probe_concurrency_mode === "auto" ? "warning" : "neutral"}>
              {cfg.probe_concurrency_mode}
            </Badge>
          ) : (
            <strong>-</strong>
          )}
        </div>
        <div className="syscfg-field">
          <span className="muted">{t("台账保留期")}</span>
          <strong>{cfg?.verdict_gc_retention ?? "-"}</strong>
        </div>
      </div>

      <div className="syscfg-field" style={{ marginTop: 12 }}>
        <span className="muted">{t("最近调整原因")}</span>
        <strong>{cfg?.probe_concurrency_reason || t("尚未调整")}</strong>
        {cfg?.probe_concurrency_adjusted_at ? (
          <span className="muted"> @ {cfg.probe_concurrency_adjusted_at}</span>
        ) : null}
      </div>
    </Card>
  );
}
