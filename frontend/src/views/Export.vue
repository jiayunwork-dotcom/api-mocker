<template>
  <div class="layout">
    <header class="top-bar">
      <div class="brand" @click="$router.push('/')">API Mocker</div>
      <el-button text style="color:#fff" @click="$router.push(`/project/${projectId}`)">返回项目</el-button>
    </header>

    <div class="page-container">
      <div class="page-header">
        <h2>导出</h2>
      </div>

      <div class="export-grid">
        <div class="export-card card">
          <h3>OpenAPI 3.0</h3>
          <p>导出为标准OpenAPI 3.0格式的JSON文件，可导入Swagger、Postman等工具</p>
          <el-button type="primary" @click="exportOpenAPI" :loading="exporting.openapi">导出 OpenAPI JSON</el-button>
        </div>

        <div class="export-card card">
          <h3>Markdown 文档</h3>
          <p>导出为Markdown格式的接口文档，适合放在项目Wiki中</p>
          <el-button type="primary" @click="exportMarkdown" :loading="exporting.markdown">导出 Markdown</el-button>
        </div>

        <div class="export-card card">
          <h3>cURL 命令</h3>
          <p>为单个接口生成cURL测试命令</p>
          <el-select v-model="selectedApiId" placeholder="选择接口" style="width:100%;margin-bottom:12px">
            <el-option v-for="api in apis" :key="api.id" :label="`${api.method} ${api.path}`" :value="api.id" />
          </el-select>
          <el-button type="primary" @click="generateCurl" :loading="exporting.curl" :disabled="!selectedApiId">生成 cURL</el-button>
        </div>
      </div>

      <div v-if="exportResult" class="card result-card">
        <div class="result-header">
          <span>{{ exportResultType }}</span>
          <div>
            <el-button size="small" @click="copyResult">复制</el-button>
            <el-button size="small" @click="downloadResult">下载</el-button>
          </div>
        </div>
        <pre class="result-block">{{ exportResult }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiDefAPI, exportAPI } from '../api'

const route = useRoute()
const projectId = route.params.projectId

const apis = ref([])
const selectedApiId = ref('')
const exportResult = ref('')
const exportResultType = ref('')
const exporting = reactive({ openapi: false, markdown: false, curl: false })

async function loadApis() {
  try {
    const res = await apiDefAPI.list(projectId)
    apis.value = res.apis || []
  } catch {}
}

async function exportOpenAPI() {
  exporting.openapi = true
  try {
    const res = await exportAPI.openapi({ projectId })
    const json = JSON.stringify(res, null, 2)
    exportResult.value = json
    exportResultType.value = 'OpenAPI 3.0 JSON'
  } finally { exporting.openapi = false }
}

async function exportMarkdown() {
  exporting.markdown = true
  try {
    const res = await exportAPI.markdown({ projectId })
    exportResult.value = res.markdown
    exportResultType.value = 'Markdown 文档'
  } finally { exporting.markdown = false }
}

async function generateCurl() {
  if (!selectedApiId.value) return
  exporting.curl = true
  try {
    const res = await exportAPI.curl({ apiId: selectedApiId.value })
    exportResult.value = res.curl
    exportResultType.value = 'cURL 命令'
  } finally { exporting.curl = false }
}

function copyResult() {
  navigator.clipboard.writeText(exportResult.value)
  ElMessage.success('已复制')
}

function downloadResult() {
  const extensions = { 'OpenAPI 3.0 JSON': 'json', 'Markdown 文档': 'md', 'cURL 命令': 'sh' }
  const ext = extensions[exportResultType.value] || 'txt'
  const blob = new Blob([exportResult.value], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `api_export.${ext}`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(loadApis)
</script>

<style scoped>
.top-bar {
  background: #1a1a2e;
  color: #fff;
  padding: 0 24px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  font-size: 20px;
  font-weight: 700;
  cursor: pointer;
}

.export-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.export-card h3 {
  font-size: 18px;
  margin-bottom: 8px;
  color: #1a1a2e;
}

.export-card p {
  color: #888;
  font-size: 14px;
  margin-bottom: 16px;
}

.result-card {
  margin-top: 24px;
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-weight: 600;
}

.result-block {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
  font-size: 13px;
  line-height: 1.6;
  max-height: 500px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-wrap: break-word;
}
</style>
