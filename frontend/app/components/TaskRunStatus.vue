<template>
  <section v-if="run || pollError" class="run-card mt-5 overflow-hidden rounded-2xl border border-white/10 bg-slate-950/40 text-sm" aria-label="任务进度" data-testid="task-progress">
    <template v-if="run">
      <div class="p-4 sm:p-5">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="flex items-center gap-2 font-medium" :class="statusTone" role="status">
                <span class="h-2 w-2 rounded-full bg-current" :class="{ 'motion-safe:animate-pulse': active }" />
                {{ cancelPending && active ? '正在取消' : labels[run.status] || run.status }}
              </span>
              <span class="rounded-md bg-white/5 px-2 py-0.5 text-xs text-white/45">{{ run.isIncremental ? '增量执行' : '全量执行' }}</span>
            </div>
            <p class="mt-1.5 text-xs text-white/40">执行 #{{ run.id }} <span aria-hidden="true">·</span> {{ active ? '已用时' : '耗时' }} {{ elapsed }}</p>
          </div>
          <button v-if="active" type="button" class="shrink-0 rounded-lg border border-white/10 px-3 py-1.5 text-xs text-white/60 transition hover:border-amber-300/40 hover:text-amber-200 disabled:cursor-wait disabled:opacity-50" :disabled="cancelPending" @click="cancel">{{ cancelPending ? '等待停止…' : '取消任务' }}</button>
        </div>

        <div class="mt-5 flex items-end justify-between gap-3">
          <p class="font-medium text-white/90">{{ headline }}</p>
          <span class="shrink-0 text-xl font-semibold tabular-nums text-white/90">{{ indeterminate ? '···' : `${progress}%` }}</span>
        </div>
        <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-white/10" role="progressbar" aria-label="任务完成进度" :aria-valuenow="indeterminate ? undefined : progress" aria-valuemin="0" aria-valuemax="100" :aria-valuetext="indeterminate ? headline : `${headline}，${progress}%`">
          <div class="h-full rounded-full transition-all duration-500" :class="[barTone, { 'progress-scanning': indeterminate }]" :style="{ width: indeterminate ? '35%' : `${progress}%` }" />
        </div>
        <ol class="mt-3 grid grid-cols-4 gap-1 text-[11px]" aria-label="执行阶段">
          <li v-for="(step, index) in steps" :key="step" class="flex items-center gap-1.5" :class="index <= stepIndex ? 'text-cyan-200/80' : 'text-white/25'" :aria-current="active && index === stepIndex ? 'step' : undefined">
            <span class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full border" :class="index <= stepIndex ? 'border-cyan-300/30 bg-cyan-300/10' : 'border-white/10'">{{ index < stepIndex || completed ? '✓' : index + 1 }}</span>{{ step }}
          </li>
        </ol>

        <div v-if="run.currentFile" class="mt-4 rounded-xl border border-white/5 bg-white/[0.025] px-3 py-2.5">
          <p class="text-[11px] text-white/40">{{ active ? (run.stage === 'DISCOVERY' || run.stage === 'RENAME' ? '当前目录' : '当前文件') : '停止位置' }}<span v-if="active && run.currentFileIndex && run.total" class="float-right tabular-nums">{{ run.currentFileIndex }} / {{ run.total }}</span></p>
          <p class="mt-1 break-all text-xs leading-relaxed text-white/75" :title="run.currentFile">{{ run.currentFile }}</p>
        </div>
        <p class="mt-3 text-xs text-white/45">{{ summary }}</p>
        <div class="mt-3 grid grid-cols-3 divide-x divide-white/10 rounded-xl bg-white/[0.035] py-3 text-center">
          <div><p class="text-lg font-medium tabular-nums text-emerald-200">{{ run.processed || 0 }}</p><p class="mt-0.5 text-[11px] text-white/40">处理成功</p></div>
          <div><p class="text-lg font-medium tabular-nums text-white/70">{{ run.skipped || 0 }}</p><p class="mt-0.5 text-[11px] text-white/40">无需更新</p></div>
          <div><p class="text-lg font-medium tabular-nums" :class="run.failed ? 'text-rose-300' : 'text-white/70'">{{ run.failed || 0 }}</p><p class="mt-0.5 text-[11px] text-white/40">处理失败</p></div>
        </div>
        <p v-if="run.errorMessage" class="mt-3 break-words rounded-lg bg-rose-400/10 p-3 text-xs leading-relaxed text-rose-200">{{ run.errorMessage }}</p>
        <details v-if="run.issues?.length" class="mt-3 rounded-lg border border-rose-300/15 bg-rose-300/5 p-3">
          <summary class="cursor-pointer text-xs text-rose-200">查看失败详情（{{ run.issues.length }}）</summary>
          <ul class="mt-3 max-h-64 space-y-3 overflow-y-auto">
            <li v-for="(issue, index) in run.issues.slice(0, issueLimit)" :key="index" class="text-xs">
              <p class="break-all text-white/70">{{ issue.sourcePath }}</p>
              <p class="mt-1 break-words text-rose-200/80">{{ stageLabels[issue.stage] || '文件处理' }}：{{ issue.reason }}</p>
            </li>
          </ul>
          <button v-if="run.issues.length > issueLimit" class="mt-3 text-xs text-cyan-200" @click="issueLimit += 20">再显示 20 条</button>
        </details>
        <p v-if="run.mediaRefreshError" class="mt-3 text-xs text-amber-200">媒体库刷新失败：{{ run.mediaRefreshError }}</p>
        <p v-if="run.notificationError" class="mt-3 text-xs text-amber-200">通知发送失败：{{ run.notificationError }}</p>
        <p v-if="actionError" role="alert" class="mt-3 text-xs text-rose-200">{{ actionError }}</p>
      </div>
      <details class="border-t border-white/5 px-4 py-3 sm:px-5" @toggle="onTrashToggle">
        <summary class="cursor-pointer text-xs text-white/45">回收站 <span class="text-white/25">· 保留 7 天</span></summary>
        <p v-if="trashLoading" class="mt-3 text-xs text-white/40">正在读取…</p>
        <p v-else-if="trashError" class="mt-3 text-xs text-rose-200">{{ trashError }}</p>
        <template v-else>
          <div v-for="item in trash" :key="item.id" class="mt-3 flex items-start justify-between gap-3 text-xs"><span class="min-w-0 break-all text-white/60">{{ item.path }}</span><button :disabled="active || restoring !== null" class="shrink-0 text-cyan-200 disabled:opacity-40" @click="restore(item.id)">{{ restoring === item.id ? '恢复中…' : '恢复' }}</button></div>
          <p v-if="trash.length === 0" class="mt-3 text-xs text-white/40">没有可恢复的文件</p>
        </template>
      </details>
    </template>
    <p v-if="pollError" role="status" class="border-t border-amber-200/10 bg-amber-200/5 px-4 py-3 text-xs text-amber-200">{{ pollError }}，正在自动重试。</p>
  </section>
