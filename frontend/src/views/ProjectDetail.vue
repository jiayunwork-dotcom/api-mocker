<template>
  <div class="layout">
    <header class="top-bar">
      <div class="brand" @click="$router.push('/')">API Mocker</div>
      <div class="nav-links">
        <el-button text style="color:#fff" @click="$router.push('/')">工作空间</el-button>
      </div>
    </header>

    <div class="page-container" v-if="project">
      <div class="page-header">
        <div>
          <h2>{{ project.name }}</h2>
          <p class="desc">{{ project.description }}</p>
        </div>
        <div class="actions">
          <el-button @click="$router.push(`/project/${projectId}/models`)">公共模型</el-button>
          <el-button @click="$router.push(`/project/${projectId}/codegen`)">代码生成</el-button>
          <el-button @click="$router.push(`/project/${projectId}/export`)">导出</el-button>
          <el-button @click="showImportDialog = true">导入</el-button>
          <el-button type="primary" @click="createApi">新建接口</el-button>
        </div>
      </div>

      <div class="mock-url-hint card">
        <strong>Mock Base URL:</strong>
        <code>{{ mockBaseUrl }}/mock/{{ projectId }}</code>
        <el-button size="small" text @click="copyMockUrl">复制</el-button>
      </div>

      <div class="api-list">
        <div v-for="api in apis" :key="api.id" class="api-item card" @click="editApi(api)">
          <div class="api-left">
            <span :class="['method-badge', `method-${api.method.toLowerCase()}`]">{{ api.method }}</span>
            <span class="api-path">{{ api.path }}</span>
            <span class="api-desc">{{ api.description }}</span>
          </div>
          <div class="api-right">
            <el-tag v-for="tag in api.tags" :key="tag" size="small" style="margin-left:4px">{{ tag }}</el-tag>
            <el-button size="small" type="danger" text @click.stop="deleteApi(api)">删除</el-button>
          </div>
        </div>
        <el-empty v-if="!apis.length" description="暂无接口，点击右上角创建" />
      </div>

      <div class="section-title">项目动态</div>
      <el-timeline>
        <el-timeline-item
          v-for="act in activities"
          :key="act.id"
          :timestamp="formatTime(act.created_at)"
          placement="top"
          :type="act.is_breaking ? 'danger' : 'primary'"
        >
          <div :class="{ 'breaking-change': act.is_breaking }">
            <strong>{{ act.changer_name }}</strong>
            {{ act.change_summary }}
            <el-tag v-if="act.is_breaking" type="danger" size="small" style="margin-left:8px">Breaking Change</el-tag>
          </div>
        </el-timeline-item>
      </el-timeline>
    </div>

    <el-dialog
      v-model="showImportDialog"
      title="批量导入接口"
      width="700px"
      :close-on-click-modal="false"
    >
      <el-tabs v-model="importTab" type="border-card">
        <el-tab-pane label="粘贴JSON" name="paste">
          <div class="paste-area">
            <el-input
              v-model="pasteContent"
              type="textarea"
              :rows="16"
              placeholder="请粘贴 OpenAPI 3.0 格式的 JSON 内容..."
              resize="none"
            />
            <div class="hint">
              <el-icon><InfoFilled /></el-icon>
              <span>支持 OpenAPI 3.0.x 格式，系统将自动解析 paths 对象中的所有接口定义</span>
            </div>
          </div>
        </el-tab-pane>
        <el-tab-pane label="上传文件" name="upload">
          <div class="upload-area">
            <el-upload
              ref="uploadRef"
              class="upload-dragger"
              drag
              :auto-upload="false"
              :on-change="handleFileChange"
              :on-exceed="handleFileExceed"
              :limit="1"
              accept=".json"
              :file-list="fileList"
            >
              <el-icon class="upload-icon"><UploadFilled /></el-icon>
              <div class="upload-text">将文件拖到此处，或<em>点击选择文件</em></div>
              <div class="upload-hint">仅支持 .json 格式文件，文件大小不超过 2MB</div>
            </el-upload>
            <div v-if="selectedFileName" class="selected-file">
              <el-icon><Document /></el-icon>
              <span>{{ selectedFileName }}</span>
              <el-button link type="danger" @click="clearFile">移除</el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <div v-if="importError" class="error-alert">
        <el-alert :title="importError" type="error" :closable="false" show-icon />
      </div>

      <div v-if="importResult" class="import-result">
        <el-alert type="success" :closable="false" show-icon>
          <template #title>
            <div class="result-summary">
              <span>导入完成：</span>
              <el-tag type="success">成功 {{ importResult.success }} 条</el-tag>
              <el-tag type="warning" v-if="importResult.skipped > 0">跳过 {{ importResult.skipped }} 条</el-tag>
              <el-tag type="danger" v-if="importResult.failed > 0">失败 {{ importResult.failed }} 条</el-tag>
            </div>
          </template>
        </el-alert>
        <el-table :data="importResult.items" style="margin-top: 16px" size="small" max-height="300">
          <el-table-column prop="method" label="方法" width="80">
            <template #default="{ row }">
              <el-tag :type="getMethodTagType(row.method)" size="small">{{ row.method }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="path" label="路径" min-width="200" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag v-if="row.status === 'success'" type="success" size="small">成功</el-tag>
              <el-tag v-else-if="row.status === 'skipped'" type="warning" size="small">跳过</el-tag>
              <el-tag v-else type="danger" size="small">失败</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="error" label="备注" min-width="150">
            <template #default="{ row }">
              <span v-if="row.error" class="error-text">{{ row.error }}</span>
              <span v-else>-</span>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <template #footer>
        <el-button @click="closeImportDialog">关闭</el-button>
        <el-button type="primary" :loading="importing" @click="doImport">
          {{ importResult ? '重新导入' : '确认导入' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, UploadFilled, Document } from '@element-plus/icons-vue'
import { apiDefAPI, activityAPI } from '../api'

const route = useRoute()
const router = useRouter()
const projectId = route.params.id

const project = ref(null)
const apis = ref([])
const activities = ref([])
const mockBaseUrl = window.location.origin

const showImportDialog = ref(false)
const importTab = ref('paste')
const pasteContent = ref('')
const importing = ref(false)
const importError = ref('')
const importResult = ref(null)
const uploadRef = ref(null)
const fileList = ref([])
const selectedFile = ref(null)
const selectedFileName = ref('')

const MAX_FILE_SIZE = 2 * 1024 * 1024

async function loadProject() {
  project.value = { id: projectId, name: '加载中...', description: '' }
}

async function loadApis() {
  try {
    const res = await apiDefAPI.list(projectId)
    apis.value = res.apis || []
  } catch {}
}

async function loadActivities() {
  try {
    const res = await activityAPI.list(projectId, { page: 1, pageSize: 20 })
    activities.value = res.activities || []
  } catch {}
}

function createApi() {
  router.push(`/project/${projectId}/api/new`)
}

function editApi(api) {
  router.push(`/project/${projectId}/api/${api.id}`)
}

async function deleteApi(api) {
  try {
    await ElMessageBox.confirm(`确定删除 ${api.method} ${api.path}?`, '确认')
    await apiDefAPI.delete(projectId, api.id)
    ElMessage.success('已删除')
    loadApis()
  } catch {}
}

function copyMockUrl() {
  navigator.clipboard.writeText(`${mockBaseUrl}/mock/${projectId}`)
  ElMessage.success('已复制')
}

function formatTime(t) {
  return new Date(t).toLocaleString('zh-CN')
}

function closeImportDialog() {
  showImportDialog.value = false
  pasteContent.value = ''
  importError.value = ''
  importResult.value = null
  clearFile()
}

function handleFileChange(uploadFile) {
  if (uploadFile.raw.size > MAX_FILE_SIZE) {
    ElMessage.error('文件大小不能超过 2MB')
    clearFile()
    return
  }
  if (!uploadFile.name.endsWith('.json')) {
    ElMessage.error('仅支持 .json 格式文件')
    clearFile()
    return
  }
  selectedFile.value = uploadFile.raw
  selectedFileName.value = uploadFile.name
  fileList.value = [uploadFile]
}

function handleFileExceed() {
  ElMessage.warning('只能上传一个文件')
}

function clearFile() {
  selectedFile.value = null
  selectedFileName.value = ''
  fileList.value = []
  if (uploadRef.value) {
    uploadRef.value.clearFiles()
  }
}

function getMethodTagType(method) {
  const map = {
    GET: 'success',
    POST: 'primary',
    PUT: 'warning',
    PATCH: '',
    DELETE: 'danger',
    HEAD: 'info',
    OPTIONS: 'info'
  }
  return map[method] || ''
}

async function doImport() {
  importError.value = ''
  importResult.value = null
  importing.value = true

  try {
    let res
    if (importTab.value === 'paste') {
      if (!pasteContent.value.trim()) {
        importError.value = '请输入 OpenAPI JSON 内容'
        importing.value = false
        return
      }
      res = await apiDefAPI.import(projectId, { content: pasteContent.value })
    } else {
      if (!selectedFile.value) {
        importError.value = '请选择要上传的文件'
        importing.value = false
        return
      }
      res = await apiDefAPI.importFile(projectId, selectedFile.value)
    }

    importResult.value = res.result

    if (res.result.success > 0) {
      ElMessage.success(`成功导入 ${res.result.success} 条接口`)
      loadApis()
      loadActivities()
    } else if (res.result.skipped > 0 && res.result.failed === 0) {
      ElMessage.warning(`所有接口均已存在，共跳过 ${res.result.skipped} 条`)
    } else if (res.result.failed > 0) {
      ElMessage.error(`导入失败 ${res.result.failed} 条，请查看详情`)
    }
  } catch (err) {
    if (err.response && err.response.data && err.response.data.error) {
      importError.value = err.response.data.error
    } else {
      importError.value = '导入失败，请检查网络连接或稍后重试'
    }
  } finally {
    importing.value = false
  }
}

onMounted(() => { loadProject(); loadApis(); loadActivities() })
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

.desc {
  color: #888;
  font-size: 14px;
  margin-top: 4px;
}

.actions {
  display: flex;
  gap: 8px;
}

.mock-url-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: #f0f9eb;
  border: 1px solid #e1f3d8;
}

.mock-url-hint code {
  background: #e8f5e9;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 14px;
}

.api-list {
  margin-top: 16px;
}

.api-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  padding: 12px 16px;
  transition: background 0.2s;
}

