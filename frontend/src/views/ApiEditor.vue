<template>
  <div class="layout">
    <header class="top-bar">
      <div class="brand" @click="$router.push('/')">API Mocker</div>
      <el-button text style="color:#fff" @click="$router.push(`/project/${projectId}`)">返回项目</el-button>
    </header>

    <div class="page-container" v-if="loaded">
      <div class="page-header">
        <h2>{{ isEdit ? '编辑接口' : '新建接口' }}</h2>
        <div>
          <el-button @click="$router.back()">取消</el-button>
          <el-button type="primary" @click="saveApi" :loading="saving">保存</el-button>
        </div>
      </div>

      <div class="card">
        <el-form :model="form" label-width="100px" label-position="top">
          <el-row :gutter="16">
            <el-col :span="6">
              <el-form-item label="请求方法">
                <el-select v-model="form.method" style="width:100%">
                  <el-option v-for="m in methods" :key="m" :label="m" :value="m" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="路径">
                <el-input v-model="form.path" placeholder="/api/users/:id" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="标签">
                <el-select v-model="form.tags" multiple filterable allow-create placeholder="添加标签" style="width:100%">
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="描述">
            <el-input v-model="form.description" type="textarea" :rows="2" placeholder="接口描述" />
          </el-form-item>
        </el-form>
      </div>

      <el-tabs v-model="activeTab" class="detail-tabs">
        <el-tab-pane label="请求参数" name="params">
          <div class="card">
            <div class="section-header">
              <h3>参数列表</h3>
              <el-button size="small" type="primary" @click="addParam">添加参数</el-button>
            </div>
            <el-table :data="form.params" stripe style="width:100%">
              <el-table-column label="名称" width="160">
                <template #default="{ row }">
                  <el-input v-model="row.name" size="small" placeholder="参数名" />
                </template>
              </el-table-column>
              <el-table-column label="位置" width="120">
                <template #default="{ row }">
                  <el-select v-model="row.in" size="small">
                    <el-option label="Query" value="query" />
                    <el-option label="Header" value="header" />
                    <el-option label="Path" value="path" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="类型" width="120">
                <template #default="{ row }">
                  <el-select v-model="row.type" size="small">
                    <el-option label="string" value="string" />
                    <el-option label="number" value="number" />
                    <el-option label="integer" value="integer" />
                    <el-option label="boolean" value="boolean" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="必填" width="70">
                <template #default="{ row }">
                  <el-checkbox v-model="row.required" />
                </template>
              </el-table-column>
              <el-table-column label="示例值" width="160">
                <template #default="{ row }">
                  <el-input v-model="row.example" size="small" placeholder="示例" />
                </template>
              </el-table-column>
              <el-table-column label="说明">
                <template #default="{ row }">
                  <el-input v-model="row.desc" size="small" placeholder="说明" />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="70">
                <template #default="{ $index }">
                  <el-button size="small" type="danger" text @click="form.params.splice($index, 1)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <el-tab-pane label="请求体" name="requestBody">
          <div class="card">
            <div class="section-header">
              <h3>请求体字段</h3>
              <el-button size="small" type="primary" @click="addField('requestBody')">添加字段</el-button>
            </div>
            <BodyFieldEditor
              :fields="form.requestBody.fields"
              :models="models"
              :depth="0"
              @add-child="addChildField('requestBody', $event)"
              @remove="removeField('requestBody', $event)"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="响应体" name="responses">
          <div class="card">
            <div class="section-header">
              <h3>响应定义</h3>
              <el-button size="small" type="primary" @click="addResponse">添加状态码</el-button>
            </div>
            <el-collapse v-model="expandedResponses">
              <el-collapse-item v-for="(resp, code) in form.responses" :key="code" :name="code">
                <template #title>
                  <span :class="['status-badge', statusClass(code)]">{{ code }}</span>
                  <el-input v-model="resp.description" size="small" style="width:300px;margin-left:12px" placeholder="描述" @click.stop />
                </template>
                <div class="section-header">
                  <span>字段定义</span>
                  <el-button size="small" type="primary" @click="addField('response', code)">添加字段</el-button>
                </div>
                <BodyFieldEditor
                  :fields="resp.body"
                  :models="models"
                  :depth="0"
                  @add-child="addChildField('response', { parentIndex: $event, code })"
                  @remove="removeField('response', { index: $event, code })"
                />
              </el-collapse-item>
            </el-collapse>
          </div>
        </el-tab-pane>

        <el-tab-pane label="Mock场景" name="scenarios">
          <div class="card">
            <div class="section-header">
              <h3>条件Mock场景</h3>
              <el-button size="small" type="primary" @click="addScenario">添加场景</el-button>
            </div>
            <div v-for="(scenario, idx) in form.scenarios" :key="idx" class="scenario-block">
              <div class="scenario-header">
                <span>场景 {{ idx + 1 }}</span>
                <el-button size="small" type="danger" text @click="form.scenarios.splice(idx, 1)">删除</el-button>
              </div>
              <el-form :model="scenario" label-width="80px" size="small">
                <el-row :gutter="12">
                  <el-col :span="8">
                    <el-form-item label="名称">
                      <el-input v-model="scenario.name" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="4">
                    <el-form-item label="优先级">
                      <el-input-number v-model="scenario.priority" :min="0" :max="100" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="4">
                    <el-form-item label="状态码">
                      <el-input-number v-model="scenario.statusCode" :min="100" :max="599" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="4">
                    <el-form-item label="延迟(ms)">
                      <el-input-number v-model="scenario.delayMs" :min="0" :max="200" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </el-form>
              <div class="conditions-section">
                <div class="section-header">
                  <span>匹配条件</span>
                  <el-button size="small" @click="addCondition(idx)">添加条件</el-button>
                </div>
                <div v-for="(cond, ci) in scenario.conditions" :key="ci" class="condition-row">
                  <el-select v-model="cond.in" size="small" style="width:90px">
                    <el-option label="Query" value="query" />
                    <el-option label="Header" value="header" />
                    <el-option label="Body" value="body" />
                    <el-option label="Path" value="path" />
                  </el-select>
                  <el-input v-model="cond.field" size="small" style="width:120px" placeholder="字段名" />
                  <el-select v-model="cond.operator" size="small" style="width:100px">
                    <el-option label="等于" value="eq" />
                    <el-option label="不等于" value="neq" />
                    <el-option label="包含" value="contains" />
                    <el-option label="存在" value="exists" />
                  </el-select>
                  <el-input v-model="cond.value" size="small" style="width:120px" placeholder="值" />
                  <el-button size="small" type="danger" text @click="scenario.conditions.splice(ci, 1)">删除</el-button>
                </div>
              </div>
            </div>
            <el-empty v-if="!form.scenarios.length" description="暂无Mock场景" />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { apiDefAPI, scenarioAPI, modelAPI } from '../api'
