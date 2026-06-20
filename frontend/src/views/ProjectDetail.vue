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
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { apiDefAPI, activityAPI } from '../api'

const route = useRoute()
const router = useRouter()
const projectId = route.params.id

const project = ref(null)
const apis = ref([])
const activities = ref([])
const mockBaseUrl = window.location.origin

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
</style>
