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

      <el-tabs v-model="activeTab" class="project-tabs">
        <el-tab-pane label="接口列表" name="apis">
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
        </el-tab-pane>

        <el-tab-pane label="健康监控" name="health">
          <HealthMonitor :projectId="projectId" />
        </el-tab-pane>

        <el-tab-pane label="依赖图谱" name="dependency">
          <div class="topo-section">
            <div class="section-header">
              <div class="section-title" style="margin:0">依赖拓扑图</div>
              <div class="topo-legend">
                <span class="legend-item"><span class="legend-dot" style="background:#f56c6c"></span>Breaking</span>
                <span class="legend-item"><span class="legend-dot" style="background:#e6a23c"></span>Warning</span>
                <span class="legend-item"><span class="legend-dot" style="background:#c0c4cc"></span>正常</span>
              </div>
            </div>
            <DependencyTopoGraph
              :dependencies="dependencies"
              :apis="apis"
              :impactReports="impactReports"
            />
          </div>

          <div class="dependency-section">
            <div class="section-header">
              <div class="section-title" style="margin:0">依赖关系管理</div>
              <div style="display:flex;gap:8px">
                <el-button @click="showBatchDepDialog = true">
                  <el-icon style="margin-right:4px"><UploadFilled /></el-icon>
                  批量导入
                </el-button>
                <el-button type="primary" @click="openDepDialog()">新建依赖</el-button>
              </div>
            </div>

            <el-table
              :data="dependencies"
              style="width: 100%"
              row-key="id"
              :expand-row-keys="expandedDepRows"
              @expand-change="onDepExpandChange"
            >
              <el-table-column type="expand">
                <template #default="{ row }">
                  <div class="mapping-detail">
                    <div class="mapping-title">字段映射详情：</div>
                    <div
                      v-for="(mapping, idx) in parseMappings(row.field_mappings)"
                      :key="idx"
                      class="mapping-item"
                    >
                      <span class="mapping-arrow">{{ mapping.upstreamField }}</span>
                      <el-icon><Right /></el-icon>
                      <span class="mapping-arrow">{{ mapping.downstreamField }}</span>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="上游接口" min-width="200">
                <template #default="{ row }">
                  <span :class="['method-badge', `method-${row.upstream_method.toLowerCase()}`]">
                    {{ row.upstream_method }}
                  </span>
                  <span class="dep-path">{{ row.upstream_path }}</span>
                </template>
              </el-table-column>
              <el-table-column label="下游接口" min-width="200">
                <template #default="{ row }">
                  <span :class="['method-badge', `method-${row.downstream_method.toLowerCase()}`]">
                    {{ row.downstream_method }}
                  </span>
                  <span class="dep-path">{{ row.downstream_path }}</span>
                </template>
              </el-table-column>
              <el-table-column label="映射数量" width="100" align="center">
                <template #default="{ row }">
                  <el-tag size="small">{{ parseMappings(row.field_mappings).length }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="创建时间" width="180">
                <template #default="{ row }">
                  {{ formatTime(row.created_at) }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="150" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" text @click="editDependency(row)">编辑</el-button>
                  <el-button size="small" type="danger" text @click="deleteDependency(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-if="!dependencies.length" description="暂无依赖关系，点击右上角创建" />
          </div>

          <div class="section-divider"></div>

          <div class="impact-section">
            <div class="section-header">
              <div class="section-title" style="margin:0">变更影响记录</div>
            </div>

            <el-table :data="impactReports" style="width: 100%" row-key="id">
              <el-table-column label="变更接口" min-width="200">
                <template #default="{ row }">
                  <span :class="['method-badge', `method-${row.changed_api_method.toLowerCase()}`]">
                    {{ row.changed_api_method }}
                  </span>
                  <span class="dep-path">{{ row.changed_api_path }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="change_type" label="变更类型" width="120">
                <template #default="{ row }">
                  <el-tag :type="getChangeTypeTag(row.change_type)" size="small">
                    {{ getChangeTypeLabel(row.change_type) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="是否破坏性" width="100" align="center">
                <template #default="{ row }">
                  <el-tag v-if="row.has_breaking_change" type="danger" size="small">Breaking</el-tag>
                  <el-tag v-else type="warning" size="small">Warning</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="受影响下游" width="100" align="center">
                <template #default="{ row }">
                  {{ parseAffected(row.affected_downstream).length }}
                </template>
              </el-table-column>
              <el-table-column label="操作人" width="100">
                <template #default="{ row }">
                  {{ row.user_name }}
                </template>
              </el-table-column>
              <el-table-column label="创建时间" width="180">
                <template #default="{ row }">
                  {{ formatTime(row.created_at) }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="100" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" text @click="viewReport(row)">查看详情</el-button>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-if="!impactReports.length" description="暂无变更影响记录" />
          </div>
        </el-tab-pane>
      </el-tabs>
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

    <el-dialog
      v-model="showDepDialog"
      :title="depDialogMode === 'create' ? '新建依赖关系' : '编辑依赖关系'"
      width="700px"
      :close-on-click-modal="false"
    >
      <el-form label-width="100px">
        <el-form-item label="上游接口">
          <el-select
            v-model="depForm.upstream_api_id"
            placeholder="请选择上游接口"
            style="width: 100%"
            :disabled="depDialogMode === 'edit'"
          >
            <el-option
              v-for="api in apis"
              :key="api.id"
              :label="`${api.method} ${api.path}`"
              :value="api.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="下游接口">
          <el-select
            v-model="depForm.downstream_api_id"
            placeholder="请选择下游接口"
            style="width: 100%"
            :disabled="depDialogMode === 'edit'"
          >
            <el-option
              v-for="api in apis"
              :key="api.id"
              :label="`${api.method} ${api.path}`"
              :value="api.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="字段映射">
          <div style="width: 100%">
            <div
              v-for="(mapping, idx) in depForm.field_mappings"
              :key="idx"
              class="mapping-row"
            >
              <el-input
                v-model="mapping.upstreamField"
                placeholder="上游响应字段 (如: data.id)"
                style="flex: 1"
              />
              <span class="mapping-icon">→</span>
              <el-input
                v-model="mapping.downstreamField"
                placeholder="下游请求字段 (如: body.userId)"
                style="flex: 1"
              />
              <el-button
                type="danger"
                text
                @click="removeMapping(idx)"
                :disabled="depForm.field_mappings.length === 1"
              >
                删除
              </el-button>
            </div>
            <el-button type="primary" text style="margin-top: 8px" @click="addMapping">
              + 添加字段映射
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDepDialog = false">取消</el-button>
        <el-button type="primary" @click="saveDependency">
          {{ depDialogMode === 'create' ? '创建' : '保存' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showBatchDepDialog"
      title="批量导入依赖关系"
      width="700px"
      :close-on-click-modal="false"
    >
      <div class="batch-dep-hint">
        请粘贴JSON格式的依赖关系数组，格式示例：
        <pre class="batch-dep-example">[
  {
    "upstream": "GET /users",
    "downstream": "POST /orders",
    "mappings": [{"from": "id", "to": "userId"}]
  }
]</pre>
      </div>
      <el-input
        v-model="batchDepContent"
        type="textarea"
        :rows="12"
        placeholder='[{"upstream":"GET /users","downstream":"POST /orders","mappings":[{"from":"id","to":"userId"}]}]'
        resize="none"
      />
      <div v-if="batchDepError" class="error-alert" style="margin-top:12px">
        <el-alert :title="batchDepError" type="error" :closable="false" show-icon />
      </div>
      <div v-if="batchDepResult" style="margin-top:12px">
        <el-alert type="success" :closable="false" show-icon>
          <template #title>
            <div class="result-summary">
              <span>导入完成：</span>
              <el-tag type="success">成功 {{ batchDepResult.created }} 条</el-tag>
              <el-tag type="warning" v-if="batchDepResult.skipped > 0">跳过 {{ batchDepResult.skipped }} 条</el-tag>
            </div>
          </template>
        </el-alert>
      </div>
      <template #footer>
        <el-button @click="closeBatchDepDialog">关闭</el-button>
        <el-button type="primary" :loading="batchDepLoading" @click="doBatchDepImport">确认导入</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showReportDetail"
      title="影响报告详情"
      width="700px"
      :close-on-click-modal="false"
    >
      <div v-if="reportDetail" class="report-detail-content">
        <div class="report-detail-section">
          <div class="report-detail-label">变更接口：</div>
          <div>
            <span :class="['method-badge', `method-${reportDetail.changed_api_method?.toLowerCase()}`]">
              {{ reportDetail.changed_api_method }}
            </span>
            <span class="dep-path">{{ reportDetail.changed_api_path }}</span>
          </div>
        </div>
        <div class="report-detail-section">
          <div class="report-detail-label">变更类型：</div>
          <el-tag :type="getChangeTypeTag(reportDetail.change_type)" size="small">
            {{ getChangeTypeLabel(reportDetail.change_type) }}
          </el-tag>
        </div>
        <div class="report-detail-section">
          <div class="report-detail-label">操作人：</div>
          <div>{{ reportDetail.user_name }}</div>
        </div>

        <div class="report-detail-section">
          <div class="report-detail-label">变更字段：</div>
          <ul class="report-detail-list">
            <li v-for="(f, i) in parseAffected(reportDetail.changed_fields)" :key="i">
              {{ f.fieldPath }}
              <el-tag size="small" :type="f.changeType === 'delete' ? 'danger' : f.changeType === 'type_change' ? 'danger' : 'warning'" style="margin-left:8px">
                {{ f.changeType === 'delete' ? '删除' : f.changeType === 'type_change' ? f.oldType + ' → ' + f.newType : f.oldName + ' → ' + f.newName }}
              </el-tag>
            </li>
          </ul>
        </div>

        <div class="report-detail-section">
          <div class="report-detail-label">直接受影响下游：</div>
          <ul class="report-detail-list">
            <li v-for="(d, i) in parseAffected(reportDetail.affected_downstream)" :key="i">
              <span :class="['method-badge', `method-${d.downstream_method?.toLowerCase()}`]">{{ d.downstream_method }}</span>
              <span class="dep-path">{{ d.downstream_path }}</span>
              <el-tag size="small" :type="d.impact_level === 'Breaking' ? 'danger' : 'warning'" style="margin-left:8px">{{ d.impact_level }}</el-tag>
              <div class="affected-mapping-text">受影响映射：{{ d.affected_mappings.join(', ') }}</div>
            </li>
          </ul>
        </div>

        <div class="report-detail-section" v-if="impactChain.length > 0">
          <div class="report-detail-label">影响链路追踪：</div>
          <div class="chain-tree">
            <chain-node
              v-for="(node, i) in impactChain"
              :key="i"
              :node="node"
            />
          </div>
        </div>
        <div v-else-if="chainLoading" class="chain-loading">
          <el-icon class="is-loading"><Loading /></el-icon> 正在加载链路...
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, watch, inject, onUnmounted, h, defineComponent } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { InfoFilled, UploadFilled, Document, Right, Loading } from '@element-plus/icons-vue'
import { apiDefAPI, activityAPI, dependencyAPI, impactReportAPI, projectAPI } from '../api'
import HealthMonitor from '../components/HealthMonitor.vue'
import DependencyTopoGraph from '../components/DependencyTopoGraph.vue'

const route = useRoute()
const router = useRouter()
const projectId = route.params.id
const wsConnect = inject('wsConnect')
const wsClose = inject('wsClose')

const project = ref(null)
const apis = ref([])
const activities = ref([])
const dependencies = ref([])
const impactReports = ref([])
const mockBaseUrl = window.location.origin
const activeTab = ref('apis')
const expandedDepRows = ref([])

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

const showDepDialog = ref(false)
const depDialogMode = ref('create')
const editingDep = ref(null)
const depForm = ref({
  upstream_api_id: '',
  downstream_api_id: '',
  field_mappings: [{ upstreamField: '', downstreamField: '' }]
})

const showBatchDepDialog = ref(false)
const batchDepContent = ref('')
const batchDepLoading = ref(false)
const batchDepError = ref('')
const batchDepResult = ref(null)

const showReportDetail = ref(false)
const reportDetail = ref(null)
const impactChain = ref([])
const chainLoading = ref(false)

const ChainNode = defineComponent({
  name: 'ChainNode',
  props: {
    node: { type: Object, required: true }
  },
  setup(props) {
    return () => {
      const n = props.node
      const levelTag = n.impact === 'Breaking'
        ? h('el-tag', { type: 'danger', size: 'small', style: 'margin-left:8px' }, 'Breaking')
        : n.impact === 'Warning'
          ? h('el-tag', { type: 'warning', size: 'small', style: 'margin-left:8px' }, 'Warning')
          : n.impact === 'indirect'
            ? h('el-tag', { type: 'info', size: 'small', style: 'margin-left:8px' }, '间接影响')
            : null

      const mappingText = n.mappings && n.mappings.length > 0
        ? h('div', { style: 'color:#909399;font-size:12px;margin-top:4px' },
            '映射：' + n.mappings.join(', '))
        : null

      const children = n.children && n.children.length > 0
        ? h('div', { class: 'chain-children' },
            n.children.map((child, i) => h(ChainNode, { node: child, key: i })))
        : null

      return h('div', { class: 'chain-node-item' }, [
        h('div', { class: 'chain-node-header' }, [
          h('span', { class: `method-badge method-${n.method?.toLowerCase()}` }, n.method),
          h('span', { class: 'dep-path' }, n.path),
          levelTag
        ]),
        mappingText,
        children
      ])
    }
  }
})

async function loadProject() {
  try {
    const res = await projectAPI.getById(projectId)
    project.value = res.project
  } catch {
    project.value = { id: projectId, name: '加载中...', description: '' }
  }
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

async function loadDependencies() {
  try {
    const res = await dependencyAPI.list(projectId)
    dependencies.value = res.dependencies || []
  } catch {}
}

async function loadImpactReports() {
  try {
    const res = await impactReportAPI.list(projectId)
    impactReports.value = res.reports || []
  } catch {}
}

function parseMappings(mappings) {
  console.log('[parseMappings] input:', mappings, 'type:', typeof mappings)
  if (!mappings) return []
  let parsed = mappings
  if (typeof mappings === 'string') {
    try {
      parsed = JSON.parse(mappings)
    } catch (e) {
      console.error('[parseMappings] JSON.parse failed:', e)
      return []
    }
  }
  if (!Array.isArray(parsed)) {
    console.error('[parseMappings] result is not array:', parsed)
    return []
  }
  return parsed.map(m => ({
    upstreamField: m.upstreamField || m.upstream_field || '',
    downstreamField: m.downstreamField || m.downstream_field || ''
  }))
}

function parseAffected(affected) {
  if (!affected) return []
  if (typeof affected === 'string') {
    try {
      return JSON.parse(affected)
    } catch {
      return []
    }
  }
  return affected
}

function getChangeTypeTag(type) {
  const map = {
    field_delete: 'danger',
    type_change: 'danger',
    field_rename: 'warning',
    mixed: 'warning'
  }
  return map[type] || ''
}

function getChangeTypeLabel(type) {
  const map = {
    field_delete: '字段删除',
    type_change: '类型变更',
    field_rename: '字段重命名',
    mixed: '混合变更'
  }
  return map[type] || type
}

function onDepExpandChange(row, expandedRows) {
  expandedDepRows.value = expandedRows.map(r => r.id)
}

function openDepDialog(dep = null) {
  if (dep) {
    depDialogMode.value = 'edit'
    editingDep.value = dep
    depForm.value = {
      upstream_api_id: dep.upstream_api_id,
      downstream_api_id: dep.downstream_api_id,
      field_mappings: [...parseMappings(dep.field_mappings)]
    }
  } else {
    depDialogMode.value = 'create'
    editingDep.value = null
    depForm.value = {
      upstream_api_id: '',
      downstream_api_id: '',
      field_mappings: [{ upstreamField: '', downstreamField: '' }]
    }
  }
  showDepDialog.value = true
}

function editDependency(dep) {
  openDepDialog(dep)
}

async function deleteDependency(dep) {
  try {
    await ElMessageBox.confirm('确定删除这条依赖关系？', '确认')
    await dependencyAPI.delete(projectId, dep.id)
    ElMessage.success('已删除')
    loadDependencies()
  } catch {}
}

function addMapping() {
  depForm.value.field_mappings.push({ upstreamField: '', downstreamField: '' })
}

function removeMapping(idx) {
  if (depForm.value.field_mappings.length > 1) {
    depForm.value.field_mappings.splice(idx, 1)
  }
}

async function saveDependency() {
  try {
    if (!depForm.value.upstream_api_id || !depForm.value.downstream_api_id) {
      ElMessage.error('请选择上游和下游接口')
      return
    }
    if (depForm.value.upstream_api_id === depForm.value.downstream_api_id) {
      ElMessage.error('上游和下游接口不能相同')
      return
    }
    const validMappings = depForm.value.field_mappings.filter(
      m => m.upstreamField.trim() && m.downstreamField.trim()
    )
    if (validMappings.length === 0) {
      ElMessage.error('请至少填写一组字段映射')
      return
    }

    const data = {
      upstream_api_id: depForm.value.upstream_api_id,
      downstream_api_id: depForm.value.downstream_api_id,
      field_mappings: validMappings
    }

    if (depDialogMode.value === 'create') {
      await dependencyAPI.create(projectId, data)
      ElMessage.success('创建成功')
    } else {
      await dependencyAPI.update(projectId, editingDep.value.id, { field_mappings: validMappings })
      ElMessage.success('更新成功')
    }

    showDepDialog.value = false
    loadDependencies()
  } catch {}
}

async function viewReport(report) {
  reportDetail.value = report
  impactChain.value = []
  showReportDetail.value = true
  chainLoading.value = true

  try {
    const res = await impactReportAPI.getChain(projectId, report.id)
    impactChain.value = res.chain || []
  } catch {
    impactChain.value = []
  } finally {
    chainLoading.value = false
  }
}

function closeBatchDepDialog() {
  showBatchDepDialog.value = false
  batchDepContent.value = ''
  batchDepError.value = ''
  batchDepResult.value = null
}

async function doBatchDepImport() {
  batchDepError.value = ''
  batchDepResult.value = null
  batchDepLoading.value = true

  try {
    if (!batchDepContent.value.trim()) {
      batchDepError.value = '请输入JSON格式的依赖关系'
      batchDepLoading.value = false
      return
    }

    let items
    try {
      items = JSON.parse(batchDepContent.value)
    } catch {
      batchDepError.value = 'JSON格式不正确，请检查输入'
      batchDepLoading.value = false
      return
    }

    if (!Array.isArray(items)) {
      batchDepError.value = '输入必须是JSON数组格式'
      batchDepLoading.value = false
      return
    }

    const res = await dependencyAPI.batchCreate(projectId, items)
    batchDepResult.value = res

    if (res.created > 0) {
      ElMessage.success(`成功导入 ${res.created} 条依赖关系`)
      loadDependencies()
    } else if (res.skipped > 0) {
      ElMessage.warning(`所有依赖均已存在或接口不匹配，共跳过 ${res.skipped} 条`)
    }
  } catch (err) {
    if (err.response && err.response.data && err.response.data.error) {
      batchDepError.value = err.response.data.error
    } else {
      batchDepError.value = '导入失败，请检查网络连接或稍后重试'
    }
  } finally {
    batchDepLoading.value = false
  }
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

watch(activeTab, (newTab) => {
  if (newTab === 'dependency') {
    loadDependencies()
    loadImpactReports()
  }
})

watch(() => route.query, (query) => {
  if (query.tab === 'dependency') {
    activeTab.value = 'dependency'
    if (query.report) {
      setTimeout(() => {
        loadImpactReports().then(() => {
          const report = impactReports.value.find(r => r.id === query.report)
          if (report) {
            viewReport(report)
          }
        })
      }, 100)
    }
  }
}, { immediate: true })

function handleDependencyBreak(e) {
  if (e.detail.projectId === projectId) {
    loadImpactReports()
    if (activeTab.value === 'dependency') {
      loadImpactReports()
    }
  }
}

onMounted(() => {
  loadProject()
  loadApis()
  loadActivities()

  if (wsConnect) {
    wsConnect(projectId)
  }

  window.addEventListener('dependency-break', handleDependencyBreak)

  if (route.query.tab === 'dependency') {
    activeTab.value = 'dependency'
  }
})

onUnmounted(() => {
  if (wsClose) {
    wsClose(projectId)
  }
  window.removeEventListener('dependency-break', handleDependencyBreak)
})
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

.project-tabs {
  margin-top: 16px;
}

.project-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
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

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-divider {
  height: 1px;
  background: #e4e7ed;
  margin: 32px 0;
}

.dependency-section,
.impact-section {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

.dep-path {
  margin-left: 8px;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-weight: 500;
}

.mapping-detail {
  padding: 16px 24px;
  background: #f5f7fa;
  border-radius: 4px;
}

.mapping-title {
  font-weight: 600;
  margin-bottom: 12px;
  color: #606266;
}

.mapping-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: #fff;
  border-radius: 4px;
  margin-bottom: 8px;
}

.mapping-item:last-child {
  margin-bottom: 0;
}

.mapping-arrow {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 13px;
  color: #409eff;
}

.mapping-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.mapping-row:last-child {
  margin-bottom: 0;
}

.mapping-icon {
  color: #409eff;
  font-weight: bold;
}

.topo-section {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  margin-bottom: 20px;
}

.topo-legend {
  display: flex;
  gap: 16px;
  align-items: center;
  font-size: 13px;
  color: #606266;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.legend-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  display: inline-block;
}

.dep-actions {
  display: flex;
  gap: 8px;
}

.batch-dep-hint {
  margin-bottom: 12px;
  font-size: 13px;
  color: #606266;
}

.batch-dep-example {
  background: #f5f7fa;
  border-radius: 4px;
  padding: 12px;
  font-size: 12px;
  font-family: SF Mono, Fira Code, monospace;
  margin-top: 8px;
  color: #303133;
  overflow-x: auto;
}

.report-detail-content {
  text-align: left;
}

.report-detail-section {
  margin-bottom: 16px;
}

.report-detail-label {
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}

.report-detail-list {
  padding-left: 20px;
  margin: 0;
}

.report-detail-list li {
  margin-bottom: 8px;
  line-height: 1.6;
}

.affected-mapping-text {
  color: #909399;
  font-size: 12px;
  margin-top: 2px;
}

.chain-tree {
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 16px;
  background: #fafafa;
}

.chain-node-item {
  padding: 8px 0;
}

.chain-node-header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.chain-children {
  margin-left: 32px;
  padding-left: 12px;
  border-left: 2px solid #dcdfe6;
}

.chain-loading {
  text-align: center;
  padding: 16px;
  color: #909399;
}
</style>