import BodyFieldEditor from '../components/BodyFieldEditor.vue'

const route = useRoute()
const router = useRouter()
const projectId = route.params.projectId
const apiId = route.params.apiId
const isEdit = computed(() => apiId && apiId !== 'new')

const loaded = ref(false)
const saving = ref(false)
const activeTab = ref('params')
const expandedResponses = ref(['200'])
const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
const models = ref([])

const form = ref({
  method: 'GET',
  path: '',
  description: '',
  tags: [],
  params: [],
  requestBody: { fields: [] },
  responses: {
    '200': { description: '成功', body: [] }
  },
  scenarios: []
})

function addParam() {
  form.value.params.push({ name: '', in: 'query', type: 'string', required: false, example: '', desc: '' })
}

function addField(section, code) {
  const field = { name: '', type: 'string', required: false, example: '', desc: '', children: [], ref: '', enum: [] }
  if (section === 'requestBody') {
    form.value.requestBody.fields.push(field)
  } else if (section === 'response') {
    form.value.responses[code].body.push(field)
  }
}

function addChildField(section, { parentIndex, code }) {
  const field = { name: '', type: 'string', required: false, example: '', desc: '', children: [], ref: '', enum: [] }
  if (section === 'requestBody') {
    form.value.requestBody.fields[parentIndex].children.push(field)
  } else if (section === 'response') {
    form.value.responses[code].body[parentIndex].children.push(field)
  }
}

function removeField(section, { index, code }) {
  if (section === 'requestBody') {
    form.value.requestBody.fields.splice(index, 1)
  } else if (section === 'response') {
    form.value.responses[code].body.splice(index, 1)
  }
}

