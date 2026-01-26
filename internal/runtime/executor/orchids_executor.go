package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/orchids"
	"github.com/gorilla/websocket"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/thinking"
	kiroclaude "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/claude"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	orchidsProviderName = "orchids"
)

var orchidsSessionCache = orchids.NewSessionCache()

// OrchidsExecutor proxies requests to Orchids via WebSocket.
type OrchidsExecutor struct {
	cfg *config.Config
}

// NewOrchidsExecutor constructs a new Orchids executor.
func NewOrchidsExecutor(cfg *config.Config) *OrchidsExecutor {
	return &OrchidsExecutor{cfg: cfg}
}

// Identifier returns the provider name.
func (e *OrchidsExecutor) Identifier() string { return orchidsProviderName }

// PrepareRequest injects Orchids cookies when available.
func (e *OrchidsExecutor) PrepareRequest(req *http.Request, auth *cliproxyauth.Auth) error {
	if req == nil {
		return nil
	}
	clientJWT := orchidsClientJWT(auth)
	if clientJWT != "" && req.Header.Get("Cookie") == "" {
		req.Header.Set("Cookie", "__client="+clientJWT)
	}
	if e.cfg != nil && e.cfg.Orchids.Origin != "" && req.Header.Get("Origin") == "" {
		req.Header.Set("Origin", e.cfg.Orchids.Origin)
	}
	if e.cfg != nil && e.cfg.Orchids.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", e.cfg.Orchids.UserAgent)
	}
	return nil
}

// HttpRequest injects Orchids credentials into the request and executes it.
func (e *OrchidsExecutor) HttpRequest(ctx context.Context, auth *cliproxyauth.Auth, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("orchids executor: request is nil")
	}
	if ctx == nil {
		ctx = req.Context()
	}
	httpReq := req.WithContext(ctx)
	if err := e.PrepareRequest(httpReq, auth); err != nil {
		return nil, err
	}
	httpClient := newProxyAwareHTTPClient(ctx, e.cfg, auth, 0)
	return httpClient.Do(httpReq)
}

// Execute performs a non-streaming request to Orchids.
func (e *OrchidsExecutor) Execute(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (resp cliproxyexecutor.Response, err error) {
	baseModel := thinking.ParseSuffix(req.Model).ModelName
	reporter := newUsageReporter(ctx, e.Identifier(), baseModel, auth)
	defer reporter.trackFailure(ctx, &err)

	translated, originalTranslated, err := e.translateRequest(req, opts, false)
	if err != nil {
		return resp, err
	}

	session, err := ensureOrchidsSession(ctx, e.cfg, auth, e.fetchSession)
	if err != nil {
		return resp, err
	}

	wsConn, err := e.openWebSocket(ctx, auth, session.Token)
	if err != nil {
		return resp, err
	}
	defer func() {
		_ = wsConn.Close()
	}()

	if err = e.awaitConnected(ctx, wsConn); err != nil {
		return resp, err
	}

	if err = wsConn.WriteMessage(websocket.TextMessage, translated); err != nil {
		return resp, err
	}

	content := ""
	stopReason := ""
	for {
		_, message, readErr := wsConn.ReadMessage()
		if readErr != nil {
			return resp, readErr
		}
		msgType := gjson.GetBytes(message, "type").String()
		if msgType == "connected" {
			continue
		}
		if e.handleFsOperation(wsConn, message) {
			continue
		}
		if msgType == "model" {
			eventType := gjson.GetBytes(message, "event.type").String()
			switch eventType {
			case "text-delta":
				content += gjson.GetBytes(message, "event.delta").String()
			case "finish":
				stopReason = mapOrchidsFinishReason(gjson.GetBytes(message, "event.finishReason").String())
			}
		}
		if isOrchidsStreamEnd(msgType) {
			break
		}
	}

	if stopReason == "" {
		stopReason = "end_turn"
	}

	claudeResp := kiroclaude.BuildClaudeResponse(content, nil, baseModel, usage.Detail{}, stopReason)
	reporter.ensurePublished(ctx)
	var param any
	out := sdktranslator.TranslateNonStream(ctx, sdktranslator.FromString(orchidsProviderName), opts.SourceFormat, req.Model, bytes.Clone(opts.OriginalRequest), originalTranslated, claudeResp, &param)
	resp = cliproxyexecutor.Response{Payload: []byte(out)}
	return resp, nil
}

// ExecuteStream performs a streaming request to Orchids.
func (e *OrchidsExecutor) ExecuteStream(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (stream <-chan cliproxyexecutor.StreamChunk, err error) {
	baseModel := thinking.ParseSuffix(req.Model).ModelName
	reporter := newUsageReporter(ctx, e.Identifier(), baseModel, auth)
	defer reporter.trackFailure(ctx, &err)
	out := make(chan cliproxyexecutor.StreamChunk, 16)

	translated, originalTranslated, err := e.translateRequest(req, opts, true)
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(out)

		session, sessionErr := ensureOrchidsSession(ctx, e.cfg, auth, e.fetchSession)
		if sessionErr != nil {
			out <- cliproxyexecutor.StreamChunk{Err: sessionErr}
			return
		}

		wsConn, dialErr := e.openWebSocket(ctx, auth, session.Token)
		if dialErr != nil {
			out <- cliproxyexecutor.StreamChunk{Err: dialErr}
			return
		}
		defer func() {
			_ = wsConn.Close()
		}()

		if waitErr := e.awaitConnected(ctx, wsConn); waitErr != nil {
			out <- cliproxyexecutor.StreamChunk{Err: waitErr}
			return
		}

		if writeErr := wsConn.WriteMessage(websocket.TextMessage, translated); writeErr != nil {
			out <- cliproxyexecutor.StreamChunk{Err: writeErr}
			return
		}

		var translatorParam any
		for {
			_, message, readErr := wsConn.ReadMessage()
			if readErr != nil {
				out <- cliproxyexecutor.StreamChunk{Err: readErr}
				return
			}
			msgType := gjson.GetBytes(message, "type").String()
			if msgType == "connected" {
				continue
			}
			if e.handleFsOperation(wsConn, message) {
				continue
			}

			chunks := sdktranslator.TranslateStream(ctx, sdktranslator.FromString(orchidsProviderName), opts.SourceFormat, req.Model, bytes.Clone(opts.OriginalRequest), originalTranslated, message, &translatorParam)
			for _, chunk := range chunks {
				if strings.TrimSpace(chunk) == "" {
					continue
				}
				out <- cliproxyexecutor.StreamChunk{Payload: []byte(chunk + "\n\n")}
			}

			if isOrchidsStreamEnd(msgType) {
				reporter.ensurePublished(ctx)
				return
			}
		}
	}()

	return out, nil
}

// CountTokens reports token counting unsupported for Orchids.
func (e *OrchidsExecutor) CountTokens(_ context.Context, _ *cliproxyauth.Auth, _ cliproxyexecutor.Request, _ cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "count tokens not supported for orchids"}
}

