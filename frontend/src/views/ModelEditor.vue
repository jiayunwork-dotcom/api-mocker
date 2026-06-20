<template>
  <div class="layout">
    <header class="top-bar">
      <div class="brand" @click="$router.push('/')">API Mocker</div>
      <el-button text style="color:#fff" @click="$router.push(`/project/${projectId}`)">返回项目</el-button>
    </header>

    <div class="page-container">
      <div class="page-header">
        <h2>公共模型</h2>
        <el-button type="primary" @click="showCreateDialog = true">新建模型</el-button>
      </div>

      <div class="model-grid">
        <div v-for="model in models" :key="model.id" class="model-card card">
          <div class="model-header">
            <h3>{{ model.name }}</h3>
            <div class="model-actions">
              <el-button size="small" @click="editModel(model)">编辑</el-button>
              <el-button size="small" type="danger" text @click="deleteModel(model)">删除</el-button>
            </div>
          </div>
          <p class="desc">{{ model.description || '暂无描述' }}</p>
          <div class="schema-preview">
            <pre>{{ formatSchema(model.schema_definition) }}</pre>
          </div>
        </div>
      </div>

      <el-empty v-if="!models.length" description="暂无公共模型" />

      <el-dialog v-model="showCreateDialog" :title="editingModel ? '编辑模型' : '新建模型'" width="640px">
        <el-form :model="modelForm" label-position="top">
          <el-form-item label="模型名称">
            <el-input v-model="modelForm.name" placeholder="如 User, Product" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="modelForm.description" type="textarea" :rows="2" />
          </el-form-item>
          <el-form-item label="字段定义">
            <div class="section-header">
              <span></span>
              <el-button size="small" type="primary" @click="addModelField">添加字段</el-button>
            </div>
            <BodyFieldEditor
              v-for="(field, idx) in modelForm.fields"
              :key="idx"
              :field="field"
              :models="models"
              :depth="0"
              :index="idx"
              @add-child="addChildField($event)"
              @remove="removeField($event)"
            />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="closeModelDialog">取消</el-button>
          <el-button type="primary" @click="saveModel" :loading="saving">保存</el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { modelAPI } from '../api'
import BodyFieldEditor from '../components/BodyFieldEditor.vue'

const route = useRoute()
const router = useRouter()
const projectId = route.params.projectId

const models = ref([])
const showCreateDialog = ref(false)
const editingModel = ref(null)
const saving = ref(false)
const modelForm = ref({ name: '', description: '', fields: [] })

async function loadModels() {
  try {
    const res = await modelAPI.list(projectId)
    models.value = res.models || []
  } catch {}
}

function addModelField() {
  modelForm.value.fields.push({ name: '', type: 'string', required: false, example: '', desc: '', children: [], ref: '', enum: [] })
}

function addChildField({ parentIndex }) {
  modelForm.value.fields[parentIndex].children.push({ name: '', type: 'string', required: false, example: '', desc: '', children: [], ref: '', enum: [] })
}

function removeField({ index }) {
  modelForm.value.fields.splice(index, 1)
}

function editModel(model) {
  editingModel.value = model
  let fields = []
  if (model.schema_definition) {
    if (Array.isArray(model.schema_definition)) {
      fields = model.schema_definition
    } else if (model.schema_definition.fields) {
      fields = model.schema_definition.fields
    }
  }
  modelForm.value = {
    name: model.name,
    description: model.description || '',
    fields: JSON.parse(JSON.stringify(fields))
  }
  showCreateDialog.value = true
}

function closeModelDialog() {
  showCreateDialog.value = false
  editingModel.value = null
  modelForm.value = { name: '', description: '', fields: [] }
}

async function saveModel() {
  if (!modelForm.value.name) { ElMessage.warning('请输入模型名称'); return }
  saving.value = true
  try {
    const payload = {
      name: modelForm.value.name,
      description: modelForm.value.description,
      schemaDefinition: modelForm.value.fields
    }
    if (editingModel.value) {
      await modelAPI.update(projectId, editingModel.value.id, payload)
      ElMessage.success('更新成功')
    } else {
      await modelAPI.create(projectId, payload)
      ElMessage.success('创建成功')
    }
    closeModelDialog()
    loadModels()
  } finally { saving.value = false }
}

async function deleteModel(model) {
  try {
    await ElMessageBox.confirm(`确定删除模型 ${model.name}?`, '确认')
    await modelAPI.delete(projectId, model.id)
    ElMessage.success('已删除')
    loadModels()
  } catch {}
}

function formatSchema(schema) {
  if (!schema) return ''
  try {
    return JSON.stringify(schema, null, 2)
  } catch { return '' }
}

onMounted(loadModels)
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

.model-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 16px;
}

.model-card h3 {
  font-size: 16px;
  margin-bottom: 4px;
  color: #1a1a2e;
}

.model-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.model-actions {
  display: flex;
  gap: 4px;
}

.desc {
  color: #888;
  font-size: 13px;
  margin-bottom: 8px;
}

.schema-preview {
  background: #f5f7fa;
  border-radius: 4px;
  padding: 8px;
  max-height: 200px;
  overflow: auto;
}

.schema-preview pre {
  font-size: 12px;
  margin: 0;
  white-space: pre-wrap;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
</style>
