/**
 * JWT Token 工具函数
 * 统一处理token的验证、解析和管理
 * 
 * @author hienao
 * @date 2026-01-31
 */

/**
 * JWT Payload 类型定义
 */
export interface JwtPayload {
  exp?: number
  iat?: number
  sub?: string
  [key: string]: unknown
}

/**
 * Cookie 配置类型
 */
export interface CookieConfig {
  default: () => null
  maxAge: number
  secure: boolean
  sameSite: 'lax' | 'strict' | 'none'
  httpOnly: boolean
  path: string
}

/**
 * 获取统一的Cookie配置
 * @param maxAge - Cookie最大存活时间（秒）
 * @returns Cookie配置对象
 */
export function getCookieConfig(maxAge: number = 60 * 60 * 24): CookieConfig {
  return {
    default: () => null,
    maxAge: maxAge,
    secure: false, // 本地 HTTP 服务使用非 Secure Cookie
    sameSite: 'lax', // 使用lax策略，兼容性更好
    httpOnly: false, // 客户端需要访问
    path: '/' // 明确设置路径
  }
}

/**
 * 解析JWT payload
 * @param token - JWT token
 * @returns 解析后的payload
 */
export function parseJwtPayload(token: string): JwtPayload {
  const parts = token.split('.')
  if (parts.length !== 3) {
    throw new Error('Invalid JWT format')
  }
  
  const payload = parts[1]
  if (!payload) throw new Error('Missing JWT payload')
  const decoded = atob(payload.replace(/-/g, '+').replace(/_/g, '/'))
  return JSON.parse(decoded) as JwtPayload
}

/**
 * 验证token是否有效（仅前端验证）
 * @param token - JWT token
 * @returns 是否有效
 */
export function isValidToken(token: string | null | undefined): boolean {
  if (!token) return false
  
  try {
    // 解析JWT token
    const payload = parseJwtPayload(token)
    
    // 检查token是否有exp字段
    if (payload.exp === undefined) {
      console.warn('Token缺少exp字段，视为无效')
      return false
    }
    
    // 检查是否过期
    const now = Math.floor(Date.now() / 1000)
    const isValid = payload.exp > now
    
    if (!isValid) {
      console.warn('Token已过期')
    }
    
    return isValid
  } catch (error) {
    console.error('Token解析失败，视为无效:', error)
    return false
  }
}

/**
 * 检查是否需要刷新token
 * 匹配后端策略：已使用超过7天且剩余少于7天时需要刷新
 * @param token - JWT token
 * @returns 是否需要刷新
 */
export function shouldRefreshToken(token: string): boolean {
  try {
    const payload = parseJwtPayload(token)
    const now = Math.floor(Date.now() / 1000)
    
    // 检查必要字段
    if (!payload.exp || !payload.iat) {
      console.warn('Token缺少exp或iat字段，无法计算刷新时机')
      return false
    }
    
    // 计算已使用时间（秒）
    const usedSeconds = now - payload.iat
    // 计算剩余时间（秒）
    const remainingSeconds = payload.exp - now
    
    // 7天 = 7 * 24 * 60 * 60 = 604800 秒
    const sevenDays = 7 * 24 * 60 * 60
    
    // 如果已使用超过7天且剩余时间少于7天，则需要刷新
    const shouldRefresh = usedSeconds > sevenDays && remainingSeconds < sevenDays && remainingSeconds > 0
    
    if (shouldRefresh) {
      console.log(`Token需要刷新：已使用${Math.floor(usedSeconds / 86400)}天，剩余${Math.floor(remainingSeconds / 86400)}天`)
    }
    
    return shouldRefresh
  } catch (error) {
    console.error('检查Token刷新状态失败:', error)
    return false
  }
}

/**
 * 获取 token 已使用时间（秒）
 * @param token - JWT token
 * @returns 已使用秒数，-1表示无效
 */
export function getTokenAge(token: string): number {
  try {
    const payload = parseJwtPayload(token)
    if (!payload.iat) return -1
    
    const now = Math.floor(Date.now() / 1000)
    return now - payload.iat
  } catch {
    return -1
  }
}

/**
 * 清除所有认证相关的cookie
 */
export function clearAuthCookies(): void {
  // 使用统一的Cookie配置来清除Cookie
  const cookieConfig = getCookieConfig()
  const tokenCookie = useCookie('token', {
    default: cookieConfig.default,
    maxAge: cookieConfig.maxAge,
    secure: cookieConfig.secure,
    sameSite: cookieConfig.sameSite,
    path: cookieConfig.path
  })
  const userInfoCookie = useCookie('userInfo', {
    default: cookieConfig.default,
    maxAge: cookieConfig.maxAge,
    secure: cookieConfig.secure,
    sameSite: cookieConfig.sameSite,
    path: cookieConfig.path
  })

  // 设置为null并立即过期
  tokenCookie.value = null
  userInfoCookie.value = null

  // 在客户端环境下，额外使用document.cookie来确保清除
  if (import.meta.client) {
    // 设置过期时间为过去的时间来删除Cookie
    document.cookie = 'token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; SameSite=Lax'
    document.cookie = 'userInfo=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; SameSite=Lax'
    console.log('已通过document.cookie强制清除认证Cookie')
  }

  console.log('已清除所有认证相关的cookie')
}

/**
 * 验证token是否与后端匹配（通过API调用）
 * @param token - JWT token
 * @returns 是否有效
 */
export async function validateTokenWithBackend(token: string): Promise<boolean> {
  if (!token) return false
  
  try {
    const { apiCall } = await import('~/core/utils/api')
    
    const response = await apiCall('/validate', {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
    
    return response.code === 200
  } catch (error) {
    console.error('后端token验证失败:', error)
    return false
  }
}

/**
 * 获取token的剩余有效时间（秒）
 * @param token - JWT token
 * @returns 剩余秒数，-1表示无效或已过期
 */
export function getTokenRemainingTime(token: string): number {
  try {
    const payload = parseJwtPayload(token)
    if (!payload.exp) return -1
    
    const now = Math.floor(Date.now() / 1000)
    const remaining = payload.exp - now
    
    return remaining > 0 ? remaining : -1
  } catch {
    return -1
  }
}

/**
 * 格式化token剩余时间为可读字符串
 * @param token - JWT token
 * @returns 格式化的时间字符串
 */
export function formatTokenRemainingTime(token: string): string {
  const remaining = getTokenRemainingTime(token)
  
  if (remaining <= 0) return '已过期'
  
  const days = Math.floor(remaining / (24 * 3600))
  const hours = Math.floor((remaining % (24 * 3600)) / 3600)
  const minutes = Math.floor((remaining % 3600) / 60)
  
  if (days > 0) return `${days}天${hours}小时`
  if (hours > 0) return `${hours}小时${minutes}分钟`
  return `${minutes}分钟`
}
