/**
 * 前端日志收集服务
 * 收集console日志并发送到后端
 * 
 * @author hienao
 * @date 2026-01-31
 */

import { useAuthStore } from '~/core/stores/auth'

/**
 * 日志级别类型
 */
export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

/**
 * 日志条目类型
 */
export interface LogEntry {
  timestamp: number
  level: LogLevel
  message: string
  url: string
  userAgent: string
  sessionId?: string
  [key: string]: unknown
}

/**
 * 额外日志信息类型
 */
export interface LogExtra {
  filename?: string
  lineno?: number
  colno?: number
  stack?: string | undefined
  [key: string]: unknown
}

class Logger {
  private logQueue: LogEntry[] = []
  private isProcessing: boolean = false
  private readonly maxQueueSize: number = 100
  private readonly batchSize: number = 10
  private readonly flushInterval: number = 5000 // 5秒

  constructor() {
    // 启动定时发送
    this.startPeriodicFlush()
    
    // 页面卸载时发送剩余日志
    if (import.meta.client) {
      window.addEventListener('beforeunload', () => {
        this.flush(true)
      })
    }
  }

  /**
   * 添加日志到队列
   */
  addLog(level: LogLevel, message: unknown, extra: LogExtra = {}): void {
    const logEntry: LogEntry = {
      timestamp: Date.now(), // 使用数字时间戳而不是ISO字符串
      level,
      message: typeof message === 'string' ? message : JSON.stringify(message),
      url: import.meta.client ? window.location.href : '',
      userAgent: import.meta.client ? navigator.userAgent : '',
      ...extra
    }

    this.logQueue.push(logEntry)

    // 如果队列过大，移除最旧的日志
    if (this.logQueue.length > this.maxQueueSize) {
      this.logQueue.shift()
    }

    // 如果是错误级别，立即发送
    if (level === 'error') {
      this.flush()
    }
  }

  /**
   * 发送日志到后端
   */
  async flush(sync: boolean = false): Promise<void> {
    if (this.isProcessing || this.logQueue.length === 0) {
      return
    }

    this.isProcessing = true

    try {
      // 取出要发送的日志
      const logsToSend = this.logQueue.splice(0, this.batchSize)
      
      // 为每个日志条目添加会话ID
      const enrichedLogs = logsToSend.map(log => ({
        ...log,
        sessionId: this.getSessionId()
      }))
      
      const token = useAuthStore().getToken
      if (!token) return
      await fetch('/api/logs/frontend', {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ logs: enrichedLogs }), keepalive: sync
      })
    } catch (error) {
      // Avoid feeding a transport failure back into the console collection queue.
      // 发送失败时，将日志重新加入队列
      // this.logQueue.unshift(...logsToSend)
    } finally {
      this.isProcessing = false
    }
  }

  /**
   * 获取或生成会话ID
   */
  private getSessionId(): string {
    if (!import.meta.client) return ''
    
    let sessionId = sessionStorage.getItem('logSessionId')
    if (!sessionId) {
      sessionId = 'session_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9)
      sessionStorage.setItem('logSessionId', sessionId)
    }
    return sessionId
  }

  /**
   * 启动定时发送
   */
  private startPeriodicFlush(): void {
    if (import.meta.client) {
      setInterval(() => {
        this.flush()
      }, this.flushInterval)
    }
  }

  /**
   * 记录信息日志
   */
  info(message: unknown, extra: LogExtra = {}): void {
    this.addLog('info', message, extra)
  }

  /**
   * 记录警告日志
   */
  warn(message: unknown, extra: LogExtra = {}): void {
    this.addLog('warn', message, extra)
  }

  /**
   * 记录错误日志
   */
  error(message: unknown, extra: LogExtra = {}): void {
    this.addLog('error', message, extra)
  }

  /**
   * 记录调试日志
   */
  debug(message: unknown, extra: LogExtra = {}): void {
    this.addLog('debug', message, extra)
  }
}

// 创建全局日志实例
const logger = new Logger()

/**
 * 拦截原生console方法
 */
if (import.meta.client) {
  const originalConsole = {
    log: console.log,
    info: console.info,
    warn: console.warn,
    error: console.error,
    debug: console.debug
  }

  // 重写console.log
  console.log = function(...args: unknown[]): void {
    originalConsole.log.apply(console, args)
    logger.info(args.join(' '))
  }

  // 重写console.info
  console.info = function(...args: unknown[]): void {
    originalConsole.info.apply(console, args)
    logger.info(args.join(' '))
  }

  // 重写console.warn
  console.warn = function(...args: unknown[]): void {
    originalConsole.warn.apply(console, args)
    logger.warn(args.join(' '))
  }

  // 重写console.error
  console.error = function(...args: unknown[]): void {
    originalConsole.error.apply(console, args)
    logger.error(args.join(' '))
  }

  // 重写console.debug
  console.debug = function(...args: unknown[]): void {
    originalConsole.debug.apply(console, args)
    logger.debug(args.join(' '))
  }

  // 捕获未处理的错误
  window.addEventListener('error', (event: ErrorEvent) => {
    logger.error(`未捕获的错误: ${event.message}`, {
      filename: event.filename,
      lineno: event.lineno,
      colno: event.colno,
      stack: event.error?.stack
    })
  })

  // 捕获未处理的Promise拒绝
  window.addEventListener('unhandledrejection', (event: PromiseRejectionEvent) => {
    const reason = event.reason as { stack?: string } | undefined
    logger.error(`未处理的Promise拒绝: ${event.reason}`, {
      stack: reason?.stack
    })
  })
}

export default logger