// Refresh attempts to refresh Orchids session information.
func (e *OrchidsExecutor) Refresh(ctx context.Context, auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "missing auth"}
	}
	session, err := ensureOrchidsSession(ctx, e.cfg, auth, e.fetchSession)
	if err != nil {
		return nil, err
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["session_token"] = session.Token
	if session.UserID != "" {
		auth.Metadata["session_user_id"] = session.UserID
	}
	auth.LastRefreshedAt = time.Now().UTC()
	return auth, nil
}

func (e *OrchidsExecutor) translateRequest(req cliproxyexecutor.Request, opts cliproxyexecutor.Options, stream bool) ([]byte, []byte, error) {
	baseModel := thinking.ParseSuffix(req.Model).ModelName
	from := opts.SourceFormat
	to := sdktranslator.FromString(orchidsProviderName)
	originalPayload := bytes.Clone(req.Payload)
	if len(opts.OriginalRequest) > 0 {
		originalPayload = bytes.Clone(opts.OriginalRequest)
	}
	originalTranslated := sdktranslator.TranslateRequest(from, to, baseModel, originalPayload, stream)
	body := sdktranslator.TranslateRequest(from, to, baseModel, bytes.Clone(req.Payload), stream)
	requestedModel := payloadRequestedModel(opts, req.Model)
	body = applyPayloadConfigWithRoot(e.cfg, baseModel, to.String(), "", body, originalTranslated, requestedModel)
	return body, originalTranslated, nil
}

type orchidsSession struct {
	Token     string
	SessionID string
	UserID    string
}