function addResponse() {
  const code = prompt('状态码（如 400、401、500）:')
  if (code && !form.value.responses[code]) {
    form.value.responses[code] = { description: '', body: [] }
    expandedResponses.value.push(code)
  }
}

function addScenario() {
  form.value.scenarios.push({
    name: '',
    priority: form.value.scenarios.length,
    statusCode: 200,
    delayMs: 0,
    conditions: [],
    response: {}
  })
}

function addCondition(scenarioIdx) {
  form.value.scenarios[scenarioIdx].conditions.push({
    field: '', in: 'query', operator: 'eq', value: ''
  })
}

function statusClass(code) {
  const c = parseInt(code)
  if (c >= 200 && c < 300) return 'status-success'
  if (c >= 400 && c < 500) return 'status-warn'
  if (c >= 500) return 'status-error'
  return ''
}

async function loadModels() {
  try {
    const res = await modelAPI.list(projectId)
    models.value = res.models || []
  } catch {}
}

async function loadApi() {
  if (!isEdit.value) { loaded.value = true; return }
  try {
    const res = await apiDefAPI.get(projectId, apiId)
    const api = res.api
    form.value = {
      method: api.method,
      path: api.path,
      description: api.description || '',
      tags: api.tags || [],
      params: api.params || [],
      requestBody: api.request_body || { fields: [] },
      responses: api.responses || { '200': { description: '成功', body: [] } },
      scenarios: []
    }
    if (!form.value.requestBody.fields) {
      form.value.requestBody = { fields: [] }
    }
    if (!form.value.responses['200']) {
      form.value.responses['200'] = { description: '成功', body: [] }
    }
    for (const code of Object.keys(form.value.responses)) {
      if (!form.value.responses[code].body) {
        form.value.responses[code].body = []
      }
    }

    const scenarioRes = await scenarioAPI.list(projectId, apiId)
    form.value.scenarios = (scenarioRes.scenarios || []).map(s => ({
      id: s.id,
      name: s.name,
      priority: s.priority,
      statusCode: s.status_code,
      delayMs: s.delay_ms,
      conditions: s.conditions || [],
      response: s.response || {}
    }))
  } catch {}
  loaded.value = true
}

async function saveApi() {
  if (!form.value.path) { ElMessage.warning('请输入路径'); return }
  if (!form.value.path.startsWith('/')) { ElMessage.warning('路径必须以 / 开头'); return }
  if (form.value.path.includes('//')) { ElMessage.warning('路径不允许连续双斜杠'); return }

  saving.value = true
  try {
    const payload = {
      method: form.value.method,
      path: form.value.path,
      description: form.value.description,
      tags: form.value.tags,
      params: form.value.params,
      requestBody: form.value.requestBody,
      responses: form.value.responses
    }

    if (isEdit.value) {
      await apiDefAPI.update(projectId, apiId, payload)

      for (const s of form.value.scenarios) {
        if (s.id) {
          await scenarioAPI.update(projectId, apiId, s.id, {
            name: s.name,
            priority: s.priority,
            statusCode: s.statusCode,
            delayMs: s.delayMs,
            conditions: s.conditions,
            response: s.response
          })
        } else {
          await scenarioAPI.create(projectId, apiId, {
            name: s.name,
            priority: s.priority,
            statusCode: s.statusCode,
            delayMs: s.delayMs,
            conditions: s.conditions,
            response: s.response
          })
        }
      }
      ElMessage.success('保存成功')
    } else {
      await apiDefAPI.create(projectId, payload)
      ElMessage.success('创建成功')
    }
    router.push(`/project/${projectId}`)
  } finally { saving.value = false }
}

onMounted(() => { loadModels(); loadApi() })
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

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.section-header h3 {
  font-size: 15px;
  color: #333;
}

.scenario-block {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 12px;
}

.scenario-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-weight: 600;
}

.conditions-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed #ebeef5;
}

.condition-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.status-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 4px;
  font-weight: 600;
  font-size: 13px;
}

.status-success { background: #f0f9eb; color: #67c23a; }
.status-warn { background: #fdf6ec; color: #e6a23c; }
.status-error { background: #fef0f0; color: #f56c6c; }

.detail-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
}
</style>
