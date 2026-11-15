//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildStableSessionSeed(t *testing.T) {
	first := buildStableSessionSeed("client-abc", "hello")
	second := buildStableSessionSeed("client-abc", "hello")
	require.NotEmpty(t, first)
	require.Equal(t, first, second)
	require.Equal(t, "client-abc::hello", first)

	// 两个锚点都为空时，也必须保持确定且非空，避免退化成随机 session。
	require.NotEmpty(t, buildStableSessionSeed("", ""))
	require.Equal(t, buildStableSessionSeed("", ""), buildStableSessionSeed("", ""))
}
