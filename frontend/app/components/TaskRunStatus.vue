<template>
  <div v-if="run" class="mt-4 rounded-xl border border-white/10 p-3 text-sm" aria-live="polite">
    <div class="flex justify-between gap-3">
      <span>{{ labels[run.status] || run.status }} · {{ run.stage }}</span>
      <button v-if="active" class="text-amber-300" @click="cancel">取消任务</button>
    </div>
    <progress class="mt-2 w-full" :value="run.progress || 0" max="100" />
    <p class="mt-1 text-white/50">处理 {{ run.processed || 0 }} · 跳过 {{ run.skipped || 0 }} · 失败 {{ run.failed || 0 }}</p>
    <details class="mt-2"><summary @click="loadTrash" class="cursor-pointer text-white/60">隔离文件（保留 7 天）</summary>
      <div v-for="item in trash" :key="item.id" class="mt-2 flex justify-between gap-3"><span class="break-all">{{ item.path }}</span><button :disabled="active" class="text-blue-300" @click="restore(item.id)">恢复</button></div>
      <p v-if="trash.length === 0" class="text-white/40">没有可恢复的文件</p>
    </details>
    <p v-if="run.errorMessage || error" class="mt-1 text-red-300">{{ run.errorMessage || error }}</p>
  </div>
</template>
<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { authenticatedApiCall } from '~/core/utils/api'
const props = defineProps({ taskId: { type: Number, required: true } })
const run = ref(null)
const error = ref('')
const trash = ref([])
async function loadTrash() { try { const r = await authenticatedApiCall(`/task-config/${props.taskId}/trash/list`); if(r.code === 200) trash.value = r.data } catch { error.value = '读取隔离记录失败' } }
async function restore(id) { try { const r = await authenticatedApiCall(`/task-config/${props.taskId}/trash/${id}/restore`, {method: 'POST'}); if(r.code !== 200) throw Error(r.message); await loadTrash() } catch(e) { error.value = e.message || '恢复失败' } }
const active = computed(() => ['QUEUED', 'RUNNING'].includes(run.value?.status))
const labels = { QUEUED: '排队中', RUNNING: '执行中', SUCCESS: '已完成', PARTIAL_SUCCESS: '部分完成', FAILED: '失败', CANCELED: '已取消', INTERRUPTED: '已中断' }
let timer
let stopped = false
async function poll() {
  try {
    const res = await authenticatedApiCall(`/task-config/${props.taskId}/runs/latest`)
    if (res.code === 200) { run.value = res.data; error.value = '' }
  } catch { error.value = '暂时无法获取任务状态' }
  if (!stopped) timer = setTimeout(poll, 2000)
}
async function cancel() {
  try { await authenticatedApiCall(`/task-config/${props.taskId}/cancel`, { method: 'POST' }) }
  catch { error.value = '取消请求失败' }
}
onMounted(poll)
onBeforeUnmount(() => { stopped = true; clearTimeout(timer) })
</script>
