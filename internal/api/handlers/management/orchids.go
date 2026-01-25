package management

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
)

type orchidsImportRequest struct {
	Token string `json:"token"`
}

type orchidsListItem struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	ImportedAt string `json:"imported_at,omitempty"`
}

// ImportOrchidsToken handles manual Orchids token import.
func (h *Handler) ImportOrchidsToken(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
		return
	}
	if strings.TrimSpace(h.cfg.AuthDir) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth directory not configured"})
		return
	}

	var req orchidsImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}

	authenticator := sdkAuth.NewOrchidsAuthenticator()
	record, err := authenticator.Login(context.Background(), h.cfg, &sdkAuth.LoginOptions{
		Metadata: map[string]string{"token": token},
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token", "message": err.Error()})
		return
	}

	savedPath, err := h.saveTokenRecord(c.Request.Context(), record)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"auth-file": savedPath,
		"id":        record.ID,
	})
}

// ListOrchidsTokens returns imported Orchids credentials.
func (h *Handler) ListOrchidsTokens(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config unavailable"})
		return
	}
	if strings.TrimSpace(h.cfg.AuthDir) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth directory not configured"})
		return
	}

	ctx := c.Request.Context()
	store := h.tokenStoreWithBaseDir()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "token store unavailable"})
		return
	}

	auths, err := store.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list_failed", "message": err.Error()})
		return
	}

	items := make([]orchidsListItem, 0)
	for _, auth := range auths {
		if auth == nil {
			continue
		}
		if !strings.EqualFold(auth.Provider, "orchids") {
			if auth.Metadata == nil || !strings.EqualFold(stringValue(auth.Metadata, "type"), "orchids") {
				continue
			}
		}
		item := orchidsListItem{
			ID:    auth.ID,
			Label: auth.Label,
		}
		if auth.Metadata != nil {
			item.ImportedAt = stringValue(auth.Metadata, "imported_at")
		}
		if item.Label == "" {
			item.Label = "orchids"
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"items":      items,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"auth-count": len(items),
	})
}
