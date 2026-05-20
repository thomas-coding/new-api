package channel

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProcessHeaderOverride_ChannelTestSkipsPassthroughRules(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Empty(t, headers)
}

func TestProcessHeaderOverride_ChannelTestSkipsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	_, ok := headers["x-upstream-trace"]
	require.False(t, ok)
}

func TestProcessHeaderOverride_NonTestKeepsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-upstream-trace"])
}

func TestProcessHeaderOverride_RuntimeOverrideIsFinalHeaderMap(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		IsChannelTest:             false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"x-static":  "runtime-value",
			"x-runtime": "runtime-only",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
				"X-Legacy": "legacy-only",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "runtime-value", headers["x-static"])
	require.Equal(t, "runtime-only", headers["x-runtime"])
	_, exists := headers["x-legacy"]
	require.False(t, exists)
}

func TestProcessHeaderOverride_PassthroughSkipsAcceptEncoding(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])

	_, hasAcceptEncoding := headers["accept-encoding"]
	require.False(t, hasAcceptEncoding)
}

func TestProcessHeaderOverride_PassHeadersTemplateSetsRuntimeHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Originator", "Codex CLI")
	ctx.Request.Header.Set("Session_id", "sess-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]any{
				"operations": []any{
					map[string]any{
						"mode":  "pass_headers",
						"value": []any{"Originator", "Session_id", "X-Codex-Beta-Features"},
					},
				},
			},
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-4.1"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	require.Equal(t, "legacy-value", info.RuntimeHeadersOverride["x-static"])

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "Codex CLI", headers["originator"])
	require.Equal(t, "sess-123", headers["session_id"])
	_, exists = headers["x-codex-beta-features"]
	require.False(t, exists)

	upstreamReq := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	applyHeaderOverrideToRequest(upstreamReq, headers)
	require.Equal(t, "Codex CLI", upstreamReq.Header.Get("Originator"))
	require.Equal(t, "sess-123", upstreamReq.Header.Get("Session_id"))
	require.Empty(t, upstreamReq.Header.Get("X-Codex-Beta-Features"))
}

func TestApplyArrouteAffinityHeader_SetsHeaderForCliproxyTargets(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		UserId: 15,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "http://43.135.179.166:18317",
		},
	}

	applyArrouteAffinityHeader(headers, info, "http://43.135.179.166:18317/v1/responses")

	require.Equal(t, "user:15", headers.Get(arrouteAffinityHeader))
}

func TestApplyArrouteAffinityHeader_SetsHeaderForGrok2APITargets(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		UserId: 15,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "http://43.153.119.162:18000",
		},
	}

	applyArrouteAffinityHeader(headers, info, "http://43.153.119.162:18000/v1/responses")

	require.Equal(t, "user:15", headers.Get(arrouteAffinityHeader))
}

func TestApplyArrouteAffinityHeader_SkipsNonCliproxyTargets(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		UserId: 15,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.openai.com",
		},
	}

	applyArrouteAffinityHeader(headers, info, "https://api.openai.com/v1/responses")

	require.Empty(t, headers.Get(arrouteAffinityHeader))
}

func TestApplyArrouteAffinityHeader_SkipsPort80Targets(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		UserId: 15,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "http://grok2api",
		},
	}

	applyArrouteAffinityHeader(headers, info, "http://grok2api/v1/responses")

	require.Empty(t, headers.Get(arrouteAffinityHeader))
}

func TestApplyArrouteAffinityHeader_PreservesExistingHeader(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Set(arrouteAffinityHeader, "user:existing")
	info := &relaycommon.RelayInfo{
		UserId: 15,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "http://43.135.179.166:18317",
		},
	}

	applyArrouteAffinityHeader(headers, info, "http://43.135.179.166:18317/v1/responses")

	require.Equal(t, "user:existing", headers.Get(arrouteAffinityHeader))
}

func TestProcessHeaderOverride_PassthroughSkipsArrouteAffinityHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set(arrouteAffinityHeader, "user:spoofed")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])
	_, exists := headers[strings.ToLower(arrouteAffinityHeader)]
	require.False(t, exists)
}

func TestProcessHeaderOverride_PassthroughSkipsTransparentSnapshotHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set(arrouteTransparentClientHeadersHeader, "spoofed")
	ctx.Request.Header.Set(arrouteTransparentClientQueryHeader, "spoofed")
	ctx.Request.Header.Set(arrouteTransparentSnapshotStatusHeader, "spoofed")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])
	_, exists := headers[strings.ToLower(arrouteTransparentClientHeadersHeader)]
	require.False(t, exists)
	_, exists = headers[strings.ToLower(arrouteTransparentClientQueryHeader)]
	require.False(t, exists)
	_, exists = headers[strings.ToLower(arrouteTransparentSnapshotStatusHeader)]
	require.False(t, exists)
}

