/**
 * 访问令牌（ADMIN_TOKEN）在前端的存放与广播。
 *
 * 为什么放 localStorage 而不是打包进产物：令牌是**部署方**的秘密，
 * 写进前端产物等于随静态文件一起公开；这里由使用者在界面上填一次，只存本机浏览器。
 */

const KEY = 'novamind.token'

/** 后端要求令牌但前端没有（或令牌不对）时广播，由 <TokenGate /> 弹窗接住。 */
export const TOKEN_REQUIRED_EVENT = 'novamind:token-required'

export function getToken(): string {
  try {
    return localStorage.getItem(KEY) ?? ''
  } catch {
    return ''
  }
}

export function setToken(token: string): void {
  try {
    if (token) {
      localStorage.setItem(KEY, token)
    } else {
      localStorage.removeItem(KEY)
    }
  } catch {
    // 隐私模式下 localStorage 可能不可写：此时本次会话内仍会通过内存变量生效
  }
}

export function clearToken(): void {
  setToken('')
}

export function notifyTokenRequired(): void {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event(TOKEN_REQUIRED_EVENT))
  }
}
