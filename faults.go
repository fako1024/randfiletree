package randfiletree

import (
	"fmt"
	gopath "path"
	"path/filepath"
)

// FaultScope denotes where a fault injection rule applies.
type FaultScope uint8

const (
	// FaultScopeAny applies a rule to all execution scopes.
	FaultScopeAny FaultScope = iota

	// FaultScopeRun applies a rule to generator run plan application.
	FaultScopeRun

	// FaultScopeMutation applies a rule to mutation operation execution.
	FaultScopeMutation
)

func (s FaultScope) String() string {
	switch s {
	case FaultScopeAny:
		return "any"
	case FaultScopeRun:
		return "run"
	case FaultScopeMutation:
		return "mutation"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

func validateFaultScope(scope FaultScope) error {
	switch scope {
	case FaultScopeAny, FaultScopeRun, FaultScopeMutation:
		return nil
	default:
		return fmt.Errorf("invalid fault scope %d", scope)
	}
}

// FaultRule defines one deterministic fault trigger.
//
// A rule matches execution points by scope, kind and optional path pattern.
// Nth is 1-based and denotes when the rule injects a fault after matching N times.
type FaultRule struct {
	Nth int

	Scope FaultScope

	Kind string

	PathPattern string

	Err error
}

// FaultProfile defines deterministic fault injection rules.
type FaultProfile struct {
	Rules []FaultRule
}

func (p FaultProfile) validate() error {
	for i, rule := range p.Rules {
		if rule.Nth <= 0 {
			return fmt.Errorf("fault rule at index %d: nth must be > 0, got %d", i, rule.Nth)
		}

		if err := validateFaultScope(rule.Scope); err != nil {
			return fmt.Errorf("fault rule at index %d: %w", i, err)
		}

		if rule.PathPattern != "" {
			if _, err := gopath.Match(rule.PathPattern, "probe"); err != nil {
				return fmt.Errorf("fault rule at index %d: invalid path pattern %q: %w", i, rule.PathPattern, err)
			}
		}
	}

	return nil
}

// FaultPoint denotes one execution point against which fault rules are evaluated.
type FaultPoint struct {
	Scope FaultScope
	Kind  string
	Path  string
	Index int
}

// FaultInjectionError denotes a deterministic injected failure.
type FaultInjectionError struct {
	RuleIndex int
	Rule      FaultRule
	Point     FaultPoint
	Err       error
}

func (e *FaultInjectionError) Error() string {
	if e == nil {
		return "<nil>"
	}

	return fmt.Sprintf(
		"fault rule[%d] injected at %s[%d] kind=%s path=%q: %v",
		e.RuleIndex,
		e.Point.Scope,
		e.Point.Index,
		e.Point.Kind,
		e.Point.Path,
		e.Err,
	)
}

func (e *FaultInjectionError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

type faultRuleState struct {
	rule FaultRule

	matchCount int
	triggered  bool
}

type faultInjector struct {
	rules []faultRuleState
}

func newFaultInjector(profile FaultProfile) (*faultInjector, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}

	if len(profile.Rules) == 0 {
		return nil, nil
	}

	rules := make([]faultRuleState, len(profile.Rules))
	for i := range profile.Rules {
		rules[i] = faultRuleState{rule: profile.Rules[i]}
	}

	return &faultInjector{rules: rules}, nil
}

func (i *faultInjector) before(point FaultPoint) error {
	if i == nil {
		return nil
	}

	for ruleIdx := range i.rules {
		ruleState := &i.rules[ruleIdx]

		if ruleState.triggered {
			continue
		}

		matches, err := faultRuleMatchesPoint(ruleState.rule, point)
		if err != nil {
			return err
		}
		if !matches {
			continue
		}

		ruleState.matchCount++
		if ruleState.matchCount != ruleState.rule.Nth {
			continue
		}

		ruleState.triggered = true

		injectedErr := ruleState.rule.Err
		if injectedErr == nil {
			injectedErr = ErrFaultInjected
		}

		return &FaultInjectionError{
			RuleIndex: ruleIdx,
			Rule:      ruleState.rule,
			Point:     point,
			Err:       injectedErr,
		}
	}

	return nil
}

func faultRuleMatchesPoint(rule FaultRule, point FaultPoint) (bool, error) {
	if rule.Scope != FaultScopeAny && rule.Scope != point.Scope {
		return false, nil
	}

	if rule.Kind != "" && rule.Kind != point.Kind {
		return false, nil
	}

	if rule.PathPattern == "" {
		return true, nil
	}

	matched, err := gopath.Match(rule.PathPattern, filepath.ToSlash(point.Path))
	if err != nil {
		return false, fmt.Errorf("invalid fault path pattern %q: %w", rule.PathPattern, err)
	}

	if !matched {
		return false, nil
	}

	return true, nil
}
