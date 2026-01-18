package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/yukin371/Kore/internal/core"
)

// ReActRunner defines the Plan -> Execute -> Observe -> Reflect hooks.
type ReActRunner interface {
	Plan(ctx context.Context, input string) (string, error)
	Execute(ctx context.Context, plan string) (string, error)
	Observe(ctx context.Context, execution string) (string, error)
	Reflect(ctx context.Context, observation string) (string, error)
}

// SimpleReActRunner is a minimal runner built on top of core.Agent.
type SimpleReActRunner struct {
	Agent        *core.Agent
	PlanModel    string
	ExecuteModel string
	ReviewModel  string
}

func (r *SimpleReActRunner) Plan(ctx context.Context, input string) (string, error) {
	return r.withModel(r.PlanModel, func() (string, error) {
		return r.singleShot(ctx, "You are a planner. Provide a concise plan.", input)
	})
}

func (r *SimpleReActRunner) Execute(ctx context.Context, plan string) (string, error) {
	if r.Agent == nil {
		return "", fmt.Errorf("agent is nil")
	}

	before := len(r.Agent.History.GetMessages())
	_, err := r.withModel(r.ExecuteModel, func() (string, error) {
		if err := r.Agent.Run(ctx, plan); err != nil {
			return "", err
		}
		return "", nil
	})
	if err != nil {
		return "", err
	}

	after := r.Agent.History.GetMessages()
	if len(after) <= before {
		return "", nil
	}

	for i := len(after) - 1; i >= 0; i-- {
		if after[i].Role == "assistant" {
			return strings.TrimSpace(after[i].Content), nil
		}
	}

	return "", nil
}

func (r *SimpleReActRunner) Observe(ctx context.Context, execution string) (string, error) {
	return strings.TrimSpace(execution), nil
}

func (r *SimpleReActRunner) Reflect(ctx context.Context, observation string) (string, error) {
	return r.withModel(r.ReviewModel, func() (string, error) {
		return r.singleShot(ctx, "You are a reviewer. Reflect on the result and note risks.", observation)
	})
}

func (r *SimpleReActRunner) singleShot(ctx context.Context, systemPrompt string, input string) (string, error) {
	if r.Agent == nil {
		return "", fmt.Errorf("agent is nil")
	}

	maxTokens := r.Agent.Config.LLM.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	req := core.ChatRequest{
		Messages: []core.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input},
		},
		MaxTokens:   maxTokens,
		Temperature: r.Agent.Config.LLM.Temperature,
	}

	stream, err := r.Agent.LLMProvider.ChatStream(ctx, req)
	if err != nil {
		return "", err
	}

	var content strings.Builder
	for event := range stream {
		if event.Type == core.EventContent {
			content.WriteString(event.Content)
		}
	}

	return strings.TrimSpace(content.String()), nil
}

func (r *SimpleReActRunner) withModel(model string, fn func() (string, error)) (string, error) {
	if r.Agent == nil || model == "" {
		return fn()
	}

	original := r.Agent.LLMProvider.GetModel()
	if model != "" && model != original {
		r.Agent.LLMProvider.SetModel(model)
	}

	out, err := fn()

	if model != "" && original != model {
		r.Agent.LLMProvider.SetModel(original)
	}

	return out, err
}
