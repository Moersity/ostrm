/**
 * API 工具函数
 * 支持 Token 自动刷新和请求重试
 * 
 * @author hienao
 * @date 2026-01-31
 */

import { shouldRefreshToken } from '~/core/utils/token'

/**
 * API 响应类型
 */
export interface ApiResponse<T = unknown> {
  code: number
  message?: string
  data?: T
}

/**
 * HTTP 方法类型
 */
export type HttpMethod = 'GET' | 'HEAD' | 'PATCH' | 'POST' | 'PUT' | 'DELETE' | 'CONNECT' | 'OPTIONS' | 'TRACE'

/**
 * API 请求选项类型
 */
export interface ApiRequestOptions {
  method?: HttpMethod
  headers?: Record<string, string>
  body?: Record<string, unknown> | unknown[] | string
  cache?: RequestCache
  credentials?: RequestCredentials
}

/**
 * 增强的错误类型
 */
export interface ApiError extends Error {
  status?: number
  data?: unknown
  code?: number
}

/**
 * 获取 API 基础 URL
 * 统一开发和生产环境的API调用方式
 */
export function getApiBaseUrl(): string {
  const config = useRuntimeConfig()

  // 直接使用配置的 API 基础路径
  // 开发环境: http://127.0.0.1:3111/api
  // 生产环境: /api (相对路径，由 Nginx 代理)
  return config.public.apiBase as string
}

/**
 * 统一的 API 调用函数
 * @param endpoint - API 端点路径（如 '/auth/sign-in'）
 * @param options - fetch 选项
 * @returns API 响应
 */
export async function apiCall<T = unknown>(
  endpoint: string,
  options: ApiRequestOptions = {},
  handle401 = true
): Promise<ApiResponse<T>> {
  const baseUrl = getApiBaseUrl()
  // 确保 endpoint 以 / 开头，避免重复的 /api 前缀
  const cleanEndpoint = endpoint.startsWith('/') ? endpoint : `/${endpoint}`
  const url = `${baseUrl}${cleanEndpoint}`

  // 默认选项
  const defaultHeaders: Record<string, string> = {
    'Content-Type': 'application/json',
    ...options.headers
  }

  // 合并选项
  const finalOptions = {
    method: (options.method || 'GET') as HttpMethod,
    ...options,
    headers: defaultHeaders
  }

  try {
    return await $fetch<ApiResponse<T>>(url, finalOptions)
  } catch (error: unknown) {
    const fetchError = error as { status?: number; data?: ApiResponse }
    console.error(`API 调用失败: ${url}`, error)

    // 全局处理 401 未授权错误
    if (fetchError.status === 401 && handle401) {
      await handleUnauthorizedError()
    }

    // 处理响应体中的错误信息
    if (fetchError.data) {
      // 如果错误响应中包含 ApiResponse 格式的数据
      const errorData = fetchError.data
      if (errorData.message) {
        // 创建一个新的错误对象，包含正确的错误信息
        const enhancedError: ApiError = new Error(errorData.message)
        if (fetchError.status !== undefined) enhancedError.status = fetchError.status
        enhancedError.data = fetchError.data
        enhancedError.code = errorData.code
        throw enhancedError
      }
    }

    throw error
  }
}

/**
 * 处理 401 未授权错误的统一逻辑
 */
async function handleUnauthorizedError(): Promise<void> {
  console.warn('检测到 401 未授权错误，清除 token 并跳转到登录页')

  // 使用认证store清除认证信息
  const { useAuthStore } = await import('~/core/stores/auth')
  const authStore = useAuthStore()
  authStore.clearAuth()

  // 停止 Token 刷新服务
  const { stopTokenRefreshService } = await import('~/core/utils/tokenRefresh')
  stopTokenRefreshService()

  // 只在客户端执行跳转，避免服务端渲染时的问题
  if (import.meta.client) {
    // 检查用户是否存在，决定跳转到登录页还是注册页
    try {
      const response = await $fetch<ApiResponse<{ exists: boolean }>>(`${getApiBaseUrl()}/auth/check-user`, {
        method: 'GET'
      })

      if (response.code === 200 && response.data?.exists) {
        // 用户存在，跳转到登录页
        await navigateTo('/auth/login')
      } else {
        // 用户不存在，跳转到注册页
        await navigateTo('/auth/register')
      }
    } catch (checkError) {
      console.error('检查用户失败，默认跳转到登录页:', checkError)
      // 检查失败时默认跳转到登录页
      await navigateTo('/auth/login')
    }
  }
}

/**
 * 带认证的 API 调用函数
 * 支持 Token 自动刷新：如果 Token 即将过期，先刷新再请求
 * @param endpoint - API 端点路径
 * @param options - fetch 选项
 * @param skipRefreshCheck - 是否跳过刷新检查（避免循环调用）
 * @returns API 响应
 */
export async function authenticatedApiCall<T = unknown>(
  endpoint: string,
  options: ApiRequestOptions = {},
  skipRefreshCheck: boolean = false
): Promise<ApiResponse<T>> {
  // 使用认证store获取token
  const { useAuthStore } = await import('~/core/stores/auth')
  const authStore = useAuthStore()
  let token = authStore.getToken
  
  if (!token) {
    throw new Error('未找到认证令牌')
  }

  // 检查是否需要在请求前刷新 Token（避免在刷新接口本身调用时循环）
  if (!skipRefreshCheck && shouldRefreshToken(token)) {
    console.log('[API] Token 即将过期，尝试刷新...')
    const refreshed = await authStore.refreshToken()
    if (refreshed) {
      token = authStore.getToken // 使用新 token
      console.log('[API] Token 刷新成功，使用新 Token 发送请求')
    }
  }
  
  const authOptions: ApiRequestOptions = {
    ...options,
    headers: {
      'Authorization': `Bearer ${token}`,
      ...options.headers
    }
  }

  try {
    return await apiCall<T>(endpoint, authOptions, false)
  } catch (error: unknown) {
    const apiError = error as ApiError
    // 如果是 401 错误且还没尝试过刷新，尝试刷新后重试
    if (apiError.status === 401 && !skipRefreshCheck && !authStore.isRefreshing) {
      console.log('[API] 请求返回 401，尝试刷新 Token 后重试...')
      const refreshed = await authStore.refreshToken()
      
      if (refreshed) {
        // 刷新成功，使用新 token 重试请求
        console.log('[API] Token 刷新成功，重试原请求')
        return authenticatedApiCall<T>(endpoint, options, true) // 标记跳过刷新检查
      }
    }
    
    if (apiError.status === 401) await handleUnauthorizedError()

    // 刷新失败或其他错误，继续抛出
    throw error
  }
}
