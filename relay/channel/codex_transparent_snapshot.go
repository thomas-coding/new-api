package channel

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

const (
	arrouteTransparentClientHeadersHeader  = "X-Arroute-Client-Headers"
	arrouteTransparentClientQueryHeader    = "X-Arroute-Client-Query"
	arrouteTransparentSnapshotStatusHeader = "X-Arroute-Transparent-Snapshot-Status"

	transparentCodexSnapshotStatusHeadersOversize   = "headers_oversize"
	transparentCodexSnapshotStatusHeadersEncodeFail = "headers_encode_failed"
	transparentCodexSnapshotStatusQueryOversize     = "query_oversize"

	transparentCodexSnapshotMaxHeadersEncodedBytes = 4 << 10
	transparentCodexSnapshotMaxQueryEncodedBytes   = 2 << 10
)

var codexTransparentSnapshotAllowedRequestHeaders = map[string]struct{}{
	textproto.CanonicalMIMEHeaderKey("Accept"):              {},
	textproto.CanonicalMIMEHeaderKey("Accept-Encoding"):     {},
	textproto.CanonicalMIMEHeaderKey("Accept-Language"):     {},
	textproto.CanonicalMIMEHeaderKey("Content-Type"):        {},
	textproto.CanonicalMIMEHeaderKey("Idempotency-Key"):     {},
	textproto.CanonicalMIMEHeaderKey("OpenAI-Beta"):         {},
	textproto.CanonicalMIMEHeaderKey("OpenAI-Organization"): {},
	textproto.CanonicalMIMEHeaderKey("OpenAI-Project"):      {},
	textproto.CanonicalMIMEHeaderKey("Originator"):          {},
	"Session_id": {},
	textproto.CanonicalMIMEHeaderKey("Traceparent"):                           {},
	textproto.CanonicalMIMEHeaderKey("Tracestate"):                            {},
	textproto.CanonicalMIMEHeaderKey("User-Agent"):                            {},
	textproto.CanonicalMIMEHeaderKey("Version"):                               {},
	textproto.CanonicalMIMEHeaderKey("X-Codex-Beta-Features"):                 {},
	textproto.CanonicalMIMEHeaderKey("X-Codex-Turn-Metadata"):                 {},
	textproto.CanonicalMIMEHeaderKey("X-Codex-Turn-State"):                    {},
	textproto.CanonicalMIMEHeaderKey("X-ResponsesAPI-Include-Timing-Metrics"): {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Arch"):                      {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Lang"):                      {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Os"):                        {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Package-Version"):           {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Retry-Count"):               {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Runtime"):                   {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Runtime-Version"):           {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Timeout"):                   {},
}

