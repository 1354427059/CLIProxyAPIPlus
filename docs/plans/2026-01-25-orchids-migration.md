# Orchids Provider Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 CLIProxyAPIPlus 中新增 `orchids` OAuth 反代能力，并提供管理接口与本地 UI 以导入/管理 Orchids 凭据。

**Architecture:** 复用现有 Kiro 分层：`internal/auth/orchids` 负责凭据与 Clerk 会话，`internal/runtime/executor/orchids_executor.go` 负责 WebSocket 反代，`internal/translator/orchids` 负责 Claude/OpenAI ↔ Orchids 的格式转换；在 `sdk/cliproxy/service.go` 注册 provider 与模型，并在管理 API + 本地静态页面提供导入入口。

**Tech Stack:** Go 标准库 + `github.com/gorilla/websocket` + 现有 `sdk/translator` 与 Gin 管理 API。

**Status 说明:** 每个 Task 完成后在标题末尾追加 `✅`。

---

### Task 1: Orchids 配置与常量接入（Status: DONE） ✅

**Files:**
- Modify: `internal/config/config.go:138`
- Modify: `internal/constant/constant.go:6`
- Modify: `internal/config/config.go:60`

**Step 1: Write the failing test**
```go
func TestDefaultOrchidsConfig(t *testing.T) {
    dir := t.TempDir()
    cfgPath := filepath.Join(dir, "config.yaml")
    if err := os.WriteFile(cfgPath, []byte("{}\n"), 0o600); err != nil {
        t.Fatalf("write config: %v", err)
    }
    cfg, err := config.LoadConfig(cfgPath)
    if err != nil {
        t.Fatalf("load config: %v", err)
    }
    if cfg.Orchids.EndpointWS == "" {
        t.Fatalf("orchids ws endpoint not set")
    }
}
```

**Step 2: Run test to verify it fails**
Run: `go test ./internal/config -run TestDefaultOrchidsConfig -v`
Expected: FAIL (missing Orchids config / undefined field)

**Step 3: Write minimal implementation**
```go
type OrchidsConfig struct {
    EndpointWS   string `yaml:"orchids-ws-endpoint" json:"orchids-ws-endpoint"`
    ClerkClient  string `yaml:"orchids-clerk-client" json:"orchids-clerk-client"`
    ClerkToken   string `yaml:"orchids-clerk-token" json:"orchids-clerk-token"`
    TokenRefreshBufferSeconds int `yaml:"orchids-token-refresh-buffer-seconds" json:"orchids-token-refresh-buffer-seconds"`
}
```

**Step 4: Run test to verify it passes**
Run: `go test ./internal/config -run TestDefaultOrchidsConfig -v`
Expected: PASS

**Step 5: Commit**
```bash
git add internal/config/config.go internal/constant/constant.go
git commit -m "feat: add orchids config defaults"
```

---

### Task 2: Orchids 凭据解析与导入（Status: DONE） ✅

**Files:**
- Create: `internal/auth/orchids/credentials.go`
- Create: `internal/auth/orchids/credentials_test.go`
- Create: `sdk/auth/orchids.go`
- Modify: `sdk/auth/refresh_registry.go`

**Step 1: Write the failing test**
```go
func TestParseOrchidsClientJWT(t *testing.T) {
    in := "eyJhbGciOi...sig"
    jwt, err := orchids.ParseClientJWT(in)
    if err != nil || jwt == "" {
        t.Fatalf("expected jwt extracted")
    }
}
```

**Step 2: Run test to verify it fails**
Run: `go test ./internal/auth/orchids -run TestParseOrchidsClientJWT -v`
Expected: FAIL with "undefined: ParseClientJWT"

**Step 3: Write minimal implementation**
```go
func ParseClientJWT(input string) (string, error) { /* 支持纯 JWT / __client= / JWT|rotating */ }
```

**Step 4: Run test to verify it passes**
Run: `go test ./internal/auth/orchids -run TestParseOrchidsClientJWT -v`
Expected: PASS

**Step 5: Commit**
```bash
git add internal/auth/orchids sdk/auth/orchids.go sdk/auth/refresh_registry.go
git commit -m "feat: add orchids credential parser and authenticator"
```

---

### Task 3: 管理 API - Orchids 导入与列表（Status: DONE） ✅

**Files:**
- Modify: `internal/api/server.go`
- Create: `internal/api/handlers/management/orchids.go`
- Create: `internal/api/handlers/management/orchids_test.go`

**Step 1: Write the failing test**
```go
func TestOrchidsImportToken(t *testing.T) {
    // POST /v0/management/orchids/import-token -> 200
}
```

**Step 2: Run test to verify it fails**
Run: `go test ./internal/api/handlers/management -run TestOrchidsImportToken -v`
Expected: FAIL with 404

**Step 3: Write minimal implementation**
```go
type orchidsImportRequest struct { Token string `json:"token"` }
// 解析 __client JWT -> 写入 auth dir, type=orchids
```

**Step 4: Run test to verify it passes**
Run: `go test ./internal/api/handlers/management -run TestOrchidsImportToken -v`
Expected: PASS

**Step 5: Commit**
```bash
git add internal/api/handlers/management/orchids.go internal/api/server.go internal/api/handlers/management/orchids_test.go
git commit -m "feat: add orchids management import endpoint"
```

---

### Task 4: Orchids Translator（Claude/OpenAI ↔ Orchids）（Status: DONE） ✅

