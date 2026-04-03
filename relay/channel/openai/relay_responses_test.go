package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupResponsesStreamTest(t *testing.T, body string, path string) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, *http.Response) {
	t.Helper()

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Set(common.RequestIdKey, "req-test")

	info := &relaycommon.RelayInfo{
		StartTime:         time.Now().Add(-100 * time.Millisecond),
		OriginModelName:   "gpt-5.4",
		ChannelMeta:       &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.4"},
		FirstResponseTime: time.Time{},
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}

	return c, recorder, info, resp
}

func TestOaiResponsesStreamHandler_CompletedStreamRequiresResponseCompleted(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4"}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
	}, "\n") + "\n"

	c, recorder, info, resp := setupResponsesStreamTest(t, body, "/v1/responses")

	usage, err := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	assert.True(t, info.StreamCompleted)
	assert.False(t, info.StreamIncomplete)
	assert.Equal(t, 5, usage.TotalTokens)
	assert.Equal(t, 3, usage.PromptTokens)
	assert.Equal(t, 2, usage.CompletionTokens)
	assert.Contains(t, recorder.Body.String(), `event: response.completed`)
	assert.NotContains(t, recorder.Body.String(), `"code":"stream_incomplete"`)
}

func TestOaiResponsesStreamHandler_IncompleteStreamSendsTerminalError(t *testing.T) {
	t.Parallel()

	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n"
	c, recorder, info, resp := setupResponsesStreamTest(t, body, "/v1/responses")

	usage, err := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	assert.True(t, info.StreamIncomplete)
	assert.False(t, info.StreamCompleted)
	assert.Equal(t, "stream_incomplete", info.StreamErrorCode)
	assert.Equal(t, 0, usage.TotalTokens)
	assert.Contains(t, recorder.Body.String(), `event: error`)
	assert.Contains(t, recorder.Body.String(), `"code":"stream_incomplete"`)
	assert.Contains(t, recorder.Body.String(), `"message":"stream closed before response.completed"`)
}

func TestOaiResponsesStreamHandler_EmptyStreamReturnsStructuredError(t *testing.T) {
	t.Parallel()

	c, _, info, resp := setupResponsesStreamTest(t, "", "/v1/responses")

	usage, err := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, usage)
	require.NotNil(t, err)
	assert.Equal(t, http.StatusRequestTimeout, err.StatusCode)
	assert.Equal(t, types.ErrorCode("stream_incomplete"), err.GetErrorCode())
	assert.Equal(t, "stream closed before response.completed", err.Error())
}

func TestOaiResponsesToChatStreamHandler_IncompleteStreamSkipsStopAndDone(t *testing.T) {
	t.Parallel()

	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n"
	c, recorder, info, resp := setupResponsesStreamTest(t, body, "/v1/chat/completions")
	info.RelayFormat = types.RelayFormatOpenAI
	info.ShouldIncludeUsage = true

	usage, err := OaiResponsesToChatStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	assert.True(t, info.StreamIncomplete)
	assert.False(t, info.StreamCompleted)
	assert.Equal(t, 0, usage.TotalTokens)
	assert.Contains(t, recorder.Body.String(), `"content":"hello"`)
	assert.NotContains(t, recorder.Body.String(), `"finish_reason":"stop"`)
	assert.NotContains(t, recorder.Body.String(), `[DONE]`)
}
