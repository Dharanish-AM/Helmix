import { describe, expect, it } from "vitest";

import { parseAlertRuleQuery, suggestedActionForAlertRule } from "./presets";

describe("suggestedActionForAlertRule", () => {
  it("maps known alert rules to expected actions", () => {
    expect(suggestedActionForAlertRule("ready-pods-zero")).toBe("restart_pods");
    expect(suggestedActionForAlertRule("p99-latency-high")).toBe("scale_pods");
    expect(suggestedActionForAlertRule("error-rate-high")).toBe("rollback_deployment");
  });

  it("returns empty string for unknown or blank rules", () => {
    expect(suggestedActionForAlertRule("unknown-rule")).toBe("");
    expect(suggestedActionForAlertRule("   ")).toBe("");
  });
});

describe("parseAlertRuleQuery", () => {
  it("prefers dedicated alert_rule query parameter", () => {
    const params = new URLSearchParams("alert_rule=ready-pods-zero&rule=error-rate-high");
    expect(parseAlertRuleQuery(params)).toBe("ready-pods-zero");
  });

  it("falls back to rule alias for external deep links", () => {
    const params = new URLSearchParams("rule=error-rate-high");
    expect(parseAlertRuleQuery(params)).toBe("error-rate-high");
  });

  it("returns empty string when no rule query is present", () => {
    const params = new URLSearchParams("alert_metric=error_rate_pct");
    expect(parseAlertRuleQuery(params)).toBe("");
  });
});