func TestApplyTransparentCodexSnapshotHeaders_UsesAllowlistAndRawQuery(t *testing.T) {
	originalEnabled := model_setting.GetGlobalSettings().TransparentCodexRelayV1Enabled
	model_setting.GetGlobalSettings().TransparentCodexRelayV1Enabled = true
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().TransparentCodexRelayV1Enabled = originalEnabled
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace_id=req-1&include=reasoning.encrypted_content", nil)
	ctx.Request.Header.Set("User-Agent", "codex-cli")
	ctx.Request.Header.Set("Version", "0.101.0")
	ctx.Request.Header.Set("Session_id", "sess-123")
	ctx.Request.Header.Set("Originator", "codex_cli_rs")
	ctx.Request.Header.Set("Accept-Encoding", "br")
	ctx.Request.Header.Set("X-Codex-Beta-Features", "beta-1")
	ctx.Request.Header.Set("X-Test-Keep", "nope")

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18317/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "http://127.0.0.1:18317",
		},
	}

	require.True(t, ShouldAttachTransparentCodexSnapshot(info, req.URL.String()))
	applyTransparentCodexSnapshotHeaders(req, ctx)

	headersValue := req.Header.Get(arrouteTransparentClientHeadersHeader)
	require.NotEmpty(t, headersValue)
	headersJSON, err := base64.RawURLEncoding.DecodeString(headersValue)
	require.NoError(t, err)

	var snapshotHeaders map[string]string
	require.NoError(t, json.Unmarshal(headersJSON, &snapshotHeaders))
	require.Equal(t, "codex-cli", snapshotHeaders["User-Agent"])
	require.Equal(t, "0.101.0", snapshotHeaders["Version"])
	require.Equal(t, "sess-123", snapshotHeaders["Session_id"])
	require.Equal(t, "codex_cli_rs", snapshotHeaders["Originator"])
	require.Equal(t, "br", snapshotHeaders["Accept-Encoding"])
	require.Equal(t, "beta-1", snapshotHeaders["X-Codex-Beta-Features"])
	_, exists := snapshotHeaders["X-Test-Keep"]
	require.False(t, exists)

	queryValue := req.Header.Get(arrouteTransparentClientQueryHeader)
	require.NotEmpty(t, queryValue)
	queryBytes, err := base64.RawURLEncoding.DecodeString(queryValue)
	require.NoError(t, err)
	require.Equal(t, "trace_id=req-1&include=reasoning.encrypted_content", string(queryBytes))
}

func TestApplyTransparentCodexSnapshotHeaders_StripsRedundantInternalHeaders(t *testing.T) {
	originalEnabled := model_setting.GetGlobalSettings().TransparentCodexRelayV1Enabled
	model_setting.GetGlobalSettings().TransparentCodexRelayV1Enabled = true
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().TransparentCodexRelayV1Enabled = originalEnabled
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("User-Agent", "codex-cli")
	ctx.Request.Header.Set("Version", "0.101.0")
	ctx.Request.Header.Set("Session_id", "sess-123")
	ctx.Request.Header.Set("Originator", "codex_cli_rs")
	ctx.Request.Header.Set("Accept-Encoding", "br")
	ctx.Request.Header.Set("X-Codex-Beta-Features", "beta-1")
	ctx.Request.Header.Set("X-Codex-Turn-Metadata", "turn-meta")
	ctx.Request.Header.Set("OpenAI-Project", "project-client")

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18317/v1/responses", nil)
	req.Header.Set("User-Agent", "codex-cli")
	req.Header.Set("Version", "0.101.0")
	req.Header.Set("Session_id", "sess-123")
	req.Header.Set("Originator", "codex_cli_rs")
	req.Header.Set("Accept-Encoding", "br")
	req.Header.Set("X-Codex-Beta-Features", "beta-1")
	req.Header.Set("X-Codex-Turn-Metadata", "turn-meta")
	req.Header.Set("OpenAI-Project", "project-override")

	applyTransparentCodexSnapshotHeaders(req, ctx)

	require.Equal(t, "codex-cli", req.Header.Get("User-Agent"))
	require.Equal(t, "0.101.0", req.Header.Get("Version"))
	require.Equal(t, "sess-123", req.Header.Get("Session_id"))
	require.Empty(t, req.Header.Get("Originator"))
	require.Empty(t, req.Header.Get("Accept-Encoding"))
	require.Empty(t, req.Header.Get("X-Codex-Beta-Features"))
	require.Empty(t, req.Header.Get("X-Codex-Turn-Metadata"))
	require.Equal(t, "project-override", req.Header.Get("OpenAI-Project"))
	require.NotEmpty(t, req.Header.Get(arrouteTransparentClientHeadersHeader))
}

func TestApplyTransparentCodexSnapshotHeaders_SkipsOversizedHeaderSnapshot(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("User-Agent", "codex-cli")
	ctx.Request.Header.Set("X-Codex-Turn-State", strings.Repeat("x", 5000))

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18317/v1/responses", nil)
	applyTransparentCodexSnapshotHeaders(req, ctx)

	require.Empty(t, req.Header.Get(arrouteTransparentClientHeadersHeader))
	require.Equal(t, transparentCodexSnapshotStatusHeadersOversize, req.Header.Get(arrouteTransparentSnapshotStatusHeader))
}

func TestApplyTransparentCodexSnapshotHeaders_SkipsOversizedQuerySnapshot(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace_id="+strings.Repeat("q", 3000), nil)
	ctx.Request.Header.Set("User-Agent", "codex-cli")

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:18317/v1/responses", nil)
	applyTransparentCodexSnapshotHeaders(req, ctx)

	require.Empty(t, req.Header.Get(arrouteTransparentClientQueryHeader))
	require.Equal(t, transparentCodexSnapshotStatusQueryOversize, req.Header.Get(arrouteTransparentSnapshotStatusHeader))
}
