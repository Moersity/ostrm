/**
 * API 客户端
 * 基于 ofetch 的 Nuxt 原生支持
 */

import { useAuthStore } from '~/core/stores/auth'

// API 基础配置
const API_BASE_URL = '/'

// Token 刷新互斥锁
let isRefreshing = false
let refreshPromise: Promise<void> | null = null
const failedQueue: Array<{ resolve: () => void; reject: (err: Error) => void }> = []

const processQueue = (error: Error | null) => {
  failedQueue.forEach(prom => {
    if (error) prom.reject(error)
    else prom.resolve()
  })
  failedQueue.length = 0
}

// 获取认证 Token - 根据存储类型标识读取
const getToken = (): string | null => {
  if (import.meta.server) return null

  try {
    const storageType = localStorage.getItem('auth_storage_type')
    const storage = storageType === 'local' ? localStorage : sessionStorage
    const token = storage.getItem('auth_token')
    
    if (token && token !== 'undefined' && token !== 'null') {
      return token
    }
    return null
  } catch (e) {
    console.error('[API Client] 获取Token失败:', e)
    return null
  }
}

// 请求拦截器
const onRequest = (context: { request: string; options: { method?: string; headers?: Record<string, string> } }) => {
  const token = getToken()
  
  // 构建新的 headers 对象（合并现有 headers 和认证 headers）
  const existingHeaders = context.options.headers || {}
  const newHeaders: Record<string, string> = {
    ...existingHeaders,
    'X-Requested-With': 'XMLHttpRequest',
    'Accept': 'application/json'
  }
  
  // 添加认证头（如果调用者没有设置且token存在）
  if (token && !existingHeaders['Authorization']) {
    newHeaders['Authorization'] = `Bearer ${token}`
  }
  
  // 重新赋值整个 headers 对象（确保修改生效）
  context.options.headers = newHeaders

  // CSRF Token 处理
  if (import.meta.client) {
    const csrfToken = useCookie('XSRF-TOKEN')
    if (csrfToken.value && context.options.method !== 'GET') {
      context.options.headers['X-XSRF-TOKEN'] = csrfToken.value
    }
  }
}

