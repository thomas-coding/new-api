package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildTransparentCodexRelayRequestBody_PreservesGovernanceAndParamOverride(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"gpt-5",
		"service_tier":"flex",
		"safety_identifier":"user-123",
		"stream_options":{"include_obfuscation":true,"foo":"bar"},
		"user":"legacy-user",
		"context_management":{"compaction":"auto"}
	}`)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []map[string]interface{}{
					{
						"path":  "temperature",
						"mode":  "set",
						"value": 0.2,
					},
					{
						"path": "user",
						"mode": "delete",
					},
				},
			},
			ChannelOtherSettings: dto.ChannelOtherSettings{},
		},
	}

	processed, err := buildTransparentCodexRelayRequestBody(body, info)
	require.NoError(t, err)

	require.Equal(t, "gpt-5", gjson.GetBytes(processed, "model").String())
	require.Equal(t, "auto", gjson.GetBytes(processed, "context_management.compaction").String())
	require.Equal(t, 0.2, gjson.GetBytes(processed, "temperature").Float())
	require.False(t, gjson.GetBytes(processed, "service_tier").Exists())
	require.False(t, gjson.GetBytes(processed, "safety_identifier").Exists())
	require.False(t, gjson.GetBytes(processed, "stream_options.include_obfuscation").Exists())
	require.Equal(t, "bar", gjson.GetBytes(processed, "stream_options.foo").String())
	require.False(t, gjson.GetBytes(processed, "user").Exists())
}

func TestBuildTransparentCodexRelayRequestBody_UsesChannelSettings(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"gpt-5",
		"service_tier":"flex",
		"safety_identifier":"user-123",
		"store":true,
		"stream_options":{"include_obfuscation":true}
	}`)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AllowServiceTier:        true,
				AllowSafetyIdentifier:   true,
				DisableStore:            true,
				AllowIncludeObfuscation: true,
			},
		},
	}

	processed, err := buildTransparentCodexRelayRequestBody(body, info)
	require.NoError(t, err)

	require.Equal(t, "flex", gjson.GetBytes(processed, "service_tier").String())
	require.Equal(t, "user-123", gjson.GetBytes(processed, "safety_identifier").String())
	require.False(t, gjson.GetBytes(processed, "store").Exists())
	require.Equal(t, true, gjson.GetBytes(processed, "stream_options.include_obfuscation").Bool())
}

func TestShouldUseTransparentCodexRelayDirectBody_HeaderOnlyOverride(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []map[string]interface{}{
					{
						"mode":  "pass_headers",
						"value": []string{"Originator"},
					},
				},
			},
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AllowServiceTier:        true,
				AllowInferenceGeo:       true,
				AllowSafetyIdentifier:   true,
				AllowIncludeObfuscation: true,
			},
		},
	}

	require.True(t, shouldUseTransparentCodexRelayDirectBody(info))
	require.NoError(t, applyTransparentCodexRelayHeaderOnlyParamOverride(info))
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
}

func TestShouldUseTransparentCodexRelayDirectBody_RequiresBodyRewrite(t *testing.T) {
	t.Parallel()

	headerOnlyInfo := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []map[string]interface{}{
					{
						"mode":  "pass_headers",
						"value": []string{"Originator"},
					},
				},
			},
			ChannelOtherSettings: dto.ChannelOtherSettings{},
		},
	}
	require.False(t, shouldUseTransparentCodexRelayDirectBody(headerOnlyInfo))

	bodyRewriteInfo := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []map[string]interface{}{
					{
						"path":  "temperature",
						"mode":  "set",
						"value": 0.2,
					},
				},
			},
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AllowServiceTier:        true,
				AllowInferenceGeo:       true,
				AllowSafetyIdentifier:   true,
				AllowIncludeObfuscation: true,
			},
		},
	}
	require.False(t, shouldUseTransparentCodexRelayDirectBody(bodyRewriteInfo))
}

func TestShouldBypassTransparentCodexRelayBodyRewrite(t *testing.T) {
	t.Parallel()

	require.False(t, shouldBypassTransparentCodexRelayBodyRewrite(transparentCodexRelayMaxBodyRewriteBytes))
	require.True(t, shouldBypassTransparentCodexRelayBodyRewrite(transparentCodexRelayMaxBodyRewriteBytes+1))
}
