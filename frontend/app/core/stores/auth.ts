/**
 * 认证状态管理 Store
 * 简化存储策略，支持自动 Token 刷新
 * 
 * @author hienao
 * @date 2026-01-31
 */

import { defineStore } from 'pinia'
import { isValidToken, shouldRefreshToken, getTokenRemainingTime } from '~/core/utils/token'

/**
 * 用户信息类型
 */
export interface UserInfo {
  id?: string | number
  username?: string
  email?: string
  [key: string]: unknown
}

/**
 * 认证状态类型
 */
export interface AuthState {
  token: string | null
  userInfo: UserInfo | null
  isLoggedIn: boolean
  tokenExpiresAt: number | null
  isRefreshing: boolean
}

// 存储类型常量
const STORAGE_TYPE_LOCAL = 'local'
const STORAGE_TYPE_SESSION = 'session'

// 存储键名常量
const AUTH_TOKEN_KEY = 'auth_token'
const AUTH_USER_INFO_KEY = 'auth_userInfo'
const AUTH_STORAGE_TYPE_KEY = 'auth_storage_type'

/**
 * 获取当前使用的存储类型
 */
function getStorageType(): string | null {
  if (!import.meta.client) return null
  return localStorage.getItem(AUTH_STORAGE_TYPE_KEY) || STORAGE_TYPE_SESSION
}

/**
 * 获取当前应该使用的存储对象
 */
