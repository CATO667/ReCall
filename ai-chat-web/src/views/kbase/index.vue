<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NInput, NSpin, NTag, useMessage } from 'naive-ui'

interface DocMeta {
  id: string
  title: string
  source_path?: string
  chunk_count: number
}

interface QAItem {
  question: string
  answer: string
  topic_name?: string
  created_at: number
}

interface TopicMeta {
  name: string
  qa_count: number
}

const router = useRouter()
const ms = useMessage()
const loading = ref(false)
const docs = ref<DocMeta[]>([])
const qaItems = ref<QAItem[]>([])
const topics = ref<TopicMeta[]>([])
const ingestPath = ref('data/kbase/samples/mykvstore-intro.md')
const topicFilter = ref('')

async function loadDocs() {
  const res = await fetch('/api/kbase/documents')
  const data = await res.json()
  if (!res.ok)
    throw new Error(data.error || '加载文档失败')
  docs.value = data.documents || []
}

async function loadTopics() {
  const res = await fetch('/api/kbase/topics')
  const data = await res.json()
  if (!res.ok)
    return
  topics.value = data.topics || []
}

async function loadQA() {
  const q = topicFilter.value.trim()
  const url = q ? `/api/kbase/qa?topic=${encodeURIComponent(q)}` : '/api/kbase/qa'
  const res = await fetch(url)
  const data = await res.json()
  if (!res.ok)
    throw new Error(data.error || '加载问答失败')
  qaItems.value = data.items || []
}

async function refreshAll() {
  loading.value = true
  try {
    await Promise.all([loadDocs(), loadQA()])
  }
  catch (e: any) {
    ms.error(e.message || '加载失败')
  }
  finally {
    loading.value = false
  }
}

async function ingestDoc(force: boolean) {
  if (!ingestPath.value.trim()) {
    ms.warning('请输入文档路径')
    return
  }
  loading.value = true
  try {
    const res = await fetch('/api/kbase/ingest', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: ingestPath.value.trim(), force }),
    })
    const data = await res.json()
    if (!res.ok)
      throw new Error(data.error || '导入失败')
    ms.success(data.skipped ? '文档已存在，已跳过' : `导入成功 (${data.chunks} chunks)`)
    await loadDocs()
  }
  catch (e: any) {
    ms.error(e.message || '导入失败')
  }
  finally {
    loading.value = false
  }
}

function exportQA(byTopic: boolean) {
  const url = `/api/kbase/export/qa?format=markdown${byTopic ? '&by_topic=1' : ''}`
  window.open(url, '_blank')
}

function goChat() {
  router.push({ name: 'Chat' })
}

function formatTime(ts: number) {
  if (!ts)
    return '-'
  return new Date(ts * 1000).toLocaleString()
}

onMounted(refreshAll)
</script>

<template>
  <div class="h-full flex flex-col bg-white dark:bg-[#24272e]">
    <header class="flex items-center justify-between px-4 h-14 border-b dark:border-neutral-800">
      <div class="flex items-center gap-3">
        <NButton quaternary @click="goChat">
          ← 返回聊天
        </NButton>
        <h1 class="text-lg font-medium">
          知识库
        </h1>
      </div>
      <NButton :loading="loading" @click="refreshAll">
        刷新
      </NButton>
    </header>

    <NSpin :show="loading" class="flex-1 overflow-auto">
      <div class="max-w-5xl mx-auto p-4 space-y-8">
        <section>
          <h2 class="text-base font-semibold mb-3">
            文档
          </h2>
          <div class="flex flex-wrap gap-2 mb-3">
            <NInput v-model:value="ingestPath" placeholder="文档路径" style="width: 360px" />
            <NButton type="primary" @click="ingestDoc(false)">
              导入
            </NButton>
            <NButton @click="ingestDoc(true)">
              强制重建
            </NButton>
          </div>
          <div v-if="docs.length === 0" class="text-gray-500 text-sm">
            暂无文档
          </div>
          <div v-else class="space-y-2">
            <div
              v-for="doc in docs"
              :key="doc.id"
              class="p-3 rounded border dark:border-neutral-700 text-sm"
            >
              <div class="font-medium">
                {{ doc.title }}
              </div>
              <div class="text-gray-500 mt-1">
                {{ doc.chunk_count }} chunks · {{ doc.source_path || doc.id }}
              </div>
            </div>
          </div>
        </section>

        <section>
          <div class="flex items-center justify-between mb-3">
            <h2 class="text-base font-semibold">
              历史问答
            </h2>
            <div class="flex gap-2">
              <NButton size="small" @click="exportQA(false)">
                导出 Markdown
              </NButton>
              <NButton size="small" @click="exportQA(true)">
                按主题导出
              </NButton>
            </div>
          </div>
          <div class="flex flex-wrap gap-2 mb-3">
            <NInput v-model:value="topicFilter" placeholder="按主题过滤" style="width: 240px" />
            <NButton @click="loadQA">
              筛选
            </NButton>
          </div>
          <div class="flex flex-wrap gap-2 mb-4">
            <NTag v-for="t in topics" :key="t.name" size="small" :bordered="false">
              {{ t.name }} ({{ t.qa_count }})
            </NTag>
          </div>
          <div v-if="qaItems.length === 0" class="text-gray-500 text-sm">
            暂无问答，去聊天页提问后会自动沉淀
          </div>
          <div v-else class="space-y-3">
            <div
              v-for="(item, idx) in qaItems"
              :key="idx"
              class="p-4 rounded border dark:border-neutral-700"
            >
              <div class="font-medium mb-1">
                {{ item.question }}
              </div>
              <div class="text-xs text-gray-500 mb-2">
                {{ item.topic_name || '未分类' }} · {{ formatTime(item.created_at) }}
              </div>
              <pre class="whitespace-pre-wrap text-sm text-gray-700 dark:text-gray-300">{{ item.answer }}</pre>
            </div>
          </div>
        </section>
      </div>
    </NSpin>
  </div>
</template>
