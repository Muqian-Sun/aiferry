/**
 * 条款文档的展示标题。文档由后端代码固定（service/setting_public.go 的 LoginAgreementDocuments），
 * 标题只有中文；页脚、登录勾选、条款页与页签标题按界面语言显示，认得的文档 id 取 legal.documents.*，
 * 其余用文档自带的标题。
 */
const TITLE_KEYS: Record<string, string> = {
  'usage-policy': 'legal.documents.usagePolicy',
  privacy: 'legal.documents.privacy'
}

export function legalDocumentTitle(doc: { id: string; title: string }, t: (key: string) => string): string {
  const key = TITLE_KEYS[doc.id]
  return key ? t(key) : doc.title
}
