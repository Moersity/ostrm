/**
 * 版本检查状态管理
 * 
 * @author hienao
 * @date 2026-01-31
 */

import { defineStore } from 'pinia'
import { authenticatedApiCall } from '~/core/utils/api'

/**
 * 版本信息类型
 */
export interface VersionInfo {
  currentVersion: string
  latestVersion: string
  hasUpdate: boolean
  releaseNotes?: string
  canUpgrade?: boolean
  upgradeReason?: string
  releaseUrl?: string
  downloadUrl?: string
  [key: string]: unknown
}

/**
 * 版本状态类型
 */
export interface VersionState {
  currentVersion: string
  latestVersion: string
  hasUpdate: boolean
  updateInfo: VersionInfo | null
  showUpdateNotice: boolean
  ignoredVersion: string | null
  lastCheckTime: Date | null
  isLoading: boolean
  error: string | null
}

/**
 * 版本检查响应类型
 */
interface VersionCheckResponse {
  code: number
  message?: string
  data?: VersionInfo
}

export const useVersionStore = defineStore('version', {
  state: (): VersionState => ({
    currentVersion: '',
    latestVersion: '',
    hasUpdate: false,
    updateInfo: null,
    showUpdateNotice: true,
    ignoredVersion: null,
    lastCheckTime: null,
    isLoading: false,
    error: null
  }),

  getters: {
    // 获取当前版本
    getCurrentVersion: (state): string => {
      return state.currentVersion || 'dev'
    },

    // 获取最新版本
    getLatestVersion: (state): string => {
      return state.latestVersion || ''
    },

    // 是否有更新
    getHasUpdate: (state): boolean => {
      return state.hasUpdate && state.showUpdateNotice
    },

    // 获取更新信息
    getUpdateInfo: (state): VersionInfo | null => {
      return state.updateInfo
    },

    // 是否显示更新提示
    getShowUpdateNotice: (state): boolean => {
      return state.showUpdateNotice && state.hasUpdate
    },

    // 是否正在加载
    getIsLoading: (state): boolean => {
      return state.isLoading
    },

    // 获取错误信息
    getError: (state): string | null => {
      return state.error
    }
  },

  actions: {
    /**
     * 检查版本更新
     */
    async checkVersion(): Promise<void> {
      try {
        this.isLoading = true
        this.error = null

        const response = await authenticatedApiCall<VersionInfo>('/version/check')

        if (response.code === 200 && response.data) {
          const data = response.data
          this.currentVersion = data.currentVersion
          this.latestVersion = data.latestVersion
          this.hasUpdate = data.hasUpdate
          this.updateInfo = data
          this.lastCheckTime = new Date()

          // 检查是否是用户忽略的版本
          if (this.ignoredVersion === data.latestVersion) {
            this.showUpdateNotice = false
          } else {
            this.showUpdateNotice = true
          }

          // 如果有更新，记录日志
          if (data.hasUpdate) {
            console.log(`发现新版本: ${data.currentVersion} -> ${data.latestVersion}`)
          }
        } else {
          this.error = response.message || '检查版本失败'
        }
      } catch (error) {
        const err = error as Error
        console.error('检查版本失败:', error)
        this.error = err.message || '检查版本失败'
      } finally {
        this.isLoading = false
      }
    },

    /**
     * 忽略特定版本
     */
    ignoreVersion(version: string): void {
      this.ignoredVersion = version
      this.showUpdateNotice = false
      
      // 保存到本地存储
      if (import.meta.client) {
        localStorage.setItem('ignoredVersion', version)
      }
    },

    /**
     * 关闭更新提示
     */
    dismissNotice(): void {
      this.showUpdateNotice = false
    },

    /**
     * 重新显示更新提示
     */
    showNotice(): void {
      this.showUpdateNotice = true
    },

    /**
     * 清除错误信息
     */
    clearError(): void {
      this.error = null
    },

    /**
     * 重置状态
     */
    reset(): void {
      this.currentVersion = ''
      this.latestVersion = ''
      this.hasUpdate = false
      this.updateInfo = null
      this.showUpdateNotice = true
      this.ignoredVersion = null
      this.lastCheckTime = null
      this.isLoading = false
      this.error = null
    },

    /**
     * 从本地存储恢复状态
     */
    restoreFromStorage(): void {
      if (import.meta.client) {
        try {
          const ignoredVersion = localStorage.getItem('ignoredVersion')
          if (ignoredVersion) {
            this.ignoredVersion = ignoredVersion
          }
        } catch (error) {
          console.warn('从本地存储恢复版本状态失败:', error)
        }
      }
    }
  }
})
