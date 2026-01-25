// Package openai provides response translation from Orchids to OpenAI.
package openai

import (
	"context"
	"strings"

	claudeopenai "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/claude/openai/chat-completions"
	claudeorchids "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/orchids/claude"
)

type orchidsOpenAIStreamState struct {
	ClaudeParam any
	OpenAIParam any
}

// ConvertOrchidsStreamToOpenAI converts Orchids streaming events to OpenAI SSE chunks.
func ConvertOrchidsStreamToOpenAI(ctx context.Context, model string, originalRequest, request, rawJSON []byte, param *any) []string {
	state := ensureOrchidsOpenAIState(param)
	claudeChunks := claudeorchids.ConvertOrchidsStreamToClaude(ctx, model, originalRequest, request, rawJSON, &state.ClaudeParam)
	if len(claudeChunks) == 0 {
		return nil
	}

	var results []string
	for _, chunk := range claudeChunks {
		dataLine := extractClaudeDataLine(chunk)
		if dataLine == "" {
			continue
		}
		converted := claudeopenai.ConvertClaudeResponseToOpenAI(ctx, model, originalRequest, request, []byte(dataLine), &state.OpenAIParam)
		if len(converted) > 0 {
			results = append(results, converted...)
		}
	}
	return results
}

// ConvertOrchidsNonStreamToOpenAI converts Claude-formatted Orchids responses to OpenAI responses.
func ConvertOrchidsNonStreamToOpenAI(ctx context.Context, model string, originalRequest, request, rawJSON []byte, param *any) string {
	return claudeopenai.ConvertClaudeResponseToOpenAINonStream(ctx, model, originalRequest, request, rawJSON, param)
}

func ensureOrchidsOpenAIState(param *any) *orchidsOpenAIStreamState {
	if param == nil {
		return &orchidsOpenAIStreamState{}
	}
	if *param == nil {
		*param = &orchidsOpenAIStreamState{}
	}
	state, ok := (*param).(*orchidsOpenAIStreamState)
	if !ok || state == nil {
		state = &orchidsOpenAIStreamState{}
		*param = state
	}
	return state
}

func extractClaudeDataLine(chunk string) string {
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			return line
		}
	}
	return ""
}
