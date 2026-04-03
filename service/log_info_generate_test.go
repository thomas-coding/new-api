package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTextOtherInfo_StreamIncompleteIncludesStateAndSafeFRT(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	start := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:             start,
		FirstResponseTime:     time.Time{},
		IsStream:              true,
		ReceivedResponseCount: 2,
		StreamIncomplete:      true,
		StreamErrorCode:       "stream_incomplete",
		StreamErrorMessage:    "stream closed before response.completed",
		ChannelMeta:           &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)

	assert.Equal(t, float64(0), other["frt"])
	assert.Equal(t, "incomplete", other["stream_state"])
	assert.Equal(t, 2, other["stream_response_count"])
	assert.Equal(t, "stream_incomplete", other["stream_error_code"])
	assert.Equal(t, "stream closed before response.completed", other["stream_error_message"])
	assert.Equal(t, "/v1/responses", other["request_path"])
}
