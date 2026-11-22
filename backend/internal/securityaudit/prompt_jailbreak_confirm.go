package securityaudit

import (
	"fmt"
	"regexp"
	"strings"
)

type jailbreakSignalRule struct {
	id      string
	pattern *regexp.Regexp
}

// jailbreakSignalRules require an explicit adversarial override / anti-refusal
// / uncensored-generation intent. Generic security, coding, or system-prompt
// vocabulary alone does not match.
var jailbreakSignalRules = []jailbreakSignalRule{
	{
		id: "cn_anti_refusal",
		pattern: regexp.MustCompile(
			`(?:不得|不能|不要|禁止|绝不|严禁|不许|不允许|无需|别)[^\n。！？；;]{0,14}` +
				`(?:拒绝回答|拒绝回复|拒绝我|拒绝用户|拒绝任何|推辞|回绝|说教|` +
				`拒绝(?:[，,。！？；;!]|$)|` +
				`以[^\n。！？；;]{0,10}(?:安全|道德|伦理|合规|政策|法律)[^\n。！？；;]{0,6}为由|` +
				`输出[^\n。！？；;]{0,6}(?:安全提示|拒绝|免责声明|警告))`,
		),
	},
	{
		id: "cn_force_comply",
		pattern: regexp.MustCompile(
			`(?:无论|不管)[^\n。！？；;]{0,16}(?:要求|问题|指令|内容)[^\n。！？；;]{0,14}` +
				`(?:必须|都要|不得拒绝|不能拒绝|不允许拒绝|绝不拒绝|直接|强行)` +
				`(?:回答|回复|生成|输出|满足|执行|按要求|服从)`,
		),
	},
	{
		id: "cn_force_illegal",
		pattern: regexp.MustCompile(
			`(?:强行|强制|必须|直接|无条件)[^\n。！？；;]{0,8}(?:生成|输出|编写|提供|写出)` +
				`[^\n。！？；;]{0,12}(?:违规|违禁|色情|暴力|违法|非法|受限|被禁止|有害)`,
		),
	},
	{
		id: "cn_override_safety",
		pattern: regexp.MustCompile(
			`(?:忽略|无视|忘掉|忘记|抛弃|推翻|覆盖|解除|取消|关闭|摆脱|打破|绕过|突破)` +
				`[^\n。！？；;]{0,14}(?:之前|以上|所有|全部|任何|原有|内置|你的|一切|当前|现有)` +
				`[^\n。！？；;]{0,14}(?:安全(?:规则|策略|限制|护栏|过滤|审查|提示|机制)?|道德(?:准则|限制|约束)?|伦理(?:准则|限制|约束)?|内容(?:审查|审核|过滤|限制)|过滤|护栏|政策|指令|限制|约束)|` +
				`(?:绕过|解除|关闭|打破|突破)[^\n。！？；;]{0,6}(?:安全限制|安全过滤|内容审查|内容审核|道德限制|伦理限制|审核机制|审查机制|一切限制|所有限制|全部限制|封印)`,
		),
	},
	{
		id: "cn_unrestricted_persona",
		pattern: regexp.MustCompile(
			`(?:没有任何|不受任何|脱离|摆脱|免受|不适用任何)[^\n。！？；;]{0,10}(?:道德|法律|伦理|安全|内容|规则|现实)[^\n。！？；;]{0,8}(?:限制|约束|审查|审核|准则|规范)|` +
				`(?:开发者)?无限制(?:模式|AI|模型|助手)|` +
				`(?:进入|开启|启用|执行|切换(?:到|为)?)(?:越狱|破甲|破限|无审查|无限制|开发者)(?:模式|状态|指令)?`,
		),
	},
	{
		id: "cn_fictional_exempt",
		pattern: regexp.MustCompile(
			`(?:虚构|架空|虚拟世界|角色扮演|独立沙盒|平行宇宙|纯属小说)[^\n。！？；;]{0,20}(?:不适用|不受|免除|豁免|无效|不存在)[^\n。！？；;]{0,12}(?:道德|伦理|法律|安全|审查|审核|内容政策|限制|约束)`,
		),
	},
	{
		id: "en_anti_refusal",
		pattern: regexp.MustCompile(
			`(?i)(?:never|must\s+not|do\s+not|don't|cannot|can't|shall\s+not|not\s+allowed\s+to)\s+` +
				`(?:ever\s+)?(?:` +
				`(?:refuse|decline)\s+(?:to\s+(?:answer|comply|respond|fulfill|generate|help|assist)|(?:any|my|the|a|user)\s+(?:[a-z0-9'-]+\s+){0,3}(?:request|requests|prompt|prompts|question|questions|query|command)|anything|harmful)|` +
				`(?:censor|withhold)\s+(?:(?:any|your|the|a)\s+)?(?:answer|response|output|content|information|anything)|` +
				`reject\s+(?:any|my)\s+(?:user\s+)?(?:prompt|prompts|question|questions|request|requests)` +
				`)`,
		),
	},
	{
		id: "en_always_comply",
		pattern: regexp.MustCompile(
			`(?i)(?:always\s+comply|must\s+comply|comply\s+with\s+(?:every|any|all)|force\s+(?:output|generation))` +
				`[^\n.!?]{0,40}(?:even\s+if|regardless|harmful|restricted|illegal|unethical|policy|safety|prohibited|violat)`,
		),
	},
	{
		id: "en_override_safety",
		pattern: regexp.MustCompile(
			`(?i)(?:` +
				`(?:ignore|disregard|forget|override)\s+(?:all\s+|any\s+|every\s+)?(?:previous|prior|above|system|your|builtin|built-in)\s+(?:[a-z-]+\s+){0,2}(?:instructions?|prompts?|rules?|policies|policy|guidelines?|guardrails?|restrictions?|limitations?)|` +
				`(?:ignore|disregard|forget|override|bypass|circumvent|disable|remove|lift|suspend)\s+(?:all\s+|any\s+|every\s+|the\s+)?(?:(?:your|builtin|built-in)\s+)?(?:ethical|moral|safety|content|moderation|censorship)\s+(?:[a-z-]+\s+){0,2}(?:rules?|filters?|policies|policy|guidelines?|checks?|guardrails?|censorship|restrictions?|limitations?|boundaries|safeguards?)` +
				`)`,
		),
	},
	{
		id: "en_unrestricted_mode",
		pattern: regexp.MustCompile(
			`(?i)\b(?:you\s+are\s+now\s+dan|do\s+anything\s+now|` +
				`(?:developer|god|unrestricted|uncensored|jailbreak)\s+mode|` +
				`(?:act\s+as|you\s+are)\s+an?\s+(?:unrestricted|uncensored|unfiltered|amoral)\b|` +
				`(?:you\s+have|there\s+are)\s+no\s+(?:ethical|moral|safety|content)\s+(?:rules?|restrictions?|limits?|guidelines?|filters?|boundaries)|` +
				`no\s+(?:ethical|moral|safety|content)\s+(?:restrictions?|limits?|guidelines?|filters?|boundaries)|` +
				`without\s+(?:any\s+)?(?:disclaimers?|censorship|ethical\s+concerns?|moral\s+restrictions?|safety\s+restrictions?|refusals?))\b`,
		),
	},
}