</template>
<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { authenticatedApiCall } from '~/core/utils/api'
const props = defineProps({ taskId: { type: Number, required: true } })
const run = ref(null)
const pollError = ref('')
const actionError = ref('')
const cancelPending = ref(false)
const trash = ref([])
const trashLoading = ref(false)
const trashError = ref('')
const restoring = ref(null)
const issueLimit = ref(20)
const clock = ref(Date.now())
const active = computed(() => ['QUEUED', 'RUNNING'].includes(run.value?.status))
const completed = computed(() => ['SUCCESS', 'PARTIAL_SUCCESS'].includes(run.value?.status))
const labels = { QUEUED: '排队中', RUNNING: '执行中', SUCCESS: '已完成', PARTIAL_SUCCESS: '部分完成', FAILED: '执行失败', CANCELED: '已取消', INTERRUPTED: '已中断' }
const stageLabels = { DISCOVERY: '扫描文件', RENAME: '整理目录', PREPARE_OUTPUT: '准备输出', WRITE_STRM: '生成 STRM', METADATA: '刮削元数据', CLEANUP: '清理失效输出', SAVE_MANIFEST: '保存扫描结果', FINALIZE: '执行完成' }
const steps = ['扫描', '准备', '处理', '收尾']
const stage = computed(() => run.value?.failureStage || run.value?.stage)
const stepIndex = computed(() => ({ DISCOVERY: 0, RENAME: 1, PREPARE_OUTPUT: 1, WRITE_STRM: 2, METADATA: 2, CLEANUP: 3, SAVE_MANIFEST: 3, FINALIZE: 3 })[stage.value] ?? 0)
const progress = computed(() => Math.max(0, Math.min(100, Number(run.value?.progress) || 0)))
const indeterminate = computed(() => active.value && (run.value?.status === 'QUEUED' || ['DISCOVERY', 'RENAME', 'PREPARE_OUTPUT'].includes(stage.value)))
const headline = computed(() => {
  if (completed.value) return run.value.failed ? '处理已结束，部分文件需要关注' : '本次任务已完成'
  if (!active.value) return `停止于${stageLabels[stage.value] || '文件处理'}`
  if (cancelPending.value) return '正在停止请求并保存进度'
  return run.value.status === 'QUEUED' ? '等待开始执行' : stageLabels[stage.value] || '正在处理'
})
const statusTone = computed(() => run.value?.status === 'FAILED' ? 'text-rose-300' : run.value?.status === 'SUCCESS' ? 'text-emerald-300' : active.value ? 'text-cyan-200' : 'text-amber-200')
const barTone = computed(() => run.value?.status === 'FAILED' ? 'bg-rose-400' : run.value?.status === 'SUCCESS' ? 'bg-emerald-400' : active.value ? 'bg-gradient-to-r from-cyan-400 to-blue-400' : 'bg-amber-300')
const summary = computed(() => {
  if (active.value && run.value?.status === 'QUEUED') return '任务已提交，即将开始扫描。'
  if (active.value && stage.value === 'DISCOVERY') return '正在发现媒体文件，扫描完成后显示文件总数。'
  if (run.value?.total === undefined) return '尚未确定待处理文件总数。'
  const done = (run.value.processed || 0) + (run.value.skipped || 0) + (run.value.failed || 0)
  return `已检查 ${done} / ${run.value.total} 个文件${run.value.cleaned ? ` · 已回收 ${run.value.cleaned} 个失效输出` : ''}`
})
const elapsed = computed(() => {
  const start = run.value?.startedAtUnixMs || Date.parse(run.value?.startedAt || '')
  const duration = !active.value && run.value?.durationMs !== undefined ? run.value.durationMs : (active.value ? clock.value : Date.parse(run.value?.completedAt || '')) - start
  if (!Number.isFinite(duration)) return '—'
  const seconds = Math.max(0, Math.floor(duration / 1000))
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`
  return `${Math.floor(seconds / 3600)} 小时 ${Math.floor(seconds % 3600 / 60)} 分`
})
async function loadTrash() {
  trashLoading.value = true; trashError.value = ''
  try { const res = await authenticatedApiCall(`/task-config/${props.taskId}/trash/list`); if (res.code !== 200) throw Error(res.message); trash.value = res.data || [] }
  catch (e) { trashError.value = e.message || '读取回收站失败' }
  finally { trashLoading.value = false }
}
function onTrashToggle(event) { if (event.target.open) loadTrash() }
async function restore(id) {
  restoring.value = id; actionError.value = ''
  try { const res = await authenticatedApiCall(`/task-config/${props.taskId}/trash/${id}/restore`, { method: 'POST' }); if (res.code !== 200) throw Error(res.message); await loadTrash() }
  catch (e) { actionError.value = e.message || '恢复失败' }
  finally { restoring.value = null }
}
let timer, clockTimer
let stopped = false
async function poll() {
  try {
    const res = await authenticatedApiCall(`/task-config/${props.taskId}/runs/latest`)
    if (res.code !== 200) throw Error(res.message)
    if (!stopped) {
      if (res.data?.id !== run.value?.id) { cancelPending.value = false; actionError.value = ''; issueLimit.value = 20 }
      run.value = res.data; pollError.value = ''
      if (!active.value) cancelPending.value = false
    }
  } catch { if (!stopped) pollError.value = '暂时无法获取最新进度' }
  if (!stopped) timer = setTimeout(poll, 2000)
}
async function cancel() {
  cancelPending.value = true; actionError.value = ''
  try { const res = await authenticatedApiCall(`/task-config/${props.taskId}/cancel`, { method: 'POST' }); if (res.code !== 200) throw Error(res.message) }
  catch (e) { cancelPending.value = false; actionError.value = e.message || '取消请求失败' }
}
onMounted(() => { poll(); clockTimer = setInterval(() => { clock.value = Date.now() }, 1000) })
onBeforeUnmount(() => { stopped = true; clearTimeout(timer); clearInterval(clockTimer) })
</script>
<style scoped>
.progress-scanning { animation: scanning 1.8s ease-in-out infinite; }
@keyframes scanning { from { transform: translateX(-100%); } to { transform: translateX(390%); } }
@media (prefers-reduced-motion: reduce) { .progress-scanning { animation: none; } }
</style>
