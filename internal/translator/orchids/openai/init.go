// Package openai provides translation between Orchids and OpenAI formats.
package openai

import (
	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
)

func init() {
	translator.Register(
		OpenAI,
		Orchids,
		ConvertOpenAIRequestToOrchids,
		interfaces.TranslateResponse{
			Stream:    ConvertOrchidsStreamToOpenAI,
			NonStream: ConvertOrchidsNonStreamToOpenAI,
		},
	)
}
