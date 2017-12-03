// code is copy from https://github.com/xjdrew/kone, Thanks xjdrew
package dns

import "log"

type Rule struct {
	patterns []Pattern
	final    string
}

type RuleConfig struct {
	Pattern []string
	Final   string
}

func (rule *Rule) DirectDomain(domain string) {
	log.Printf("[rule] add direct domain: %s", domain)
	pattern := rule.patterns[0].(*DomainSuffixPattern)
	pattern.AddDomain(domain)
}

// match a proxy for target `val`
func (rule *Rule) Proxy(val interface{}) (bool, string) {
	for _, pattern := range rule.patterns {
		if pattern.Match(val) {
			proxy := pattern.Proxy()
			log.Printf("[rule] %v -> %s: proxy %q", val, pattern.Name(), proxy)
			return true, proxy
		}
	}
	log.Printf("[rule] %v -> final: proxy %q", val, rule.final)
	return false, rule.final
}

func NewRule(config RuleConfig, patterns map[string]*PatternConfig) *Rule {
	rule := new(Rule)
	rule.final = config.Final
	pattern := NewDomainSuffixPattern("__internal__", "", nil)
	rule.patterns = append(rule.patterns, pattern)
	for _, name := range config.Pattern {
		if patternConfig, ok := patterns[name]; ok {
			if pattern := CreatePattern(name, patternConfig); pattern != nil {
				rule.patterns = append(rule.patterns, pattern)
			}
		}
	}
	return rule
}
