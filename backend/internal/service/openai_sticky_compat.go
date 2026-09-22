package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/gin-gonic/gin"
)

type openAILegacySessionHashContextKey struct{}

var openAILegacySessionHashKey = openAILegacySessionHashContextKey{}

// DeriveSessionHashFromSeed computes the current-format sticky-session hash
// from an arbitrary seed string.
func DeriveSessionHashFromSeed(seed string) string {
	currentHash, _ := deriveOpenAISessionHashes(seed)
	return currentHash
}

func deriveOpenAISessionHashes(sessionID string) (currentHash string, legacyHash string) {
	normalized := strings.TrimSpace(sessionID)
	if normalized == "" {
		return "", ""
	}

	currentHash = fmt.Sprintf("%016x", xxhash.Sum64String(normalized))
	sum := sha256.Sum256([]byte(normalized))
	legacyHash = hex.EncodeToString(sum[:])
	return currentHash, legacyHash
}

func withOpenAILegacySessionHash(ctx context.Context, legacyHash string) context.Context {
	if ctx == nil {
		return nil
	}
	trimmed := strings.TrimSpace(legacyHash)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, openAILegacySessionHashKey, trimmed)
}

func openAILegacySessionHashFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(openAILegacySessionHashKey).(string)
	return strings.TrimSpace(value)
}

func attachOpenAILegacySessionHashToGin(c *gin.Context, legacyHash string) {
	if c == nil || c.Request == nil {
		return
	}
	c.Request = c.Request.WithContext(withOpenAILegacySessionHash(c.Request.Context(), legacyHash))
}

func (s *OpenAIGatewayService) openAISessionCacheKey(sessionHash string) string {
	normalized := strings.TrimSpace(sessionHash)
	if normalized == "" {
		return ""
	}
	return "openai:" + normalized
}
