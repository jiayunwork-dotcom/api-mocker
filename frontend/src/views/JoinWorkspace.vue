<template>
  <div class="join-page">
    <div class="join-card card">
      <h2>加入工作空间</h2>
      <el-form :model="form" label-position="top">
        <el-form-item label="邀请令牌">
          <el-input v-model="form.token" placeholder="输入邀请链接中的Token" />
        </el-form-item>
        <el-form-item label="选择角色">
          <el-select v-model="form.role" style="width:100%">
            <el-option label="管理员" value="admin" />
            <el-option label="编辑者" value="editor" />
            <el-option label="查看者" value="viewer" />
          </el-select>
        </el-form-item>
        <el-button type="primary" @click="joinWorkspace" :loading="loading" style="width:100%">加入</el-button>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { workspaceAPI } from '../api'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const form = ref({ token: '', role: 'viewer' })

onMounted(() => {
  if (route.query.token) {
    form.value.token = route.query.token
  }
})

async function joinWorkspace() {
  if (!form.value.token) { ElMessage.warning('请输入Token'); return }
  loading.value = true
  try {
    const res = await workspaceAPI.join(form.value)
    ElMessage.success('加入成功')
    router.push(`/workspace/${res.workspace_id}`)
  } finally { loading.value = false }
}
</script>

<style scoped>
.join-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}

.join-card {
  width: 420px;
}

.join-card h2 {
  text-align: center;
  margin-bottom: 24px;
}
</style>
