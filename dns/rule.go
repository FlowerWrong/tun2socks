// code is copy from https://github.com/xjdrew/kone, Thanks xjdrew
package dns

import (
	"log"

	"github.com/FlowerWrong/tun2socks/configure"
)

type Rule struct {
	patterns []Pattern
	final    string
}

func (rule *Rule) DirectDomain(domain string) {
	pattern := rule.patterns[0].(*DomainSuffixPattern)
	pattern.AddDomain(domain)
}

// Proxy match a proxy for target `val`
func (rule *Rule) Proxy(val interface{}) (bool, string) {
	for _, pattern := range rule.patterns {
		if pattern.Match(val) {
			return true, pattern.Proxy()
		}
	}
	return false, rule.final
}

// Reload rule config
func (rule *Rule) Reload(config configure.RuleConfig, patterns map[string]*configure.PatternConfig) {
	log.Println("Rule hot reloaded")
	rule.patterns = rule.patterns[:0]
	rule.setUp(config, patterns)
}

func (rule *Rule) setUp(config configure.RuleConfig, patterns map[string]*configure.PatternConfig) {
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
}

func NewRule(config configure.RuleConfig, patterns map[string]*configure.PatternConfig) *Rule {
	rule := new(Rule)
	rule.setUp(config, patterns)
	return rule
}