function getStorage(): Storage | null {
  if (!import.meta.client) return null
  const storageType = getStorageType()
  return storageType === STORAGE_TYPE_LOCAL ? localStorage : sessionStorage
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: null,
    userInfo: null,
    isLoggedIn: false,
    tokenExpiresAt: null,
    isRefreshing: false
  }),

  getters: {
    /**
     * 获取当前token
     */
    getToken: (state): string | null => state.token,

    /**
     * 获取用户信息
     */
    getUserInfo: (state): UserInfo | null => state.userInfo,

    /**
     * 检查是否已登录
     */
    isAuthenticated: (state): boolean => {
      return state.isLoggedIn && !!state.token && isValidToken(state.token)
    },

    /**
     * 获取 Token 剩余有效时间（秒）
     */
    tokenRemainingTime: (state): number => {
      if (!state.token) return 0
      return getTokenRemainingTime(state.token)
    },

    /**
     * 检查是否需要刷新 Token
     */
    needsRefresh: (state): boolean => {
      if (!state.token) return false
      return shouldRefreshToken(state.token)
    }
  },

  actions: {
    /**
     * 初始化认证状态（从存储恢复）
     */
    initAuth(): void {
      if (import.meta.client) {
        this.restoreAuth()
      }
    },

    /**
     * 设置认证信息
     * @param token - JWT token
     * @param userInfo - 用户信息
     * @param rememberMe - 是否记住登录状态
     */
    setAuth(token: string, userInfo: UserInfo, rememberMe: boolean = false): void {
      this.token = token
      this.userInfo = userInfo
      this.isLoggedIn = true

      // 解析过期时间
      try {
        const payload = JSON.parse(atob((token.split('.')[1] || '').replace(/-/g, '+').replace(/_/g, '/'))) as { exp?: number }
        this.tokenExpiresAt = payload.exp !== undefined ? payload.exp * 1000 : null
      } catch (e) {
        console.error('解析token过期时间失败:', e)
        this.tokenExpiresAt = null
      }

      if (import.meta.client) {
        try {
          const storageType = rememberMe ? STORAGE_TYPE_LOCAL : STORAGE_TYPE_SESSION
          const storage = rememberMe ? localStorage : sessionStorage
          
          // 保存存储类型标识到 localStorage（始终持久化）
          localStorage.setItem(AUTH_STORAGE_TYPE_KEY, storageType)
          
          // 如果不记住登录，清除 localStorage 中可能存在的旧 token
          // 防止 getToken 读取到旧的失效 token
          if (!rememberMe) {
            localStorage.removeItem(AUTH_TOKEN_KEY)
            localStorage.removeItem(AUTH_USER_INFO_KEY)
          }
          
          // 保存认证信息到对应存储
          storage.setItem(AUTH_TOKEN_KEY, token)
          storage.setItem(AUTH_USER_INFO_KEY, JSON.stringify(userInfo))
          
          console.log(`认证信息已保存到 ${storageType}Storage`)
        } catch (error) {
          console.error('保存认证信息失败:', error)
        }
      }
    },

    /**
     * 清除认证信息
     */
    clearAuth(): void {
      this.token = null
      this.userInfo = null
      this.isLoggedIn = false
      this.tokenExpiresAt = null
      this.isRefreshing = false

      if (import.meta.client) {
        try {
          // 清除所有存储中的认证信息
          localStorage.removeItem(AUTH_TOKEN_KEY)
          localStorage.removeItem(AUTH_USER_INFO_KEY)
          localStorage.removeItem(AUTH_STORAGE_TYPE_KEY)
          sessionStorage.removeItem(AUTH_TOKEN_KEY)
          sessionStorage.removeItem(AUTH_USER_INFO_KEY)
          console.log('认证信息已清除')
        } catch (error) {
          console.error('清除认证信息失败:', error)
        }
      }
    },

    /**
     * 更新token（用于token刷新）
     * @param newToken - 新的JWT token
     * @param expiresAt - 过期时间戳（毫秒）
     */
    updateToken(newToken: string, expiresAt: number | null = null): void {
      if (newToken && isValidToken(newToken)) {
        this.token = newToken
        this.tokenExpiresAt = expiresAt
        
        if (import.meta.client) {
          try {
            const storage = getStorage()
            if (storage) {
              storage.setItem(AUTH_TOKEN_KEY, newToken)
              console.log('Token已更新')
            }
          } catch (error) {
            console.error('更新Token失败:', error)
          }
        }
      }
    },

    /**
     * 检查并恢复认证状态
     */
    restoreAuth(): boolean {
      if (!import.meta.client) {
        return false
      }

      try {
        const storageType = getStorageType()
        const storage = storageType === STORAGE_TYPE_LOCAL ? localStorage : sessionStorage
        const token = storage.getItem(AUTH_TOKEN_KEY)
        const userInfo = storage.getItem(AUTH_USER_INFO_KEY)

        if (token && isValidToken(token)) {
          this.token = token
          this.userInfo = userInfo ? JSON.parse(userInfo) : null
          this.isLoggedIn = true

          // 解析过期时间
          try {
            const payload = JSON.parse(atob((token.split('.')[1] || '').replace(/-/g, '+').replace(/_/g, '/'))) as { exp?: number }
            this.tokenExpiresAt = payload.exp !== undefined ? payload.exp * 1000 : null
          } catch {
            this.tokenExpiresAt = null
          }

          console.log('认证状态恢复成功')
          return true
        } else {
          this.clearAuth()
          return false
        }
      } catch (error) {
        console.error('恢复认证状态失败:', error)
        this.clearAuth()
        return false
      }
    },

    /**
     * 刷新 Token
     * @returns 刷新是否成功
     */
    async refreshToken(): Promise<boolean> {
      if (this.isRefreshing) {
        console.log('Token刷新已在进行中，跳过')
        return false
      }

      if (!this.token || !isValidToken(this.token)) {
        console.log('Token无效，无法刷新')
        return false
      }

      this.isRefreshing = true

      try {
        const apiModule = await import('~/core/utils/api')
        
        const response = await apiModule.apiCall('/auth/refresh', {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${this.token}`
          }
        }) as { code: number; message?: string; data?: { token: string; expiresAt?: string } }

        if (response.code === 200 && response.data?.token) {
          const newToken = response.data.token
          const expiresAt = response.data.expiresAt ? new Date(response.data.expiresAt).getTime() : null
          
          this.updateToken(newToken, expiresAt)
          console.log('Token刷新成功')
          return true
        } else {
          console.warn('Token刷新失败:', response.message)
          return false
        }
      } catch (error) {
        const err = error as { status?: number }
        console.error('Token刷新错误:', error)
        // 如果刷新失败（如401），可能token已失效，清除认证
        if (err.status === 401) {
          this.clearAuth()
        }
        return false
      } finally {
        this.isRefreshing = false
      }
    },

    /**
     * 退出登录（别名方法，兼容其他模块调用）
     */
    logout(): void {
      this.clearAuth()
    }
  }
})
