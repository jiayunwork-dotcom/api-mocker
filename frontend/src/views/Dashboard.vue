<template>
  <div class="layout">
    <header class="top-bar">
      <div class="brand" @click="$router.push('/')">API Mocker</div>
      <div class="user-area">
        <span class="user-name">{{ userStore.user?.name }}</span>
        <el-button text @click="handleLogout">退出</el-button>
      </div>
    </header>

    <div class="page-container">
      <div class="page-header">
        <h2>我的工作空间</h2>
        <el-button type="primary" @click="showCreateDialog = true">新建工作空间</el-button>
      </div>

      <div class="workspace-grid">
        <div v-for="ws in workspaces" :key="ws.id" class="workspace-card card" @click="$router.push(`/workspace/${ws.id}`)">
          <h3>{{ ws.name }}</h3>
          <p class="desc">{{ ws.description || '暂无描述' }}</p>
          <div class="meta">
            <span>角色: {{ ws.role || '管理员' }}</span>
          </div>
        </div>
      </div>

      <el-empty v-if="!workspaces.length" description="暂无工作空间，点击右上角创建" />
    </div>

    <el-dialog v-model="showCreateDialog" title="新建工作空间" width="480px">
      <el-form :model="createForm" label-position="top">
        <el-form-item label="名称">
          <el-input v-model="createForm.name" placeholder="工作空间名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createForm.description" type="textarea" :rows="3" placeholder="描述信息" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="createWorkspace" :loading="creating">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { workspaceAPI } from '../api'
import { useUserStore } from '../stores/user'

const router = useRouter()
const userStore = useUserStore()
const workspaces = ref([])
const showCreateDialog = ref(false)
const creating = ref(false)
const createForm = ref({ name: '', description: '' })

async function loadWorkspaces() {
  try {
    const res = await workspaceAPI.list()
    workspaces.value = res.workspaces || []
  } catch {}
}

async function createWorkspace() {
  if (!createForm.value.name) {
    ElMessage.warning('请输入名称')
    return
  }
  creating.value = true
  try {
    await workspaceAPI.create(createForm.value)
    ElMessage.success('创建成功')
    showCreateDialog.value = false
    createForm.value = { name: '', description: '' }
    loadWorkspaces()
  } finally {
    creating.value = false
  }
}

function handleLogout() {
  userStore.logout()
  router.push('/login')
}

onMounted(loadWorkspaces)
</script>

<style scoped>
.layout {
  min-height: 100vh;
}

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

.user-area {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-name {
  color: #ccc;
}

.workspace-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.workspace-card {
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.workspace-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.workspace-card h3 {
  font-size: 18px;
  margin-bottom: 8px;
  color: #1a1a2e;
}

.workspace-card .desc {
  color: #888;
  font-size: 14px;
  margin-bottom: 12px;
}

.workspace-card .meta {
  color: #aaa;
  font-size: 12px;
}
</style>
