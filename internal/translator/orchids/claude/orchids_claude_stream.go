// Package claude provides response translation between Orchids and Claude formats.
package claude

import (
	"context"
	"strings"

	kiroclaude "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/claude"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	"github.com/tidwall/gjson"
)

type orchidsStreamState struct {
	MessageStarted bool
	TextStarted    bool
	TextClosed     bool
	ContentIndex   int
	StopReason     string
	Usage          usage.Detail
}

// ConvertOrchidsStreamToClaude converts Orchids streaming events to Claude SSE.
func ConvertOrchidsStreamToClaude(_ context.Context, model string, originalRequest, request, rawJSON []byte, param *any) []string {
	state := ensureOrchidsStreamState(param)
	msgType := gjson.GetBytes(rawJSON, "type").String()

	switch msgType {
	case "model":
		eventType := gjson.GetBytes(rawJSON, "event.type").String()
		switch eventType {
		case "text-start":
			return state.startTextBlock(model)
		case "text-delta":
			delta := gjson.GetBytes(rawJSON, "event.delta").String()
			if strings.TrimSpace(delta) == "" {
				return nil
			}
			return state.appendTextDelta(model, delta)
		case "text-end":
			return state.stopTextBlock()
		case "finish":
			reason := mapFinishReason(gjson.GetBytes(rawJSON, "event.finishReason").String())
			state.StopReason = reason
			return state.finishMessage()
		default:
			return nil
		}
	case "response_done", "complete", "coding_agent.end":
		return state.finishMessage()
	default:
		return nil
	}
}

// ConvertOrchidsNonStreamToClaude returns Claude responses (executor builds Claude payload).
func ConvertOrchidsNonStreamToClaude(_ context.Context, _ string, originalRequest, request, rawJSON []byte, _ *any) string {
	return string(rawJSON)
}

func ensureOrchidsStreamState(param *any) *orchidsStreamState {
	if param == nil {
		return &orchidsStreamState{}
	}
	if *param == nil {
		*param = &orchidsStreamState{}
	}
	state, ok := (*param).(*orchidsStreamState)
	if !ok || state == nil {
		state = &orchidsStreamState{}
		*param = state
	}
	return state
}

func (s *orchidsStreamState) startTextBlock(model string) []string {
	if s.TextStarted {
		return nil
	}
	results := s.ensureMessageStart(model)
	s.TextStarted = true
	s.TextClosed = false
	s.ContentIndex = 0
	results = append(results, string(kiroclaude.BuildClaudeContentBlockStartEvent(s.ContentIndex, "text", "", "")))
	return results
}

func (s *orchidsStreamState) appendTextDelta(model, delta string) []string {
	results := s.ensureMessageStart(model)
	if !s.TextStarted {
		s.TextStarted = true
		s.TextClosed = false
		s.ContentIndex = 0
		results = append(results, string(kiroclaude.BuildClaudeContentBlockStartEvent(s.ContentIndex, "text", "", "")))
	}
	results = append(results, string(kiroclaude.BuildClaudeStreamEvent(delta, s.ContentIndex)))
	return results
}

func (s *orchidsStreamState) stopTextBlock() []string {
	if !s.TextStarted || s.TextClosed {
		return nil
	}
	s.TextClosed = true
	return []string{string(kiroclaude.BuildClaudeContentBlockStopEvent(s.ContentIndex))}
}

func (s *orchidsStreamState) ensureMessageStart(model string) []string {
	if s.MessageStarted {
		return nil
	}
	s.MessageStarted = true
	return []string{string(kiroclaude.BuildClaudeMessageStartEvent(model, s.Usage.InputTokens))}
}

func (s *orchidsStreamState) finishMessage() []string {
	if !s.MessageStarted {
		return nil
	}
	results := []string{}
	if s.TextStarted && !s.TextClosed {
		results = append(results, string(kiroclaude.BuildClaudeContentBlockStopEvent(s.ContentIndex)))
		s.TextClosed = true
	}
	stopReason := s.StopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	results = append(results, string(kiroclaude.BuildClaudeMessageDeltaEvent(stopReason, s.Usage)))
	results = append(results, string(kiroclaude.BuildClaudeMessageStopOnlyEvent()))
	return results
}

func mapFinishReason(reason string) string {
	reason = strings.TrimSpace(strings.ToLower(reason))
	switch reason {
	case "tool-calls", "tool_calls", "tool_use":
		return "tool_use"
	case "stop", "end", "complete", "done":
		return "end_turn"
	case "max_tokens":
		return "max_tokens"
	default:
		if reason == "" {
			return "end_turn"
		}
		return reason
	}
}
