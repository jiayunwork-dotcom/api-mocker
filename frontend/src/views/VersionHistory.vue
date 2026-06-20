<template>
  <div class="layout">
    <header class="top-bar">
      <div class="brand" @click="$router.push('/')">API Mocker</div>
      <el-button text style="color:#fff" @click="$router.push(`/project/${projectId}`)">返回项目</el-button>
    </header>

    <div class="page-container">
      <div class="page-header">
        <h2>版本历史</h2>
      </div>

      <el-timeline>
        <el-timeline-item
          v-for="ver in versions"
          :key="ver.id"
          :timestamp="formatTime(ver.created_at)"
          placement="top"
          :type="ver.is_breaking ? 'danger' : 'primary'"
        >
          <div class="version-card card">
            <div class="version-header">
              <span class="version-num">v{{ ver.version }}</span>
              <el-tag v-if="ver.is_breaking" type="danger" size="small">Breaking Change</el-tag>
              <span class="changer">by {{ ver.changer_name }}</span>
              <div class="version-actions">
                <el-button size="small" @click="viewDiff(ver)">查看Diff</el-button>
                <el-button size="small" type="warning" @click="rollback(ver)">回滚到此版本</el-button>
              </div>
            </div>
            <p class="summary">{{ ver.change_summary || '无变更记录' }}</p>
          </div>
        </el-timeline-item>
      </el-timeline>

      <el-empty v-if="!versions.length" description="暂无版本记录" />

      <el-dialog v-model="showDiffDialog" title="版本对比" width="700px">
        <div v-if="diffData" class="diff-container">
          <div class="diff-meta">
            <span>从 v{{ diffData.fromVersion }} 到 v{{ diffData.toVersion }}</span>
            <el-tag v-if="diffData.isBreaking" type="danger" size="small">Breaking Change</el-tag>
          </div>
          <div v-for="(diff, idx) in diffData.diffs" :key="idx" :class="['diff-item', `diff-${diff.type}`]">
            <span class="diff-type-badge">{{ diffTypeLabel(diff.type) }}</span>
            <span class="diff-field">{{ diff.field }}</span>
            <span v-if="diff.oldValue" class="diff-old">{{ formatVal(diff.oldValue) }}</span>
            <span v-if="diff.newValue" class="diff-new">{{ formatVal(diff.newValue) }}</span>
          </div>
          <el-empty v-if="!diffData.diffs?.length" description="无差异" />
        </div>
      </el-dialog>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { versionAPI } from '../api'

const route = useRoute()
const router = useRouter()
const projectId = route.params.projectId
const apiId = route.params.apiId

const versions = ref([])
const showDiffDialog = ref(false)
const diffData = ref(null)

async function loadVersions() {
  try {
    const res = await versionAPI.list(projectId, apiId)
    versions.value = res.versions || []
  } catch {}
}

async function viewDiff(ver) {
  try {
    const res = await versionAPI.diff(projectId, apiId, ver.id)
    diffData.value = res
    showDiffDialog.value = true
  } catch {}
}

async function rollback(ver) {
  try {
    await ElMessageBox.confirm(`确定回滚到 v${ver.version}?`, '确认回滚')
    await versionAPI.rollback(projectId, apiId, ver.id)
    ElMessage.success('已回滚')
    loadVersions()
  } catch {}
}

function diffTypeLabel(type) {
  return { added: '新增', removed: '删除', modified: '修改' }[type] || type
}

function formatVal(val) {
  if (typeof val === 'object') return JSON.stringify(val)
  return String(val)
}

function formatTime(t) {
  return new Date(t).toLocaleString('zh-CN')
}

onMounted(loadVersions)
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

.version-card {
  padding: 12px 16px;
}

.version-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.version-num {
  font-weight: 700;
  font-size: 16px;
  color: #1a1a2e;
}

.changer {
  color: #999;
  font-size: 13px;
}

.version-actions {
  margin-left: auto;
}

.summary {
  color: #666;
  font-size: 13px;
  margin-top: 4px;
}

.diff-container {
  max-height: 500px;
  overflow-y: auto;
}

.diff-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  font-weight: 600;
}

.diff-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 4px;
  margin-bottom: 4px;
}

.diff-added { background: #f0f9eb; }
.diff-removed { background: #fef0f0; }
.diff-modified { background: #fdf6ec; }

.diff-type-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  min-width: 40px;
  text-align: center;
}

.diff-added .diff-type-badge { background: #e1f3d8; color: #67c23a; }
.diff-removed .diff-type-badge { background: #fde2e2; color: #f56c6c; }
.diff-modified .diff-type-badge { background: #faecd8; color: #e6a23c; }

.diff-field {
  font-family: 'SF Mono', monospace;
  font-weight: 600;
}

.diff-old {
  color: #f56c6c;
  text-decoration: line-through;
  font-size: 13px;
}

.diff-new {
  color: #67c23a;
  font-size: 13px;
}
</style>