**Files:**
- Create: `internal/translator/orchids/claude/init.go`
- Create: `internal/translator/orchids/claude/orchids_claude_request.go`
- Create: `internal/translator/orchids/claude/orchids_claude_stream.go`
- Create: `internal/translator/orchids/openai/init.go`
- Create: `internal/translator/orchids/openai/orchids_openai_request.go`
- Create: `internal/translator/orchids/openai/orchids_openai_response.go`
- Modify: `internal/translator/init.go`

**Step 1: Write the failing test**
```go
func TestClaudeToOrchidsRequest(t *testing.T) {
    in := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
    out := ConvertClaudeRequestToOrchids("claude-sonnet-4-5", in, true)
    if !bytes.Contains(out, []byte(`"type":"user_request"`)) {
        t.Fatalf("missing user_request")
    }
}
```

**Step 2: Run test to verify it fails**
Run: `go test ./internal/translator/orchids/claude -run TestClaudeToOrchidsRequest -v`
Expected: FAIL with "undefined: ConvertClaudeRequestToOrchids"

**Step 3: Write minimal implementation**
```go
func ConvertClaudeRequestToOrchids(model string, raw []byte, stream bool) []byte { /* build Orchids payload */ }
```

**Step 4: Run test to verify it passes**
Run: `go test ./internal/translator/orchids/claude -run TestClaudeToOrchidsRequest -v`
Expected: PASS

**Step 5: Commit**
```bash
git add internal/translator/orchids internal/translator/init.go
git commit -m "feat: add orchids translators"
```

---

### Task 5: Orchids Executor（WebSocket 反代）（Status: DONE） ✅

**Files:**
- Create: `internal/runtime/executor/orchids_executor.go`
- Create: `internal/runtime/executor/orchids_executor_test.go`

**Step 1: Write the failing test**
```go
func TestOrchidsExecutorIdentifier(t *testing.T) {
    exec := NewOrchidsExecutor(&config.Config{})
    if exec.Identifier() != "orchids" { t.Fatalf("identifier mismatch") }
}
```

**Step 2: Run test to verify it fails**
Run: `go test ./internal/runtime/executor -run TestOrchidsExecutorIdentifier -v`
Expected: FAIL with "undefined: NewOrchidsExecutor"

**Step 3: Write minimal implementation**
```go
type OrchidsExecutor struct { cfg *config.Config }
func NewOrchidsExecutor(cfg *config.Config) *OrchidsExecutor { return &OrchidsExecutor{cfg: cfg} }
```

**Step 4: Run test to verify it passes**
Run: `go test ./internal/runtime/executor -run TestOrchidsExecutorIdentifier -v`
Expected: PASS

**Step 5: Commit**
```bash
git add internal/runtime/executor/orchids_executor.go internal/runtime/executor/orchids_executor_test.go
git commit -m "feat: add orchids executor skeleton"
```

---

### Task 6: Provider Wiring & Model Registry（Status: DONE） ✅

**Files:**
- Modify: `sdk/cliproxy/service.go:351`
- Modify: `sdk/cliproxy/service.go:705`
- Modify: `internal/registry/model_definitions.go:1`

**Step 1: Write the failing test**
```go
func TestRegisterModelsForOrchids(t *testing.T) {
    svc := &Service{cfg: &config.Config{}}
    a := &coreauth.Auth{ID: "id", Provider: "orchids"}
    svc.registerModelsForAuth(a)
    if len(GlobalModelRegistry().GetAvailableModelsByProvider("orchids")) == 0 {
        t.Fatalf("expected orchids models")
    }
}
```

**Step 2: Run test to verify it fails**
Run: `go test ./sdk/cliproxy -run TestRegisterModelsForOrchids -v`
Expected: FAIL with "expected orchids models"

**Step 3: Write minimal implementation**
```go
case "orchids":
    models = registry.GetClaudeModels() // 或 GetOrchidsModels()
```

**Step 4: Run test to verify it passes**
Run: `go test ./sdk/cliproxy -run TestRegisterModelsForOrchids -v`
Expected: PASS

**Step 5: Commit**
```bash
git add sdk/cliproxy/service.go internal/registry/model_definitions.go
git commit -m "feat: wire orchids provider and models"
```

---

### Task 7: 本地管理 UI（Status: DONE） ✅

**Files:**
- Create: `static/orchids.html`
- Create: `static/orchids.js`
- Modify: `internal/api/server.go:317`

**Step 1: Write the failing test**
```go
func TestOrchidsUIPresent(t *testing.T) {
    // GET /orchids.html -> 200
}
```

**Step 2: Run test to verify it fails**
Run: `go test ./internal/api -run TestOrchidsUIPresent -v`
Expected: FAIL with 404

**Step 3: Write minimal implementation**
```go
s.engine.GET("/orchids.html", s.serveStaticOrchidsUI)
```

**Step 4: Run test to verify it passes**
Run: `go test ./internal/api -run TestOrchidsUIPresent -v`
Expected: PASS

**Step 5: Commit**
```bash
git add static/orchids.html static/orchids.js internal/api/server.go
git commit -m "feat: add orchids import UI"
```

---

### Task 8: 验证与回归（Status: TODO）

**Files:**
- Modify: `docs/plans/2026-01-25-orchids-migration.md`

**Step 1: Round 1 tests**
Run: `go test ./internal/auth/orchids ./internal/translator/orchids/... -v`
Expected: PASS

**Step 2: Round 2 tests**
Run: `go test ./internal/runtime/executor -run Orchids -v`
Expected: PASS

**Step 3: Round 3 tests**
Run: `go test ./...`
Expected: PASS

**Step 4: Commit**
```bash
git add docs/plans/2026-01-25-orchids-migration.md
git commit -m "docs: mark orchids migration plan done"
```