.api-item:hover {
  background: #f5f7fa;
}

.api-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.api-path {
  font-weight: 600;
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.api-desc {
  color: #888;
  font-size: 13px;
}

.api-right {
  display: flex;
  align-items: center;
  gap: 4px;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  margin: 32px 0 16px;
  color: #1a1a2e;
}

.breaking-change {
  color: #f56c6c;
  font-weight: 500;
}

.paste-area {
  padding: 16px 0;
}

.paste-area .hint {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
  color: #909399;
  font-size: 13px;
}

.upload-area {
  padding: 16px 0;
}

.upload-icon {
  font-size: 67px;
  color: #409eff;
}

.upload-text {
  font-size: 14px;
  color: #606266;
  margin: 8px 0;
}

.upload-text em {
  color: #409eff;
  font-style: normal;
}

.upload-hint {
  font-size: 12px;
  color: #909399;
}

.selected-file {
  margin-top: 16px;
  padding: 12px 16px;
  background: #f5f7fa;
  border-radius: 4px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.selected-file span {
  flex: 1;
  color: #606266;
}

.error-alert {
  margin-top: 16px;
}

.import-result {
  margin-top: 16px;
}

.result-summary {
  display: flex;
  align-items: center;
  gap: 8px;
}

.result-summary .el-tag {
  margin-left: 4px;
}

.error-text {
  color: #f56c6c;
  font-size: 12px;
}
</style>
