// Package service — 上游请求预检（防 400 噪音）
//
// 设计目标：**凡是能预判会 400 的请求，一条都不要发出去。**
// 本文件提供预检函数 + 1 个进程内标记表，供 gateway_forward.go 在
// 「Pre-filter」阶段调用。不依赖 Redis / DB，重启后自动重建。
//
// 对应问题：
//
//	P1  签名污染：同一会话在本账号上撞过一次 Invalid signature 后，
//	    后续请求直接剥离历史 thinking 块，不再撞第二次。
//	P2  prefill：对话以 assistant 结尾时本地剥离，不发上游。
package service

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// P1 · 签名污染标记（进程内，按 账号+会话）
// ---------------------------------------------------------------------------

const signatureDirtyTTL = 2 * time.Hour

var signatureDirtyStore sync.Map // key string -> int64 (expireUnixNano)

func sigDirtyKey(accountID int64, sessionID string) string {
	return "sig_dirty:" + strconv.FormatInt(accountID, 10) + ":" + sessionID
}

// MarkSignatureDirty 记录「该会话在本账号上已经出现过签名校验失败」。
// 之后同会话的请求会走 StripAssistantThinkingBlocks，避免每次都先撞一次 400。
func MarkSignatureDirty(accountID int64, sessionID string) {
	if sessionID == "" {
		return
	}
	key := sigDirtyKey(accountID, sessionID)
	expiresAt := time.Now().Add(signatureDirtyTTL).UnixNano()
	signatureDirtyStore.Store(key, expiresAt)
	// 主动清理过期条目，避免长期运行进程只靠查询命中才删除、导致 map 持续增长。
	// 若同一 key 在到期前被刷新，则旧 timer 不能删掉新条目的更长有效期。
	time.AfterFunc(signatureDirtyTTL, func() {
		if current, ok := signatureDirtyStore.Load(key); ok {
			if currentExpiry, _ := current.(int64); currentExpiry == expiresAt {
				signatureDirtyStore.Delete(key)
			}
		}
	})
}

// IsSignatureDirty 判断该会话在本账号上是否已被标记为签名污染。
func IsSignatureDirty(accountID int64, sessionID string) bool {
	if sessionID == "" {
		return false
	}
	key := sigDirtyKey(accountID, sessionID)
	v, ok := signatureDirtyStore.Load(key)
	if !ok {
		return false
	}
	exp, _ := v.(int64)
	if exp == 0 || time.Now().UnixNano() > exp {
		// CompareAndDelete 避免并发刷新后误删新的有效期。
		signatureDirtyStore.CompareAndDelete(key, v)
		return false
	}
	return true
}

// ResetSignatureDirty 显式清理标记（账号恢复、测试或运维排障时使用）。
func ResetSignatureDirty(accountID int64, sessionID string) {
	signatureDirtyStore.Delete(sigDirtyKey(accountID, sessionID))
}

// ---------------------------------------------------------------------------
// 会话标识提取
// ---------------------------------------------------------------------------

// MetadataSessionID 从 body 的 metadata.user_id 中取出上游会话 ID。
// 兼容新 JSON 格式与 legacy 拼接格式（复用 ParseMetadataUserID）。
func MetadataSessionID(body []byte) string {
	raw := gjson.GetBytes(body, "metadata.user_id")
	if !raw.Exists() || raw.Type != gjson.String {
		return ""
	}
	s := raw.String()
	if s == "" {
		return ""
	}
	if p := ParseMetadataUserID(s); p != nil {
		return p.SessionID
	}
	return ""
}

// ---------------------------------------------------------------------------
// P1 · 剥离历史 thinking 块（保留 thinking 参数，只是不再回传历史块）
// ---------------------------------------------------------------------------

// StripAssistantThinkingBlocks 删除 messages[*] 中 role=assistant 的
// thinking / redacted_thinking 内容块，其余内容原样保留。
//
// 与 FilterThinkingBlocksForRetry 的区别：
//   - 后者会同时关闭 thinking 参数并把 thinking 降级为 text（能力损失大）
//   - 本函数只清历史块，thinking 仍开启，新一轮会生成新的、签名有效的块
//
// 签名是 Anthropic 用私钥签发、绑定到「产生它的 org + 模型」的。
// 一旦账号/模型切换，旧块必然验签失败——与其上送让它拒，不如本地清掉。
func StripAssistantThinkingBlocks(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return body
	}
	msgs, ok := root["messages"].([]any)
	if !ok || len(msgs) == 0 {
		return body
	}

	stripped := false
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if role, _ := msg["role"].(string); role != "assistant" {
			continue
		}
		blocks, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(blocks))
		for _, b := range blocks {
			blk, ok := b.(map[string]any)
			if !ok {
				kept = append(kept, b)
				continue
			}
			blkType, _ := blk["type"].(string)
			switch blkType {
			case "thinking", "redacted_thinking":
				stripped = true // 丢弃
			default:
				kept = append(kept, b)
			}
		}
		if len(kept) == 0 {
			// 整条 assistant 消息只有 thinking，替换成空文本块避免 content 为空数组
			kept = append(kept, map[string]any{"type": "text", "text": "(omitted)"})
		}
		msg["content"] = kept
	}

	if !stripped {
		return body
	}
	out, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return out
}

// ---------------------------------------------------------------------------
// P2 · 去掉结尾的 assistant 轮（prefill）
// ---------------------------------------------------------------------------

// DropTrailingAssistantTurn 删除 messages 末尾连续的 assistant 消息。
//
// 上游对 claude-* 5.5 系明确拒绝“对话以 assistant 收尾”（prefill）：
//
//	"This model does not support assistant message prefill.
//	 The conversation must end with a user message."
//
// 这类请求 100% 会 400，因此必须在本地处理，不要发出去。
//
// 返回：
//   - out     处理后的 body
//   - dropped 是否发生了删除
//   - empty   删除后 messages 为空（调用方应直接返回本地 400，不要发上游）
func DropTrailingAssistantTurn(body []byte) (out []byte, dropped bool, empty bool) {
	if len(body) == 0 {
		return body, false, false
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return body, false, false
	}
	msgs, ok := root["messages"].([]any)
	if !ok || len(msgs) == 0 {
		return body, false, false
	}

	for len(msgs) > 0 {
		m, ok := msgs[len(msgs)-1].(map[string]any)
		if !ok {
			break
		}
		if role, _ := m["role"].(string); role == "assistant" {
			msgs = msgs[:len(msgs)-1]
			dropped = true
			continue
		}
		break
	}

	if !dropped {
		return body, false, false
	}
	if len(msgs) == 0 {
		return body, true, true
	}
	root["messages"] = msgs
	b, err := json.Marshal(root)
	if err != nil {
		return body, true, false
	}
	return b, true, false
}
