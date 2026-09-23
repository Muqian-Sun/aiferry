//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultLoginAgreementDocumentsCarryWrittenTerms(t *testing.T) {
	docs := DefaultLoginAgreementDocuments()
	ids := make([]string, 0, len(docs))
	titles := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
		titles = append(titles, doc.Title)
		require.NotEmpty(t, doc.ContentMD, "default document %q has no content", doc.ID)
	}
	require.Equal(t, []string{"usage-policy", "privacy"}, ids)
	require.Equal(t, []string{"使用政策", "隐私政策"}, titles)

	// 隐私政策写的失败请求留存期，必须和运维错误日志的默认保留天数一致
	require.Contains(t, docs[1].ContentMD, "30 天后自动删除")
	require.Equal(t, 30, defaultOpsAdvancedSettings().DataRetention.ErrorLogRetentionDays)

	// 默认文档经过规范化后原样保留（没有被当成空文档丢掉）
	require.Equal(t, docs, normalizeLoginAgreementDocuments(docs))
}
