<template>
  <div class="layout">
    <header class="top-bar">
      <div class="brand" @click="$router.push('/')">API Mocker</div>
      <el-button text style="color:#fff" @click="$router.push(`/project/${projectId}`)">返回项目</el-button>
    </header>

    <div class="page-container">
      <div class="page-header">
        <h2>代码生成</h2>
      </div>

      <div class="card">
        <el-form label-position="top">
          <el-form-item label="选择接口">
            <el-checkbox-group v-model="selectedApis">
              <el-checkbox v-for="api in apis" :key="api.id" :value="api.id">
                <span :class="['method-badge', `method-${api.method.toLowerCase()}`]">{{ api.method }}</span>
                {{ api.path }}
              </el-checkbox>
            </el-checkbox-group>
          </el-form-item>

          <el-form-item label="目标语言">
            <el-radio-group v-model="language">
              <el-radio value="typescript">TypeScript</el-radio>
              <el-radio value="go">Go</el-radio>
              <el-radio value="python">Python (Pydantic)</el-radio>
            </el-radio-group>
          </el-form-item>

          <el-button type="primary" @click="generateCode" :loading="generating" :disabled="!selectedApis.length">
            生成代码
          </el-button>
        </el-form>
      </div>

      <div v-if="generatedCode" class="card">
        <div class="code-header">
          <span>生成结果 - {{ language }}</span>
          <div>
            <el-button size="small" @click="copyCode">复制代码</el-button>
            <el-button size="small" @click="downloadCode">下载文件</el-button>
          </div>
        </div>
        <pre class="code-block"><code>{{ generatedCode }}</code></pre>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiDefAPI, codegenAPI } from '../api'

const route = useRoute()
const projectId = route.params.projectId

const apis = ref([])
const selectedApis = ref([])
const language = ref('typescript')
const generating = ref(false)
const generatedCode = ref('')

async function loadApis() {
  try {
    const res = await apiDefAPI.list(projectId)
    apis.value = res.apis || []
  } catch {}
}

async function generateCode() {
  if (!selectedApis.value.length) { ElMessage.warning('请选择接口'); return }
  generating.value = true
  try {
    const res = await codegenAPI.generate({
      apiIds: selectedApis.value,
      language: language.value
    })
    generatedCode.value = res.code
  } finally { generating.value = false }
}

function copyCode() {
  navigator.clipboard.writeText(generatedCode.value)
  ElMessage.success('已复制到剪贴板')
}

function downloadCode() {
  const extensions = { typescript: 'ts', go: 'go', python: 'py' }
  const ext = extensions[language.value] || 'txt'
  const blob = new Blob([generatedCode.value], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `api_types.${ext}`
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

.code-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-weight: 600;
}

.code-block {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
  font-size: 13px;
  line-height: 1.6;
}

.code-block code {
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
}
</style>
