<template>
  <div class="layout">
    <header class="top-bar">
      <div class="brand" @click="$router.push('/')">API Mocker</div>
      <el-button text style="color:#fff" @click="$router.push('/')">返回</el-button>
    </header>

    <div class="page-container">
      <div class="page-header" v-if="workspace">
        <div>
          <h2>{{ workspace.name }}</h2>
          <p class="desc">{{ workspace.description }}</p>
        </div>
        <div class="actions">
          <el-button @click="showInviteDialog = true">邀请成员</el-button>
          <el-button type="primary" @click="showCreateDialog = true">新建项目</el-button>
        </div>
      </div>

      <el-tabs v-model="activeTab">
        <el-tab-pane label="项目列表" name="projects">
          <div class="project-grid">
            <div v-for="proj in projects" :key="proj.id" class="project-card card" @click="$router.push(`/project/${proj.id}`)">
              <h3>{{ proj.name }}</h3>
              <p class="desc">{{ proj.description || '暂无描述' }}</p>
              <div class="meta">
                <span>Base Path: {{ proj.base_path || '/' }}</span>
              </div>
            </div>
          </div>
          <el-empty v-if="!projects.length" description="暂无项目" />
        </el-tab-pane>

        <el-tab-pane label="成员管理" name="members">
          <el-table :data="members" stripe>
            <el-table-column prop="user_name" label="姓名" />
            <el-table-column prop="user_email" label="邮箱" />
            <el-table-column prop="role" label="角色" width="120">
              <template #default="{ row }">
                <el-tag :type="roleTagType(row.role)">{{ roleLabel(row.role) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200">
              <template #default="{ row }">
                <el-select v-model="row.role" size="small" style="width:100px" @change="updateRole(row)">
                  <el-option label="管理员" value="admin" />
                  <el-option label="编辑者" value="editor" />
                  <el-option label="查看者" value="viewer" />
                </el-select>
                <el-button size="small" type="danger" text @click="removeMember(row)">移除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </div>

    <el-dialog v-model="showCreateDialog" title="新建项目" width="480px">
      <el-form :model="createForm" label-position="top">
        <el-form-item label="名称">
          <el-input v-model="createForm.name" placeholder="项目名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="createForm.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="Base Path">
          <el-input v-model="createForm.basePath" placeholder="/api/v1" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="createProject" :loading="creating">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showInviteDialog" title="邀请成员" width="480px">
      <el-form :model="inviteForm" label-position="top">
        <el-form-item label="邮箱">
          <el-input v-model="inviteForm.email" placeholder="member@example.com" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="inviteForm.role" style="width:100%">
            <el-option label="管理员" value="admin" />
            <el-option label="编辑者" value="editor" />
            <el-option label="查看者" value="viewer" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showInviteDialog = false">取消</el-button>
        <el-button type="primary" @click="inviteMember" :loading="inviting">邀请</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showInviteLink" title="邀请链接" width="480px">
      <p>将以下链接发送给被邀请者：</p>
      <el-input :model-value="inviteLink" readonly>
        <template #append>
          <el-button @click="copyLink">复制</el-button>
        </template>
      </el-input>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { workspaceAPI, projectAPI } from '../api'

const route = useRoute()
const wsId = route.params.id

const workspace = ref(null)
const projects = ref([])
const members = ref([])
const activeTab = ref('projects')
const showCreateDialog = ref(false)
const showInviteDialog = ref(false)
const showInviteLink = ref(false)
const inviteLink = ref('')
const creating = ref(false)
const inviting = ref(false)
const createForm = ref({ name: '', description: '', basePath: '' })
const inviteForm = ref({ email: '', role: 'viewer' })

function roleLabel(r) { return { admin: '管理员', editor: '编辑者', viewer: '查看者' }[r] || r }
function roleTagType(r) { return { admin: 'danger', editor: 'warning', viewer: 'info' }[r] || 'info' }

async function loadWorkspace() {
  try {
    const res = await workspaceAPI.get(wsId)
    workspace.value = res.workspace
  } catch {}
}

async function loadProjects() {
  try {
    const res = await projectAPI.list(wsId)
    projects.value = res.projects || []
  } catch {}
}

async function loadMembers() {
  try {
    const res = await workspaceAPI.listMembers(wsId)
    members.value = res.members || []
  } catch {}
}

async function createProject() {
  if (!createForm.value.name) { ElMessage.warning('请输入名称'); return }
  creating.value = true
  try {
    await projectAPI.create(wsId, createForm.value)
    ElMessage.success('创建成功')
    showCreateDialog.value = false
    createForm.value = { name: '', description: '', basePath: '' }
    loadProjects()
  } finally { creating.value = false }
}

async function inviteMember() {
  if (!inviteForm.value.email) { ElMessage.warning('请输入邮箱'); return }
  inviting.value = true
  try {
    const res = await workspaceAPI.inviteMember(wsId, inviteForm.value)
    inviteLink.value = window.location.origin + res.invite_link
    showInviteLink.value = true
    showInviteDialog.value = false
    inviteForm.value = { email: '', role: 'viewer' }
    loadMembers()
  } finally { inviting.value = false }
}

function copyLink() {
  navigator.clipboard.writeText(inviteLink.value)
  ElMessage.success('已复制')
}

async function updateRole(member) {
  try {
    await workspaceAPI.updateMemberRole(wsId, member.id, { role: member.role })
    ElMessage.success('角色已更新')
  } catch { loadMembers() }
}

async function removeMember(member) {
  try {
    await ElMessageBox.confirm(`确定移除 ${member.user_name}?`, '确认')
    await workspaceAPI.removeMember(wsId, member.id)
    ElMessage.success('已移除')
    loadMembers()
  } catch {}
}

onMounted(() => { loadWorkspace(); loadProjects(); loadMembers() })
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

.project-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.project-card {
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.project-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.project-card h3 {
  font-size: 18px;
  margin-bottom: 8px;
  color: #1a1a2e;
}

.project-card .desc {
  color: #888;
  font-size: 14px;
  margin-bottom: 12px;
}

.project-card .meta {
  color: #aaa;
  font-size: 12px;
}
</style>
