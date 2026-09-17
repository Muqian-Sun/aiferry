/** 管理后台端口默认值，与后端 server.admin_port 默认值一致。 */
export const DEFAULT_ADMIN_PORT = 8081

/**
 * 安装完成后的登录地址。安装向导运行在用户站端口上，重启后管理后台只在管理端口提供，
 * 所以跳到同主机的管理端口，而不是当前端口的 /login（那里重启后是用户站）。
 */
export function adminConsoleLoginUrl(location: Pick<Location, 'protocol' | 'hostname'>, adminPort: number): string {
  return `${location.protocol}//${location.hostname}:${adminPort}/login`
}
