// Package claude provides request translation between Claude and Orchids formats.
package claude

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const (
	orchidsDefaultMode  = "agent"
	orchidsDefaultEmail = "bridge@localhost"
	orchidsDefaultUser  = "local_user"
)

type orchidsRequest struct {
	Type string             `json:"type"`
	Data orchidsRequestData `json:"data"`
}

type orchidsRequestData struct {
	ProjectID      *string               `json:"projectId"`
	Prompt         string                `json:"prompt"`
	AgentMode      string                `json:"agentMode"`
	Mode           string                `json:"mode"`
	ChatHistory    []orchidsChatMessage  `json:"chatHistory,omitempty"`
	Email          string                `json:"email"`
	IsLocal        bool                  `json:"isLocal"`
	IsFixingErrors bool                  `json:"isFixingErrors"`
	UserID         string                `json:"userId"`
}

type orchidsChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ConvertClaudeRequestToOrchids converts a Claude request into Orchids payload format.
func ConvertClaudeRequestToOrchids(model string, inputRawJSON []byte, stream bool) []byte {
	_ = stream
	systemPrompt := extractSystemPrompt(inputRawJSON)
	messages := gjson.GetBytes(inputRawJSON, "messages")
	userMessage, history := extractUserMessageAndHistory(messages)
	prompt := buildOrchidsPrompt(systemPrompt, userMessage)

	if strings.TrimSpace(model) == "" {
		model = "claude-sonnet-4-5"
	}

	payload := orchidsRequest{
		Type: "user_request",
		Data: orchidsRequestData{
			ProjectID:      nil,
			Prompt:         prompt,
			AgentMode:      model,
			Mode:           orchidsDefaultMode,
			ChatHistory:    history,
			Email:          orchidsDefaultEmail,
			IsLocal:        false,
			IsFixingErrors: false,
			UserID:         orchidsDefaultUser,
		},
	}

	out, _ := json.Marshal(payload)
	return out
}

func extractSystemPrompt(claudeBody []byte) string {
	systemField := gjson.GetBytes(claudeBody, "system")
	if systemField.IsArray() {
		var sb strings.Builder
		for _, block := range systemField.Array() {
			if block.Get("type").String() == "text" {
				sb.WriteString(block.Get("text").String())
			} else if block.Type == gjson.String {
				sb.WriteString(block.String())
			}
		}
		return sb.String()
	}
	return systemField.String()
}

func extractUserMessageAndHistory(messages gjson.Result) (string, []orchidsChatMessage) {
	if !messages.IsArray() {
		return "", nil
	}

	items := messages.Array()
	lastUserIdx := -1
	for i := len(items) - 1; i >= 0; i-- {
		msg := items[i]
		if msg.Get("role").String() != "user" {
			continue
		}
		content := extractMessageText(msg.Get("content"))
		if strings.TrimSpace(content) == "" {
			continue
		}
		lastUserIdx = i
		break
	}

	userMessage := ""
	if lastUserIdx >= 0 {
		userMessage = extractMessageText(items[lastUserIdx].Get("content"))
	}
	return userMessage, buildChatHistory(items, lastUserIdx)
}

func buildChatHistory(items []gjson.Result, lastUserIdx int) []orchidsChatMessage {
	if len(items) == 0 {
		return nil
	}
	limit := len(items)
	if lastUserIdx >= 0 {
		limit = lastUserIdx
	}
	if limit <= 0 {
		return nil
	}

	result := make([]orchidsChatMessage, 0, limit)
	for i := 0; i < limit; i++ {
		msg := items[i]
		role := msg.Get("role").String()
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(extractMessageText(msg.Get("content")))
		if content == "" {
			continue
		}
		result = append(result, orchidsChatMessage{Role: role, Content: content})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func extractMessageText(content gjson.Result) string {
	if !content.Exists() {
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if !content.IsArray() {
		return ""
	}
	var sb strings.Builder
	for _, block := range content.Array() {
		blockType := block.Get("type").String()
		switch blockType {
		case "text":
			text := block.Get("text").String()
			if text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(text)
			}
		case "tool_result":
			text := block.Get("content").String()
			if text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(text)
			}
		}
	}
	return sb.String()
}

func buildOrchidsPrompt(systemPrompt, userMessage string) string {
	dateStr := time.Now().Format("2006-01-02")

	var sb strings.Builder
	sb.WriteString("<context>\n")
	sb.WriteString("You are Claude, an AI assistant by Anthropic, helping users through a general-purpose API interface. This interface supports various programming languages and tasks beyond any specific framework.\n")
	sb.WriteString("</context>\n\n")
	sb.WriteString("<environment>\n")
	sb.WriteString("Date: ")
	sb.WriteString(dateStr)
	sb.WriteString("\n")
	sb.WriteString("Interface: General API (supports all languages and frameworks)\n")
	sb.WriteString("</environment>\n\n")
	sb.WriteString("<guidelines>\n")
	sb.WriteString("- Respond in the same language the user uses (e.g., Chinese input -> Chinese response).\n")
	sb.WriteString("- Focus on the user's actual request without assumptions about their tech stack.\n")
	sb.WriteString("- For coding tasks, support any language or framework the user is working with.\n")
	sb.WriteString("</guidelines>\n\n")
	sb.WriteString("<tone_and_style>\n")
	sb.WriteString("- Be concise and direct. Eliminate unnecessary filler, pleasantries, or robotic intros (e.g., avoid \"As an AI...\" or \"I can help with that\").\n")
	sb.WriteString("- Answer the user's question immediately without restating it.\n")
	sb.WriteString("- Maintain a professional, objective, and neutral tone.\n")
	sb.WriteString("- Avoid preaching or moralizing; focus purely on the technical or factual aspects of the request.\n")
	sb.WriteString("</tone_and_style>\n\n")

	if strings.TrimSpace(systemPrompt) != "" {
		sb.WriteString("<system_context>\n")
		sb.WriteString(systemPrompt)
		sb.WriteString("\n</system_context>\n\n")
	}

	sb.WriteString("<user_message>\n")
	sb.WriteString(userMessage)
	sb.WriteString("\n</user_message>\n")
	return sb.String()
}
