//go:build unit

package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func metadataBody(t *testing.T, userID string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"user_id": userID},
	})
	require.NoError(t, err)
	return body
}

func TestMetadataSessionID(t *testing.T) {
	const (
		deviceID    = "d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169"
		accountUUID = "11111111-1111-4111-8111-111111111111"
		sessionID   = "7578cf37-aaca-46e4-a45c-71285d9dbb83"
	)
	newFormat, err := json.Marshal(map[string]string{
		"device_id":    deviceID,
		"account_uuid": accountUUID,
		"session_id":   sessionID,
	})
	require.NoError(t, err)
	legacyFormat := "user_" + deviceID + "_account_" + accountUUID + "_session_" + sessionID

	tests := []struct {
		name string
		body []byte
		want string
	}{
		{name: "new JSON format", body: metadataBody(t, string(newFormat)), want: sessionID},
		{name: "legacy format", body: metadataBody(t, legacyFormat), want: sessionID},
		{name: "missing metadata", body: []byte(`{"messages":[]}`), want: ""},
		{name: "non-string user id", body: []byte(`{"metadata":{"user_id":123}}`), want: ""},
		{name: "unparseable user id", body: metadataBody(t, "not-a-user-id"), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MetadataSessionID(tt.body))
		})
	}
}

func TestSignatureDirtyStore(t *testing.T) {
	const (
		accountID = int64(9101)
		sessionID = "session-dirty-1"
	)
	ResetSignatureDirty(accountID, sessionID)
	t.Cleanup(func() { ResetSignatureDirty(accountID, sessionID) })

	require.False(t, IsSignatureDirty(accountID, sessionID))
	MarkSignatureDirty(accountID, sessionID)
	require.True(t, IsSignatureDirty(accountID, sessionID))
	require.False(t, IsSignatureDirty(accountID+1, sessionID))
	require.False(t, IsSignatureDirty(accountID, sessionID+"-other"))

	ResetSignatureDirty(accountID, sessionID)
	require.False(t, IsSignatureDirty(accountID, sessionID))

	MarkSignatureDirty(accountID, "")
	require.False(t, IsSignatureDirty(accountID, ""))
}

func TestIsSignatureDirtyRemovesExpiredEntry(t *testing.T) {
	const (
		accountID = int64(9102)
		sessionID = "session-expired"
	)
	key := sigDirtyKey(accountID, sessionID)
	signatureDirtyStore.Store(key, time.Now().Add(-time.Minute).UnixNano())
	t.Cleanup(func() { signatureDirtyStore.Delete(key) })

	require.False(t, IsSignatureDirty(accountID, sessionID))
	_, ok := signatureDirtyStore.Load(key)
	require.False(t, ok)
}

func TestStripAssistantThinkingBlocks(t *testing.T) {
	t.Run("removes only historical assistant thinking blocks", func(t *testing.T) {
		body := []byte(`{
"thinking":{"type":"adaptive"},
"messages":[
{"role":"user","content":[{"type":"text","text":"hello"}]},
{"role":"assistant","content":[
{"type":"thinking","thinking":"secret","signature":"sig"},
{"type":"redacted_thinking","data":"redacted"},
{"type":"text","text":"answer"}
]},
{"role":"user","content":[{"type":"text","text":"again"}]}
]
}`)

		out := StripAssistantThinkingBlocks(body)
		require.True(t, json.Valid(out))
		require.Equal(t, "adaptive", gjson.GetBytes(out, "thinking.type").String())
		require.Equal(t, int64(1), gjson.GetBytes(out, "messages.1.content.#").Int())
		require.Equal(t, "text", gjson.GetBytes(out, "messages.1.content.0.type").String())
		require.Equal(t, "answer", gjson.GetBytes(out, "messages.1.content.0.text").String())
	})

	t.Run("keeps a placeholder when an assistant turn contains only thinking", func(t *testing.T) {
		body := []byte(`{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"secret"}]}]}`)
		out := StripAssistantThinkingBlocks(body)

		require.True(t, json.Valid(out))
		require.Equal(t, int64(1), gjson.GetBytes(out, "messages.0.content.#").Int())
		require.Equal(t, "text", gjson.GetBytes(out, "messages.0.content.0.type").String())
		require.Equal(t, "(omitted)", gjson.GetBytes(out, "messages.0.content.0.text").String())
	})

	t.Run("returns original body when nothing needs stripping", func(t *testing.T) {
		body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
		require.Equal(t, body, StripAssistantThinkingBlocks(body))
	})

	t.Run("returns original body for malformed JSON", func(t *testing.T) {
		body := []byte(`{"messages":`)
		require.Equal(t, body, StripAssistantThinkingBlocks(body))
	})
}

func TestDropTrailingAssistantTurn(t *testing.T) {
	t.Run("drops a trailing assistant prefill", func(t *testing.T) {
		body := []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"prefill"}]}`)
		out, dropped, empty := DropTrailingAssistantTurn(body)

		require.True(t, dropped)
		require.False(t, empty)
		require.Equal(t, int64(1), gjson.GetBytes(out, "messages.#").Int())
		require.Equal(t, "hi", gjson.GetBytes(out, "messages.0.content").String())
		require.Equal(t, "claude-opus-5-5", gjson.GetBytes(out, "model").String())
	})

	t.Run("drops consecutive trailing assistant messages", func(t *testing.T) {
		body := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"one"},{"role":"assistant","content":"two"}]}`)
		out, dropped, empty := DropTrailingAssistantTurn(body)

		require.True(t, dropped)
		require.False(t, empty)
		require.Equal(t, int64(1), gjson.GetBytes(out, "messages.#").Int())
	})

	t.Run("keeps assistant messages that are not trailing", func(t *testing.T) {
		body := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"answer"},{"role":"user","content":"next"}]}`)
		out, dropped, empty := DropTrailingAssistantTurn(body)

		require.False(t, dropped)
		require.False(t, empty)
		require.Equal(t, body, out)
	})

	t.Run("reports an empty conversation when every message is assistant", func(t *testing.T) {
		body := []byte(`{"messages":[{"role":"assistant","content":"one"},{"role":"assistant","content":"two"}]}`)
		out, dropped, empty := DropTrailingAssistantTurn(body)

		require.True(t, dropped)
		require.True(t, empty)
		require.Equal(t, body, out)
	})

	t.Run("returns original body for malformed JSON", func(t *testing.T) {
		body := []byte(`{"messages":`)
		out, dropped, empty := DropTrailingAssistantTurn(body)

		require.False(t, dropped)
		require.False(t, empty)
		require.Equal(t, body, out)
	})
}
