package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestOrchidsExecutorIdentifier(t *testing.T) {
	exec := NewOrchidsExecutor(&config.Config{})
	if exec.Identifier() != "orchids" {
		t.Fatalf("identifier mismatch")
	}
}

func TestEnsureOrchidsSession_Reuse(t *testing.T) {
	cfg := &config.Config{}
	cfg.Orchids.TokenRefreshBufferSeconds = 300
	cfg.Orchids.MinRefreshIntervalMillis = 1000

	future := time.Now().Add(10 * time.Minute)
	jwt := buildTestJWT(map[string]any{"exp": float64(future.Unix())})

	auth := &cliproxyauth.Auth{
		ID: "orchids-1",
		Metadata: map[string]any{
			"session_token":   jwt,
			"session_id":      "sess",
			"session_user_id": "user",
		},
	}

	calls := 0
	fetcher := func(ctx context.Context, auth *cliproxyauth.Auth) (*orchidsSession, error) {
		calls++
		return &orchidsSession{Token: "new"}, nil
	}

	session, err := ensureOrchidsSession(context.Background(), cfg, auth, fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Token != jwt {
		t.Fatalf("expected cached token")
	}
	if calls != 0 {
		t.Fatalf("expected no refresh")
	}
}

func buildTestJWT(payload map[string]any) string {
	header := map[string]any{"alg": "none", "typ": "JWT"}
	return base64url(header) + "." + base64url(payload) + ".sig"
}

func base64url(v map[string]any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestEnsureOrchidsSession_Dedup(t *testing.T) {
	cfg := &config.Config{}
	cfg.Orchids.TokenRefreshBufferSeconds = 300
	cfg.Orchids.MinRefreshIntervalMillis = 1000

	auth := &cliproxyauth.Auth{ID: "orchids-1", Metadata: map[string]any{}}
	calls := int64(0)
	fetcher := func(ctx context.Context, auth *cliproxyauth.Auth) (*orchidsSession, error) {
		atomic.AddInt64(&calls, 1)
		time.Sleep(50 * time.Millisecond)
		jwt := buildTestJWT(map[string]any{"exp": float64(time.Now().Add(10 * time.Minute).Unix())})
		return &orchidsSession{Token: jwt, SessionID: "s", UserID: "u"}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = ensureOrchidsSession(context.Background(), cfg, auth, fetcher)
		}()
	}
	wg.Wait()

	if atomic.LoadInt64(&calls) != 1 {
		t.Fatalf("expected fetcher called once")
	}
}
