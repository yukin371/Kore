package agent

import (
	"context"
	"fmt"
	"strings"
)

// AgentState represents the current state in the loop controller.
type AgentState int

const (
	StatePlanning AgentState = iota
	StateExecuting
	StateObserving
	StateReflecting
	StateDone
)

// LoopIteration stores one full ReAct cycle.
type LoopIteration struct {
	Index       int
	Input       string
	Plan        string
	Execution   string
	Observation string
	Reflection  string
	Done        bool
}

// LoopController orchestrates Plan -> Execute -> Observe -> Reflect.
type LoopController struct {
	Runner          ReActRunner
	MaxLoops        int
	State           AgentState
	Iterations      []*LoopIteration
	AllowIncomplete bool
}

// Run executes the state machine until completion or max loops.
func (lc *LoopController) Run(ctx context.Context, input string) error {
	if lc.Runner == nil {
		return fmt.Errorf("loop controller requires a ReActRunner")
	}

	maxLoops := lc.MaxLoops
	if maxLoops <= 0 {
		maxLoops = 10
	}

	current := input
	lc.State = StatePlanning

	for i := 0; i < maxLoops; i++ {
		iter := &LoopIteration{
			Index: i,
			Input: current,
		}

		plan, err := lc.Runner.Plan(ctx, current)
		if err != nil {
			return fmt.Errorf("plan failed: %w", err)
		}
		iter.Plan = plan
		lc.State = StateExecuting

		exec, err := lc.Runner.Execute(ctx, plan)
		if err != nil {
			return fmt.Errorf("execute failed: %w", err)
		}
		iter.Execution = exec
		lc.State = StateObserving

		observation, err := lc.Runner.Observe(ctx, exec)
		if err != nil {
			return fmt.Errorf("observe failed: %w", err)
		}
		iter.Observation = observation
		lc.State = StateReflecting

		reflection, err := lc.Runner.Reflect(ctx, observation)
		if err != nil {
			return fmt.Errorf("reflect failed: %w", err)
		}
		iter.Reflection = reflection

		if isLoopDone(reflection) {
			iter.Done = true
			lc.State = StateDone
			lc.Iterations = append(lc.Iterations, iter)
			return nil
		}

		lc.Iterations = append(lc.Iterations, iter)
		current = reflection
		lc.State = StatePlanning
	}

	if lc.AllowIncomplete {
		return nil
	}

	return fmt.Errorf("reached max loops: %d", maxLoops)
}

func isLoopDone(reflection string) bool {
	return strings.Contains(strings.ToUpper(reflection), "DONE")
}
