// Package claude provides translation between Orchids and Claude formats.
package claude

import (
	. "github.com/router-for-me/CLIProxyAPI/v6/internal/constant"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
)

func init() {
	translator.Register(
		Claude,
		Orchids,
		ConvertClaudeRequestToOrchids,
		interfaces.TranslateResponse{
			Stream:    ConvertOrchidsStreamToClaude,
			NonStream: ConvertOrchidsNonStreamToClaude,
		},
	)
}
