package randfiletree

import "fmt"

type executionContext struct {
	injector *faultInjector
}

func newExecutionContext(profile FaultProfile) (executionContext, error) {
	injector, err := newFaultInjector(profile)
	if err != nil {
		return executionContext{}, fmt.Errorf("invalid fault profile: %w", err)
	}

	return executionContext{injector: injector}, nil
}

func (ctx executionContext) before(scope FaultScope, index int, kind, path string) error {
	if ctx.injector == nil {
		return nil
	}

	return ctx.injector.before(FaultPoint{
		Scope: scope,
		Kind:  kind,
		Path:  path,
		Index: index,
	})
}
