/** 安装已完成时访问 /setup 的去向。安装向导只在管理后台提供，登录后即管理员。 */
export function resolveCompletedSetupRedirectPath(isAuthenticated: boolean): string {
  return isAuthenticated ? '/dashboard' : '/login'
}
