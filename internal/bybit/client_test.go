package bybit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateSignature(t *testing.T) {
	got := generateSignature("secret", "hello")

	require.Equal(t, got, generateSignature("secret", "hello"))
	require.NotEqual(t, got, generateSignature("secret", "Hello"))
}

func TestBuildSignaturePayload(t *testing.T) {
	got := buildSignaturePayload("1724330000123", "abc123", "5000", "category=linear")

	want := "1724330000123abc1235000category=linear"
	require.Equal(t, got, want)
}