func ensureOrchidsSession(ctx context.Context, cfg *config.Config, auth *cliproxyauth.Auth, fetcher func(context.Context, *cliproxyauth.Auth) (*orchidsSession, error)) (*orchidsSession, error) {
	if auth == nil {
		return fetcher(ctx, auth)
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	cached := stringValue(auth.Metadata, "session_token")
	if cached != "" {
		if exp, ok := orchids.ParseJWTExpiry(cached); ok {
			bufferSeconds := 0
			if cfg != nil {
				bufferSeconds = cfg.Orchids.TokenRefreshBufferSeconds
			}
			buffer := time.Duration(bufferSeconds) * time.Second
			if time.Now().Add(buffer).Before(exp) {
				return &orchidsSession{
					Token:     cached,
					SessionID: stringValue(auth.Metadata, "session_id"),
					UserID:    stringValue(auth.Metadata, "session_user_id"),
				}, nil
			}
		}
	}

	minIntervalMillis := 0
	if cfg != nil {
		minIntervalMillis = cfg.Orchids.MinRefreshIntervalMillis
	}
	minInterval := time.Duration(minIntervalMillis) * time.Millisecond
	if minInterval > 0 && !orchidsSessionCache.CanRefresh(auth.ID, minInterval) {
		if cached != "" {
			return &orchidsSession{
				Token:     cached,
				SessionID: stringValue(auth.Metadata, "session_id"),
				UserID:    stringValue(auth.Metadata, "session_user_id"),
			}, nil
		}
	}

	result, err := orchidsSessionCache.Do(auth.ID, func() (any, error) {
		return fetcher(ctx, auth)
	})
	if err != nil {
		return nil, err
	}
	session, ok := result.(*orchidsSession)
	if !ok {
		return nil, fmt.Errorf("orchids executor: invalid session result")
	}

	auth.Metadata["session_token"] = session.Token
	if session.SessionID != "" {
		auth.Metadata["session_id"] = session.SessionID
	}
	if session.UserID != "" {
		auth.Metadata["session_user_id"] = session.UserID
	}
	if exp, ok := orchids.ParseJWTExpiry(session.Token); ok {
		auth.Metadata["expires_at"] = exp.UTC().Format(time.RFC3339)
	}
	auth.Metadata["last_refreshed_at"] = time.Now().UTC().Format(time.RFC3339)
	auth.LastRefreshedAt = time.Now().UTC()
	return session, nil
}

func (e *OrchidsExecutor) fetchSession(ctx context.Context, auth *cliproxyauth.Auth) (*orchidsSession, error) {
	if auth == nil {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "missing auth"}
	}
	clientJWT := orchidsClientJWT(auth)
	if clientJWT == "" {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "missing orchids client token"}
	}
	if e.cfg == nil {
		return nil, fmt.Errorf("orchids executor: missing config")
	}
	endpoint := strings.TrimSpace(e.cfg.Orchids.ClerkClientEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("orchids executor: clerk client endpoint not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "__client="+clientJWT)
	if e.cfg.Orchids.Origin != "" {
		req.Header.Set("Origin", e.cfg.Orchids.Origin)
	}
	if e.cfg.Orchids.UserAgent != "" {
		req.Header.Set("User-Agent", e.cfg.Orchids.UserAgent)
	}

	timeout := time.Duration(e.cfg.Orchids.RequestTimeoutSeconds) * time.Second
	client := newProxyAwareHTTPClient(ctx, e.cfg, auth, timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, statusErr{code: resp.StatusCode, msg: string(body)}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	sessionID := gjson.GetBytes(body, "response.sessions.0.id").String()
	userID := gjson.GetBytes(body, "response.sessions.0.user.id").String()
	token := gjson.GetBytes(body, "response.sessions.0.last_active_token.jwt").String()
	if token == "" {
		return nil, statusErr{code: http.StatusUnauthorized, msg: "orchids auth: missing session token"}
	}

	return &orchidsSession{Token: token, SessionID: sessionID, UserID: userID}, nil
}

func (e *OrchidsExecutor) openWebSocket(ctx context.Context, auth *cliproxyauth.Auth, token string) (*websocket.Conn, error) {
	if e.cfg == nil {
		return nil, fmt.Errorf("orchids executor: missing config")
	}
	endpoint := strings.TrimSpace(e.cfg.Orchids.EndpointWS)
	if endpoint == "" {
		return nil, fmt.Errorf("orchids executor: websocket endpoint not configured")
	}
	wsURL := endpoint
	if token != "" {
		delimiter := "?"
		if strings.Contains(endpoint, "?") {
			delimiter = "&"
		}
		wsURL = endpoint + delimiter + "token=" + url.QueryEscape(token)
	}

	headers := http.Header{}
	if e.cfg.Orchids.UserAgent != "" {
		headers.Set("User-Agent", e.cfg.Orchids.UserAgent)
	}
	if e.cfg.Orchids.Origin != "" {
		headers.Set("Origin", e.cfg.Orchids.Origin)
	}

	dialer := websocket.Dialer{HandshakeTimeout: 30 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (e *OrchidsExecutor) awaitConnected(ctx context.Context, conn *websocket.Conn) error {
	deadline := time.Now().Add(30 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	defer func() {
		_ = conn.SetReadDeadline(time.Time{})
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		msgType := gjson.GetBytes(message, "type").String()
		if msgType == "connected" {
			return nil
		}
	}
}

func (e *OrchidsExecutor) handleFsOperation(conn *websocket.Conn, message []byte) bool {
	msgType := gjson.GetBytes(message, "type").String()
	if msgType != "fs_operation" {
		return false
	}
	opID := gjson.GetBytes(message, "id").String()
	if opID == "" {
		return true
	}
	resp := map[string]any{
		"type":    "fs_operation_response",
		"id":      opID,
		"success": true,
		"data":    nil,
	}
	payload, _ := json.Marshal(resp)
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		log.Warnf("orchids: failed to respond fs_operation: %v", err)
	}
	return true
}

func isOrchidsStreamEnd(msgType string) bool {
	switch msgType {
	case "response_done", "coding_agent.end", "complete":
		return true
	default:
		return false
	}
}

func orchidsClientJWT(auth *cliproxyauth.Auth) string {
	if auth == nil || auth.Metadata == nil {
		return ""
	}
	if v, ok := auth.Metadata["client_jwt"]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	if v, ok := auth.Metadata["clientJwt"]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func mapOrchidsFinishReason(reason string) string {
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
