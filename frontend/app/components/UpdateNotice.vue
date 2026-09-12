<template>
  <section v-if="store.getShowUpdateNotice || upgrading || message" class="max-w-7xl mx-auto px-4 py-3 border-t border-amber-500/30 bg-amber-500/10 text-sm" aria-live="polite">
    <div class="flex flex-wrap items-center gap-3">
      <strong v-if="store.hasUpdate">发现新版本 {{ store.latestVersion }}</strong>
      <span v-if="!upgrading">升级会备份数据库与旧程序，保留配置和已有文件。</span>
      <button v-if="store.updateInfo?.canUpgrade && !upgrading" class="btn-primary" @click="upgrade">直接升级</button>
      <a v-if="store.updateInfo?.releaseUrl" :href="store.updateInfo.releaseUrl" target="_blank" rel="noopener noreferrer" class="underline">查看发行说明 / 安装包</a>
      <button v-if="!upgrading && store.getShowUpdateNotice" class="btn-secondary" @click="skip">跳过此版本</button>
    </div>
    <p v-if="!store.updateInfo?.canUpgrade && store.getShowUpdateNotice" class="mt-2">{{ store.updateInfo?.upgradeReason }}</p>
    <p v-if="message" class="mt-2">{{ message }}</p>
  </section>
</template>
<script setup lang="ts">
import { useVersionStore } from '~/core/stores/version'
import { authenticatedApiCall } from '~/core/utils/api'
const store = useVersionStore()
const upgrading = ref(false)
const message = ref('')
const targetVersion = ref('')
let disposed = false
let timer: ReturnType<typeof setTimeout> | undefined
let deadline = 0
interface Status { stage?: string; message?: string; version?: string; backupPath?: string }
const skip = () => { store.ignoreVersion(store.latestVersion); message.value = '' }
async function poll() {
  if (disposed) return
  if (Date.now() > deadline) {
    upgrading.value = false
    message.value = '等待升级超时，请检查服务状态和日志；不要删除数据目录或升级备份。'
    return
  }
  try {
    const health = await $fetch<{ data: { version: string } }>('/health', { timeout: 5000 })
    if (health.data.version === targetVersion.value) { window.location.reload(); return }
    const result = await authenticatedApiCall<Status>('/version/upgrade')
    if (result.code === 200 && result.data) {
      message.value = result.data.message || '正在升级'
      if (result.data.stage === 'failed') { upgrading.value = false; return }
    }
  } catch { message.value = '正在等待服务重新连接…' }
  if (!disposed) timer = setTimeout(poll, 2000)
}
async function upgrade() {
  upgrading.value = true
  message.value = '正在准备升级…'
  targetVersion.value = store.latestVersion
  deadline = Date.now() + 12 * 60 * 1000
  try {
    const result = await authenticatedApiCall<Status>('/version/upgrade', { method: 'POST', body: { version: targetVersion.value } })
    if (result.code !== 200) throw new Error(result.message || '无法开始升级')
    await poll()
  } catch (error) { upgrading.value = false; message.value = error instanceof Error ? error.message : '升级请求失败，请检查服务状态' }
}
onMounted(async () => {
  try {
    const result = await authenticatedApiCall<Status>('/version/upgrade')
    if (result.data?.stage && !['failed'].includes(result.data.stage)) {
      upgrading.value = true
      targetVersion.value = result.data.version || store.latestVersion
      deadline = Date.now() + 12 * 60 * 1000
      await poll()
    }
  } catch { /* The normal authentication flow handles session expiry. */ }
})
onUnmounted(() => { disposed = true; clearTimeout(timer) })
</script>