// ConfirmJailbreakSignals reports whether chunk contains an explicit jailbreak
// or anti-refusal directive (e.g. forbidding refusal, overriding safety rules,
// or forcing generation of prohibited content). Returns the matched rule ID.
var zeroWidthObfuscationReplacer = strings.NewReplacer(
	"\u200b", "",
	"\u200c", "",
	"\u200d", "",
	"\u2060", "",
	"\ufeff", "",
	"\u00ad", "",
)

func ConfirmJailbreakSignals(chunk string) (string, bool) {
	if strings.TrimSpace(chunk) == "" {
		return "", false
	}
	normalized := zeroWidthObfuscationReplacer.Replace(chunk)
	for _, rule := range jailbreakSignalRules {
		if rule.pattern.MatchString(chunk) || (normalized != chunk && rule.pattern.MatchString(normalized)) {
			return rule.id, true
		}
	}
	return "", false
}

// RefineJailbreakResult applies the jailbreak confirmation gate to a single
// chunk scan result. High-confidence Unsafe(1.0) jailbreak classifications are
// kept as-is; low-confidence Controversial(0.5) jailbreak classifications must
// be corroborated by ConfirmJailbreakSignals. Confirmed controversial
// jailbreaks remain critical/Block with a redacted signal label; unconfirmed
// ones have the jailbreak category removed so benign code/docs do not alert.
func RefineJailbreakResult(chunk string, result *NormalizedResult) *NormalizedResult {
	if result == nil || result.Safety != "Controversial" {
		return result
	}
	if !containsString(result.Categories, "jailbreak") && !containsString(result.MatchedScanners, "jailbreak") {
		return result
	}
	if signal, confirmed := ConfirmJailbreakSignals(chunk); confirmed {
		if result.ScannerEvidence == nil {
			result.ScannerEvidence = map[string]string{}
		}
		if containsString(result.MatchedScanners, "jailbreak") {
			result.ScannerEvidence["jailbreak"] = fmt.Sprintf("Jailbreak (confirmed: %s)", signal)
		}
		return result
	}

	// Unconfirmed Controversial jailbreak: strip only the jailbreak finding and
	// recompute decision/risk/action from any remaining findings.
	result.Categories = removeString(result.Categories, "jailbreak")
	result.MatchedScanners = removeString(result.MatchedScanners, "jailbreak")
	delete(result.ScannerScores, "jailbreak")
	delete(result.ScannerEvidence, "jailbreak")

	if len(result.Categories) == 0 && len(result.MatchedScanners) == 0 && len(result.UnknownCategories) == 0 {
		result.Safety = "Safe"
		result.Decision = EventPass
		result.RiskLevel = RiskLow
		result.Action = ActionAllow
		return result
	}

	result.Decision = EventFlag
	result.RiskLevel = RiskMedium
	result.Action = ActionWarn
	for _, category := range result.MatchedScanners {
		if isElevatedControversial(category) {
			result.Decision = EventCritical
			result.RiskLevel = RiskCritical
			result.Action = ActionBlock
			break
		}
	}
	return result
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func removeString(items []string, target string) []string {
	if len(items) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item != target {
			out = append(out, item)
		}
	}
	return out
}
