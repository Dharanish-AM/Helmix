export const ALERT_RULE_TO_ACTION: Record<string, string> = {
  "ready-pods-zero": "restart_pods",
  "p99-latency-high": "scale_pods",
  "error-rate-high": "rollback_deployment",
  "cpu-high": "scale_pods",
  "pod-restarts-high": "restart_pods",
};

export function suggestedActionForAlertRule(alertRule: string): string {
  const normalized = alertRule.trim();
  if (!normalized) {
    return "";
  }
  return ALERT_RULE_TO_ACTION[normalized] ?? "";
}

export function parseAlertRuleQuery(params: URLSearchParams): string {
  const direct = params.get("alert_rule")?.trim() ?? "";
  if (direct) {
    return direct;
  }
  return params.get("rule")?.trim() ?? "";
}
