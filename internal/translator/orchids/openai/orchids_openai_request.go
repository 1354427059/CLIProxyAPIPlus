// Package openai provides request translation from OpenAI to Orchids.
package openai

import (
	claudeopenai "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/claude/openai/chat-completions"
	claudeorchids "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/orchids/claude"
)

// ConvertOpenAIRequestToOrchids converts OpenAI chat-completions requests to Orchids payloads.
func ConvertOpenAIRequestToOrchids(modelName string, inputRawJSON []byte, stream bool) []byte {
	claudeReq := claudeopenai.ConvertOpenAIRequestToClaude(modelName, inputRawJSON, stream)
	return claudeorchids.ConvertClaudeRequestToOrchids(modelName, claudeReq, stream)
}