// 401 错误处理 - Token 刷新
const handleUnauthorized = (): Promise<void> => {
  return new Promise((resolve, reject) => {
    if (isRefreshing) {
      failedQueue.push({ resolve, reject })
      return
    }

    isRefreshing = true
    
    // 获取当前 token
    const token = getToken()
    
    if (!token) {
      // 没有 token，无法刷新
      isRefreshing = false
      reject(new Error('无 Token，无法刷新'))
      return
    }

    // 尝试刷新 Token（必须携带当前 token）
    $fetch('/api/auth/refresh', { 
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`
      }
    } as object)
      .then((response: unknown) => {
        // 如果刷新成功，更新存储中的 token
        const data = response as { code?: number; data?: { token?: string } }
        if (data?.code === 200 && data?.data?.token) {
          const newToken = data.data.token
          // 更新 authStore
          const authStore = useAuthStore()
          authStore.updateToken(newToken)
          console.log('[API Client] Token 刷新成功')
        }
        processQueue(null)
        resolve()
      })
      .catch((error) => {
        console.error('[API Client] Token 刷新失败:', error)
        processQueue(error instanceof Error ? error : new Error('Token 刷新失败'))
        reject(error)
      })
      .finally(() => {
        isRefreshing = false
      })
  })
}

// 响应错误拦截器
// 使用 WeakMap 记录重试次数，防止无限循环
const retryCountMap = new Map<string, number>()
const MAX_RETRY_COUNT = 1

const onResponseError = async (context: { response: { status: number; statusText: string; json(): Promise<unknown> }; request: string; options: { headers?: Record<string, string>; [key: string]: unknown } }) => {
  const response = context.response
  const requestKey = `${context.request}_${Date.now()}`

  // 401 Unauthorized - Token 过期或无效
  if (response.status === 401) {
    // 检查重试次数
    const currentRetry = retryCountMap.get(context.request) || 0
    if (currentRetry >= MAX_RETRY_COUNT) {
      console.log('[API] 已达最大重试次数，停止重试')
      retryCountMap.delete(context.request)
      // 刷新失败，跳转到登录页
      if (import.meta.client) {
        const authStore = useAuthStore()
        authStore.logout()
        window.location.href = '/auth/login'
      }
      throw new Error('登录已过期，请重新登录')
    }

    try {
      retryCountMap.set(context.request, currentRetry + 1)
      await handleUnauthorized()
      
      // 重试原始请求 - 需要清除旧的 Authorization header
      // 让 onRequest 拦截器使用新 token 重新设置
      const retryOptions = { ...context.options }
      if (retryOptions.headers) {
        delete retryOptions.headers['Authorization']
        delete retryOptions.headers['authorization']
      }
      
      console.log('[API] 使用新 token 重试请求:', context.request)
      // 使用 $fetchApi 重试，它会通过 onRequest 拦截器添加新 token
      const result = await $fetchApi(context.request, retryOptions)
      retryCountMap.delete(context.request)
      return result
    } catch (err) {
      retryCountMap.delete(context.request)
      // 刷新失败，跳转到登录页
      if (import.meta.client) {
        const authStore = useAuthStore()
        authStore.logout()
        window.location.href = '/auth/login'
      }
      throw new Error('登录已过期，请重新登录')
    }
  }

  // 403 Forbidden - 无权限
  if (response.status === 403) {
    throw new Error('您没有权限执行此操作')
  }

  // 422 Unprocessable Entity - 业务验证失败
  if (response.status === 422) {
    const data = await response.json().catch(() => ({}))
    const message = (data as Record<string, unknown>).message || '请求参数验证失败'
    throw new Error(message as string)
  }

  // 429 Too Many Requests - 限流
  if (response.status === 429) {
    throw new Error('请求过于频繁，请稍后再试')
  }

  // 500 Internal Server Error - 服务器错误
  if (response.status >= 500) {
    throw new Error('服务器错误，请稍后重试')
  }

  // 其他错误
  const errorText = response.statusText || '请求失败'
  throw new Error(errorText)
}

// 创建 ofetch 实例
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const $fetchApi = $fetch.create({
  baseURL: API_BASE_URL,
  credentials: 'include',
  timeout: 30000,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  onRequest: onRequest as any,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  onResponseError: onResponseError as any
})

// 便捷 API 方法
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const $api = {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  get: <T = any>(url: string, options?: object) =>
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    $fetchApi<T>(url, { method: 'GET', ...options } as any),

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  post: <T = any>(url: string, body?: unknown, options?: object) =>
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    $fetchApi<T>(url, { method: 'POST', body, ...options } as any),

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  put: <T = any>(url: string, body?: unknown, options?: object) =>
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    $fetchApi<T>(url, { method: 'PUT', body, ...options } as any),

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  patch: <T = any>(url: string, body?: unknown, options?: object) =>
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    $fetchApi<T>(url, { method: 'PATCH', body, ...options } as any),

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  delete: <T = any>(url: string, options?: object) =>
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    $fetchApi<T>(url, { method: 'DELETE', ...options } as any)
}

// 导出类型
export type ApiResponse<T> = {
  data: T
  message?: string
  code?: number
}

// 重新导出 $fetch 以便直接使用
export { $fetch }

// 便捷 API 调用函数（带自动认证）
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const authenticatedApiCall = async <T = any>(
  url: string,
  options: { method?: string; body?: unknown; headers?: Record<string, string> } = {}
): Promise<T> => {
  const { method = 'GET', body, headers } = options

  // 确保 URL 以 /api 开头（Go 同源 API）
  const apiUrl = url.startsWith('/api') ? url : `/api${url.startsWith('/') ? url : '/' + url}`

  // 使用 $fetchApi，拦截器会自动添加 Authorization header
  return await $fetchApi<T>(apiUrl, {
    method,
    body,
    headers: {
      'Content-Type': 'application/json',
      ...headers
    }
  } as any)
}

// 便捷 API 调用函数（无认证）
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const apiCall = async <T = any>(
  url: string,
  options: { method?: string; body?: unknown; headers?: Record<string, string> } = {}
): Promise<T> => {
  const { method = 'GET', body, headers } = options

  // 确保 URL 以 /api 开头（Go 同源 API）
  const apiUrl = url.startsWith('/api') ? url : `/api${url.startsWith('/') ? url : '/' + url}`

  return await $fetch<T>(apiUrl, {
    method,
    body,
    headers: {
      'Content-Type': 'application/json',
      ...headers
    }
  } as any)
}