// These headers are only needed upstream. Keeping both the direct header and the
// base64 snapshot on the internal new-api -> cliproxy hop bloats request headers
// without helping cliproxy's legacy request builder.
var codexTransparentSnapshotInternalHopRedundantHeaders = map[string]struct{}{
	textproto.CanonicalMIMEHeaderKey("Accept-Encoding"):                       {},
	textproto.CanonicalMIMEHeaderKey("Accept-Language"):                       {},
	textproto.CanonicalMIMEHeaderKey("OpenAI-Beta"):                           {},
	textproto.CanonicalMIMEHeaderKey("OpenAI-Organization"):                   {},
	textproto.CanonicalMIMEHeaderKey("OpenAI-Project"):                        {},
	textproto.CanonicalMIMEHeaderKey("Originator"):                            {},
	textproto.CanonicalMIMEHeaderKey("X-Codex-Beta-Features"):                 {},
	textproto.CanonicalMIMEHeaderKey("X-Codex-Turn-Metadata"):                 {},
	textproto.CanonicalMIMEHeaderKey("X-Codex-Turn-State"):                    {},
	textproto.CanonicalMIMEHeaderKey("X-ResponsesAPI-Include-Timing-Metrics"): {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Arch"):                      {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Lang"):                      {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Os"):                        {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Package-Version"):           {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Retry-Count"):               {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Runtime"):                   {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Runtime-Version"):           {},
	textproto.CanonicalMIMEHeaderKey("X-Stainless-Timeout"):                   {},
}

func ShouldUseTransparentCodexRelayBody(info *relaycommon.RelayInfo) bool {
	return shouldUseTransparentCodexRelay(info, "")
}

func ShouldAttachTransparentCodexSnapshot(info *relaycommon.RelayInfo, requestURL string) bool {
	return shouldUseTransparentCodexRelay(info, requestURL)
}

func shouldUseTransparentCodexRelay(info *relaycommon.RelayInfo, requestURL string) bool {
	if info == nil || !model_setting.GetGlobalSettings().TransparentCodexRelayV1Enabled {
		return false
	}
	switch info.RelayMode {
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
	default:
		return false
	}

	targetURL := strings.TrimSpace(requestURL)
	if targetURL == "" && info.ChannelMeta != nil {
		targetURL = strings.TrimSpace(info.ChannelBaseUrl)
	}
	return isArrouteCliproxyTarget(targetURL, info.ChannelBaseUrl)
}

func applyTransparentCodexSnapshotHeaders(req *http.Request, c *gin.Context) {
	if req == nil || c == nil || c.Request == nil {
		return
	}
	req.Header.Del(arrouteTransparentSnapshotStatusHeader)

	if snapshotHeaders := collectTransparentCodexSnapshotHeaders(c.Request.Header); len(snapshotHeaders) > 0 {
		encoded, err := encodeTransparentCodexSnapshotHeaders(snapshotHeaders)
		switch {
		case err != nil:
			appendTransparentCodexSnapshotStatus(req.Header, transparentCodexSnapshotStatusHeadersEncodeFail)
			logger.LogWarn(c, fmt.Sprintf("transparent codex relay snapshot skipped reason=%s err=%v", transparentCodexSnapshotStatusHeadersEncodeFail, err))
		case encoded == "":
		case len(encoded) > transparentCodexSnapshotMaxHeadersEncodedBytes:
			appendTransparentCodexSnapshotStatus(req.Header, transparentCodexSnapshotStatusHeadersOversize)
			logger.LogWarn(c, fmt.Sprintf(
				"transparent codex relay snapshot skipped reason=%s encoded_size=%d limit=%d",
				transparentCodexSnapshotStatusHeadersOversize,
				len(encoded),
				transparentCodexSnapshotMaxHeadersEncodedBytes,
			))
		default:
			req.Header.Set(arrouteTransparentClientHeadersHeader, encoded)
			stripTransparentCodexSnapshotDuplicateHeaders(req.Header, snapshotHeaders)
		}
	}

	if c.Request.URL == nil {
		return
	}
	rawQuery := strings.TrimSpace(c.Request.URL.RawQuery)
	if rawQuery == "" {
		return
	}
	encodedQuery := base64.RawURLEncoding.EncodeToString([]byte(rawQuery))
	if len(encodedQuery) > transparentCodexSnapshotMaxQueryEncodedBytes {
		appendTransparentCodexSnapshotStatus(req.Header, transparentCodexSnapshotStatusQueryOversize)
		logger.LogWarn(c, fmt.Sprintf(
			"transparent codex relay snapshot skipped reason=%s encoded_size=%d limit=%d",
			transparentCodexSnapshotStatusQueryOversize,
			len(encodedQuery),
			transparentCodexSnapshotMaxQueryEncodedBytes,
		))
		return
	}
	req.Header.Set(arrouteTransparentClientQueryHeader, encodedQuery)
}

func collectTransparentCodexSnapshotHeaders(source http.Header) map[string]string {
	if len(source) == 0 {
		return nil
	}

	headers := make(map[string]string)
	for key, values := range source {
		canonical := textproto.CanonicalMIMEHeaderKey(key)
		if _, ok := codexTransparentSnapshotAllowedRequestHeaders[canonical]; !ok {
			continue
		}
		trimmedValues := make([]string, 0, len(values))
		for _, value := range values {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				trimmedValues = append(trimmedValues, trimmed)
			}
		}
		if len(trimmedValues) == 0 {
			continue
		}
		headers[canonical] = strings.Join(trimmedValues, ", ")
	}

	if len(headers) == 0 {
		return nil
	}
	return headers
}

func encodeTransparentCodexSnapshotHeaders(headers map[string]string) (string, error) {
	if len(headers) == 0 {
		return "", nil
	}
	data, err := common.Marshal(headers)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func stripTransparentCodexSnapshotDuplicateHeaders(headers http.Header, snapshotHeaders map[string]string) {
	if headers == nil || len(snapshotHeaders) == 0 {
		return
	}
	for key, snapshotValue := range snapshotHeaders {
		canonical := textproto.CanonicalMIMEHeaderKey(key)
		if _, ok := codexTransparentSnapshotInternalHopRedundantHeaders[canonical]; !ok {
			continue
		}
		if strings.TrimSpace(headers.Get(canonical)) != strings.TrimSpace(snapshotValue) {
			continue
		}
		headers.Del(canonical)
	}
}

func isTransparentCodexSnapshotHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case strings.ToLower(arrouteTransparentClientHeadersHeader),
		strings.ToLower(arrouteTransparentClientQueryHeader),
		strings.ToLower(arrouteTransparentSnapshotStatusHeader):
		return true
	default:
		return false
	}
}

func appendTransparentCodexSnapshotStatus(headers http.Header, status string) {
	if headers == nil {
		return
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return
	}
	current := strings.TrimSpace(headers.Get(arrouteTransparentSnapshotStatusHeader))
	if current == "" {
		headers.Set(arrouteTransparentSnapshotStatusHeader, status)
		return
	}
	for _, part := range strings.Split(current, ",") {
		if strings.TrimSpace(part) == status {
			return
		}
	}
	headers.Set(arrouteTransparentSnapshotStatusHeader, current+","+status)
}
