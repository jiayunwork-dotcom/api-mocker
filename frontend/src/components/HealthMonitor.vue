<template>
  <div class="health-monitor">
    <div class="summary-cards">
      <div class="summary-card healthy" @click="filterStatus = filterStatus === 'healthy' ? '' : 'healthy'">
        <div class="card-number">{{ dashboard.summary?.healthy || 0 }}</div>
        <div class="card-label">正常</div>
      </div>
      <div class="summary-card degraded" @click="filterStatus = filterStatus === 'degraded' ? '' : 'degraded'">
        <div class="card-number">{{ dashboard.summary?.degraded || 0 }}</div>
        <div class="card-label">降级</div>
      </div>
      <div class="summary-card unhealthy" @click="filterStatus = filterStatus === 'unhealthy' ? '' : 'unhealthy'">
        <div class="card-number">{{ dashboard.summary?.unhealthy || 0 }}</div>
        <div class="card-label">异常</div>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-button type="primary" size="small" @click="showAddProbe = true">添加探针</el-button>
        <span class="toolbar-hint">已启用 {{ enabledCount }}/20</span>
      </div>
      <div class="group-filter">
        <span class="filter-label">分组筛选:</span>
        <el-radio-group v-model="filterGroup" size="small" @change="loadDashboard">
          <el-radio-button label="">全部</el-radio-button>
          <el-radio-button v-for="g in dashboard.groups || []" :key="g" :label="g">{{ g }}</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <div class="batch-bar" v-if="selectedProbes.length > 0">
      <span class="batch-info">已选择 {{ selectedProbes.length }} 项</span>
      <el-button size="small" type="success" @click="handleBatchEnable">批量启用</el-button>
      <el-button size="small" type="warning" @click="handleBatchDisable">批量禁用</el-button>
      <el-button size="small" type="danger" @click="handleBatchDelete">批量删除</el-button>
      <el-button size="small" @click="clearSelection">取消选择</el-button>
    </div>

    <el-table
      :data="filteredProbes"
      style="width: 100%"
      row-key="id"
      @row-click="toggleExpand"
      :row-class-name="getRowClass"
      :expand-row-keys="expandedRows"
      ref="probeTable"
      @selection-change="handleSelectionChange"
    >
      <el-table-column type="selection" width="50" @click.stop />
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-content" v-if="detailData[row.id]">
            <div class="detail-section">
              <h4>探测时间线（最近50次）</h4>
              <div class="timeline-chart">
                <div class="chart-container" :ref="el => setChartRef(row.id, el)">
                  <canvas :id="'chart-' + row.id" width="700" height="200"></canvas>
                </div>
              </div>
            </div>
            <div class="detail-section">
              <h4>24小时可用率趋势</h4>
              <div class="availability-chart">
                <div class="chart-container" :ref="el => setAvailabilityChartRef(row.id, el)">
                  <canvas :id="'availability-chart-' + row.id" width="700" height="180"></canvas>
                </div>
              </div>
              <div class="availability-summary" v-if="availabilityData[row.id]">
                最近24小时整体可用率: <strong>{{ availabilityData[row.id].overallRate?.toFixed?.(2) || 0 }}%</strong>
                （共 {{ availabilityData[row.id].totalCount || 0 }} 次探测，成功 {{ availabilityData[row.id].successCount || 0 }} 次）
              </div>
            </div>
            <div class="detail-section">
              <h4>告警事件</h4>
              <el-table :data="detailData[row.id]?.alerts || []" size="small" max-height="250">
                <el-table-column prop="probe_name" label="探针" width="180" />
                <el-table-column label="状态变更" width="200">
                  <template #default="{ row: alert }">
                    <el-tag :type="statusTagType(alert.old_status)" size="small">{{ statusLabel(alert.old_status) }}</el-tag>
                    <span style="margin: 0 6px">→</span>
                    <el-tag :type="statusTagType(alert.new_status)" size="small">{{ statusLabel(alert.new_status) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="耗时" width="100">
                  <template #default="{ row: alert }">{{ alert.last_response_time_ms }}ms</template>
                </el-table-column>
                <el-table-column label="状态码" width="100">
                  <template #default="{ row: alert }">{{ alert.last_status_code }}</template>
                </el-table-column>
                <el-table-column label="触发时间" min-width="180">
                  <template #default="{ row: alert }">{{ formatTime(alert.triggered_at) }}</template>
                </el-table-column>
              </el-table>
              <el-empty v-if="!detailData[row.id]?.alerts?.length" description="暂无告警事件" :image-size="40" />
            </div>
            <div class="detail-section">
              <h4>探针配置</h4>
              <div class="config-row">
                <span>分组: <strong>{{ getProbeConfig(row.id)?.group_name || '未分组' }}</strong></span>
                <span>检查间隔: <strong>{{ getProbeConfig(row.id)?.interval_seconds }}s</strong></span>
                <span>超时阈值: <strong>{{ getProbeConfig(row.id)?.timeout_ms }}ms</strong></span>
              </div>
              <div class="config-row">
                <span>连续失败标记异常: <strong>{{ getProbeConfig(row.id)?.fail_threshold }}次</strong></span>
                <span>连续成功恢复: <strong>{{ getProbeConfig(row.id)?.recover_threshold }}次</strong></span>
              </div>
              <div class="config-actions">
                <el-button size="small" @click.stop="openEditProbe(row)">编辑配置</el-button>
                <el-button size="small" type="danger" @click.stop="handleDeleteProbe(row)">删除探针</el-button>
              </div>
            </div>
          </div>
          <div v-else class="expand-loading">
            <el-icon class="is-loading"><Loading /></el-icon> 加载中...
          </div>
        </template>
      </el-table-column>
      <el-table-column label="方法" width="90">
        <template #default="{ row }">
          <el-tag :type="methodTagType(row.apiMethod)" size="small">{{ row.apiMethod }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="apiPath" label="接口路径" min-width="200">
        <template #default="{ row }">
          <span class="api-path-text">{{ row.apiPath }}</span>
          <span v-if="row.apiDescription" class="api-desc-text">{{ row.apiDescription }}</span>
          <el-tag v-if="row.groupName" size="small" type="info" class="group-tag">{{ row.groupName }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" size="small" effect="dark">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最近耗时" width="110" align="right">
        <template #default="{ row }">
          <span :class="{'text-danger': row.lastResponseMs > 1000}">{{ row.lastResponseMs }}ms</span>
        </template>
      </el-table-column>
      <el-table-column label="平均耗时" width="110" align="right">
        <template #default="{ row }">{{ row.avgResponseMs }}ms</template>
      </el-table-column>
      <el-table-column label="成功率" width="100" align="right">
        <template #default="{ row }">
          <span :class="{'text-danger': row.successRate < 80, 'text-success': row.successRate >= 95}">
            {{ row.successRate.toFixed(1) }}%
          </span>
        </template>
      </el-table-column>
      <el-table-column label="最后检查" width="170">
        <template #default="{ row }">
          <span class="time-text">{{ row.lastCheckTime ? formatTime(row.lastCheckTime) : '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="开关" width="80" align="center">
        <template #default="{ row }">
          <el-switch
            :model-value="row.enabled"
            size="small"
            @change="(val) => toggleProbe(row, val)"
            @click.stop
          />
        </template>
      </el-table-column>
    </el-table>

    <el-empty v-if="!dashboard.probes?.length" description="暂无启用的探针，点击上方按钮添加" />

    <el-dialog v-model="showAddProbe" title="添加健康探针" width="520px" :close-on-click-modal="false">
      <el-form :model="addForm" label-width="120px" size="default">
        <el-form-item label="选择接口" required>
          <el-select v-model="addForm.apiId" placeholder="选择要监控的接口" filterable style="width:100%">
            <el-option
              v-for="api in availableAPIs"
              :key="api.id"
              :label="`${api.method} ${api.path}`"
              :value="api.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="分组名称">
          <el-input v-model="addForm.groupName" placeholder="可选，例如：核心接口" />
        </el-form-item>
        <el-form-item label="启用探针">
          <el-switch v-model="addForm.enabled" />
        </el-form-item>
        <el-form-item label="检查间隔(秒)">
          <el-input-number v-model="addForm.intervalSeconds" :min="10" :max="300" :step="10" />
        </el-form-item>
        <el-form-item label="超时阈值(ms)">
          <el-input-number v-model="addForm.timeoutMs" :min="500" :max="30000" :step="500" />
        </el-form-item>
        <el-form-item label="失败次数阈值">
          <el-input-number v-model="addForm.failThreshold" :min="1" :max="20" />
        </el-form-item>
        <el-form-item label="恢复成功次数">
          <el-input-number v-model="addForm.recoverThreshold" :min="1" :max="20" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddProbe = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleAddProbe">确认</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showEditProbe" title="编辑探针配置" width="480px" :close-on-click-modal="false">
      <el-form :model="editForm" label-width="120px" size="default" v-if="editForm">
        <el-form-item label="分组名称">
          <el-input v-model="editForm.groupName" placeholder="可选，留空表示未分组" />
        </el-form-item>
        <el-form-item label="检查间隔(秒)">
          <el-input-number v-model="editForm.intervalSeconds" :min="10" :max="300" :step="10" />
        </el-form-item>
        <el-form-item label="超时阈值(ms)">
          <el-input-number v-model="editForm.timeoutMs" :min="500" :max="30000" :step="500" />
        </el-form-item>
        <el-form-item label="失败次数阈值">
          <el-input-number v-model="editForm.failThreshold" :min="1" :max="20" />
        </el-form-item>
        <el-form-item label="恢复成功次数">
          <el-input-number v-model="editForm.recoverThreshold" :min="1" :max="20" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditProbe = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleEditProbe">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { probeAPI, apiDefAPI } from '../api'

const props = defineProps({
  projectId: { type: String, required: true }
})

const dashboard = ref({ summary: {}, probes: [], groups: [] })
const allProbes = ref([])
const allAPIs = ref([])
const filterStatus = ref('')
const filterGroup = ref('')
const expandedRows = ref([])
const detailData = ref({})
const chartRefs = ref({})
const availabilityChartRefs = ref({})
const availabilityData = ref({})
const selectedProbes = ref([])

const showAddProbe = ref(false)
const showEditProbe = ref(false)
const saving = ref(false)

const addForm = ref({
  apiId: '',
  enabled: false,
  groupName: '',
  intervalSeconds: 30,
  timeoutMs: 3000,
  failThreshold: 3,
  recoverThreshold: 2
})

const editForm = ref(null)
const editingProbeId = ref('')

let refreshTimer = null
let ws = null

const enabledCount = computed(() => dashboard.value.probes?.filter(p => p.enabled).length || 0)

const filteredProbes = computed(() => {
  let list = dashboard.value.probes || []
  if (filterStatus.value) {
    list = list.filter(p => p.status === filterStatus.value)
  }
  return list
})

const availableAPIs = computed(() => {
  const probeApiIds = new Set(allProbes.value.map(p => p.api_id))
  return allAPIs.value.filter(a => !probeApiIds.has(a.id))
})

function setChartRef(probeId, el) {
  if (el) chartRefs.value[probeId] = el
}

function setAvailabilityChartRef(probeId, el) {
  if (el) availabilityChartRefs.value[probeId] = el
}

async function loadDashboard() {
  try {
    const params = {}
    if (filterGroup.value) {
      params.groupName = filterGroup.value
    }
    const res = await probeAPI.dashboard(props.projectId, params)
    dashboard.value = res
  } catch {}
}

async function loadAllProbes() {
  try {
    const params = {}
    if (filterGroup.value) {
      params.groupName = filterGroup.value
    }
    const res = await probeAPI.list(props.projectId, params)
    allProbes.value = res.probes || []
  } catch {}
}

async function loadAPIs() {
  try {
    const res = await apiDefAPI.list(props.projectId)
    allAPIs.value = res.apis || []
  } catch {}
}

function toggleExpand(row) {
  const idx = expandedRows.value.indexOf(row.id)
  if (idx >= 0) {
    expandedRows.value.splice(idx, 1)
    return
  }
  expandedRows.value.push(row.id)
  loadProbeDetail(row.id)
}

async function loadProbeDetail(probeId) {
  try {
    const res = await probeAPI.get(props.projectId, probeId)
    detailData.value[probeId] = { records: res.records || [], alerts: res.alerts || [] }
    await nextTick()
    drawChart(probeId)
    loadAvailabilityTrend(probeId)
  } catch {}
}

async function loadAvailabilityTrend(probeId) {
  try {
    const res = await probeAPI.availabilityTrend(props.projectId, probeId)
    availabilityData.value[probeId] = res
    await nextTick()
    drawAvailabilityChart(probeId)
  } catch {}
}

function drawChart(probeId) {
  const canvas = document.getElementById('chart-' + probeId)
  if (!canvas) return

  const records = detailData.value[probeId]?.records || []
  if (!records.length) return

  const ctx = canvas.getContext('2d')
  const W = canvas.width
  const H = canvas.height
  const padL = 50, padR = 20, padT = 20, padB = 40
  const chartW = W - padL - padR
  const chartH = H - padT - padB

  ctx.clearRect(0, 0, W, H)

  const sortedRecords = [...records].reverse()
  const maxMs = Math.max(100, ...sortedRecords.map(r => r.response_time_ms))
  const yMax = Math.ceil(maxMs / 500) * 500

  ctx.strokeStyle = '#e8e8e8'
  ctx.lineWidth = 1
  ctx.font = '11px sans-serif'
  ctx.fillStyle = '#999'
  ctx.textAlign = 'right'

  for (let i = 0; i <= 4; i++) {
    const y = padT + (chartH / 4) * i
    ctx.beginPath()
    ctx.moveTo(padL, y)
    ctx.lineTo(W - padR, y)
    ctx.stroke()
    const val = Math.round(yMax - (yMax / 4) * i)
    ctx.fillText(val + 'ms', padL - 6, y + 4)
  }

  const step = chartW / Math.max(sortedRecords.length - 1, 1)

  ctx.beginPath()
  ctx.strokeStyle = '#409eff'
  ctx.lineWidth = 1.5
  sortedRecords.forEach((r, i) => {
    const x = padL + step * i
    const y = padT + chartH * (1 - r.response_time_ms / yMax)
    if (i === 0) ctx.moveTo(x, y)
    else ctx.lineTo(x, y)
  })
  ctx.stroke()

  sortedRecords.forEach((r, i) => {
    const x = padL + step * i
    const y = padT + chartH * (1 - r.response_time_ms / yMax)
    ctx.beginPath()
    ctx.arc(x, y, 3, 0, Math.PI * 2)
    ctx.fillStyle = r.is_success ? '#409eff' : '#f56c6c'
    ctx.fill()
  })

  if (sortedRecords.length > 0) {
    ctx.fillStyle = '#999'
    ctx.textAlign = 'center'
    ctx.font = '10px sans-serif'
    const labelInterval = Math.max(1, Math.floor(sortedRecords.length / 6))
    sortedRecords.forEach((r, i) => {
      if (i % labelInterval === 0 || i === sortedRecords.length - 1) {
        const x = padL + step * i
        const t = new Date(r.checked_at)
        ctx.fillText(`${t.getHours().toString().padStart(2,'0')}:${t.getMinutes().toString().padStart(2,'0')}`, x, H - padB + 18)
      }
    })
  }
}

function drawAvailabilityChart(probeId) {
  const canvas = document.getElementById('availability-chart-' + probeId)
  if (!canvas) return

  const data = availabilityData.value[probeId]?.hours || []
  if (!data.length) return

  const ctx = canvas.getContext('2d')
  const W = canvas.width
  const H = canvas.height
  const padL = 50, padR = 20, padT = 20, padB = 40
  const chartW = W - padL - padR
  const chartH = H - padT - padB

  ctx.clearRect(0, 0, W, H)

  ctx.strokeStyle = '#e8e8e8'
  ctx.lineWidth = 1
  ctx.font = '11px sans-serif'
  ctx.fillStyle = '#999'
  ctx.textAlign = 'right'

  const yTicks = [0, 25, 50, 75, 100]
  yTicks.forEach((val, i) => {
    const y = padT + (chartH / 4) * (4 - i)
    ctx.beginPath()
    ctx.moveTo(padL, y)
    ctx.lineTo(W - padR, y)
    ctx.stroke()
    ctx.fillText(val + '%', padL - 6, y + 4)
  })

  const step = chartW / Math.max(data.length - 1, 1)

  ctx.beginPath()
  ctx.strokeStyle = '#67c23a'
  ctx.lineWidth = 2
  let isFirstPoint = true
  let prevHasData = false

  data.forEach((d, i) => {
    const x = padL + step * i
    const y = padT + chartH * (1 - (d.successRate || 0) / 100)

    if (d.hasData) {
      if (isFirstPoint || !prevHasData) {
        ctx.moveTo(x, y)
        isFirstPoint = false
      } else {
        ctx.lineTo(x, y)
      }
      prevHasData = true
    } else {
      prevHasData = false
      isFirstPoint = true
    }
  })
  ctx.stroke()

  data.forEach((d, i) => {
    if (!d.hasData) return
    const x = padL + step * i
    const y = padT + chartH * (1 - (d.successRate || 0) / 100)
    ctx.beginPath()
    ctx.arc(x, y, 3, 0, Math.PI * 2)
    ctx.fillStyle = d.successRate >= 95 ? '#67c23a' : d.successRate >= 80 ? '#e6a23c' : '#f56c6c'
    ctx.fill()
  })

  ctx.fillStyle = '#999'
  ctx.textAlign = 'center'
  ctx.font = '10px sans-serif'
  const labelInterval = Math.max(1, Math.floor(data.length / 6))
  data.forEach((d, i) => {
    if (i % labelInterval === 0 || i === data.length - 1) {
      const x = padL + step * i
      const hour = d.hour.split(' ')[1]?.split(':')[0] || ''
      ctx.fillText(hour + '时', x, H - padB + 18)
    }
  })
}

async function handleAddProbe() {
  if (!addForm.value.apiId) {
    ElMessage.warning('请选择要监控的接口')
    return
  }
  saving.value = true
  try {
    await probeAPI.create(props.projectId, {
      apiId: addForm.value.apiId,
      enabled: addForm.value.enabled,
      groupName: addForm.value.groupName,
      intervalSeconds: addForm.value.intervalSeconds,
      timeoutMs: addForm.value.timeoutMs,
      failThreshold: addForm.value.failThreshold,
      recoverThreshold: addForm.value.recoverThreshold
    })
    ElMessage.success('探针已创建')
    showAddProbe.value = false
    addForm.value = { apiId: '', enabled: false, groupName: '', intervalSeconds: 30, timeoutMs: 3000, failThreshold: 3, recoverThreshold: 2 }
    loadDashboard()
    loadAllProbes()
  } catch (err) {
    const msg = err.response?.data?.error || '创建失败'
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

function openEditProbe(row) {
  const cfg = getProbeConfig(row.id)
  if (!cfg) return
  editingProbeId.value = row.id
  editForm.value = {
    groupName: cfg.group_name || '',
    intervalSeconds: cfg.interval_seconds,
    timeoutMs: cfg.timeout_ms,
    failThreshold: cfg.fail_threshold,
    recoverThreshold: cfg.recover_threshold
  }
  showEditProbe.value = true
}

async function handleEditProbe() {
  saving.value = true
  try {
    await probeAPI.update(props.projectId, editingProbeId.value, editForm.value)
    ElMessage.success('配置已更新')
    showEditProbe.value = false
    loadDashboard()
    loadAllProbes()
    if (detailData.value[editingProbeId.value]) {
      loadProbeDetail(editingProbeId.value)
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || '更新失败')
  } finally {
    saving.value = false
  }
}

async function handleDeleteProbe(row) {
  try {
    await ElMessageBox.confirm('确定删除该探针？探测记录和告警事件将一并删除', '确认删除')
    await probeAPI.delete(props.projectId, row.id)
    ElMessage.success('已删除')
    expandedRows.value = expandedRows.value.filter(id => id !== row.id)
    delete detailData.value[row.id]
    loadDashboard()
    loadAllProbes()
  } catch {}
}

async function toggleProbe(row, val) {
  try {
    await probeAPI.update(props.projectId, row.id, { enabled: val })
    ElMessage.success(val ? '探针已启用' : '探针已停用')
    loadDashboard()
    loadAllProbes()
  } catch (err) {
    ElMessage.error(err.response?.data?.error || '操作失败')
  }
}

function handleSelectionChange(selection) {
  selectedProbes.value = selection
}

function clearSelection() {
  selectedProbes.value = []
}

async function handleBatchEnable() {
  if (!selectedProbes.value.length) return
  try {
    const probeIds = selectedProbes.value.map(p => p.id)
    const res = await probeAPI.batchEnable(props.projectId, { probeIds })
    let msg = `操作完成：成功 ${res.success} 条`
    if (res.skipped > 0) {
      msg += `，跳过 ${res.skipped} 条（部分因项目活跃探针达到20上限）`
    }
    if (res.failed > 0) {
      msg += `，失败 ${res.failed} 条`
    }
    ElMessage.success(msg)
    loadDashboard()
    loadAllProbes()
    clearSelection()
  } catch (err) {
    ElMessage.error(err.response?.data?.error || '操作失败')
  }
}

async function handleBatchDisable() {
  if (!selectedProbes.value.length) return
  try {
    const probeIds = selectedProbes.value.map(p => p.id)
    const res = await probeAPI.batchDisable(props.projectId, { probeIds })
    ElMessage.success(`操作完成：成功 ${res.success} 条，跳过 ${res.skipped} 条，失败 ${res.failed} 条`)
    loadDashboard()
    loadAllProbes()
    clearSelection()
  } catch (err) {
    ElMessage.error(err.response?.data?.error || '操作失败')
  }
}

async function handleBatchDelete() {
  if (!selectedProbes.value.length) return
  try {
    await ElMessageBox.confirm(`确定删除选中的 ${selectedProbes.value.length} 条探针？`, '确认批量删除', { type: 'warning' })
    const probeIds = selectedProbes.value.map(p => p.id)
    const res = await probeAPI.batchDelete(props.projectId, { probeIds })
    ElMessage.success(`操作完成：成功 ${res.success} 条，失败 ${res.failed} 条`)
    loadDashboard()
    loadAllProbes()
    clearSelection()
  } catch {}
}

function getProbeConfig(probeId) {
  return allProbes.value.find(p => p.id === probeId)
}

function statusTagType(status) {
  const map = { healthy: 'success', degraded: 'warning', unhealthy: 'danger' }
  return map[status] || 'info'
}

function statusLabel(status) {
  const map = { healthy: '正常', degraded: '降级', unhealthy: '异常' }
  return map[status] || status
}

function methodTagType(method) {
  const map = { GET: 'success', POST: 'primary', PUT: 'warning', PATCH: '', DELETE: 'danger', HEAD: 'info', OPTIONS: 'info' }
  return map[method] || ''
}

function formatTime(t) {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

function getRowClass({ row }) {
  return `row-status-${row.status || 'unknown'}`
}

function connectWebSocket() {
  if (ws) {
    ws.close()
  }

  const token = localStorage.getItem('token')
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/projects/${props.projectId}/probes/ws`

  try {
    ws = new WebSocket(wsUrl)
    if (token) {
      ws.binaryType = 'arraybuffer'
    }

    ws.onopen = () => {
      console.log('[WebSocket] Connected')
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.eventType === 'status_change') {
          handleStatusChange(data)
        }
      } catch (e) {
        console.error('[WebSocket] Parse error:', e)
      }
    }

    ws.onclose = () => {
      console.log('[WebSocket] Disconnected')
    }

    ws.onerror = (err) => {
      console.error('[WebSocket] Error:', err)
    }
  } catch (e) {
    console.error('[WebSocket] Connection failed:', e)
  }
}

function handleStatusChange(data) {
  const probes = dashboard.value.probes || []
  const probe = probes.find(p => p.id === data.probeId)

  if (probe) {
    const oldStatus = probe.status
    probe.status = data.newStatus
    probe.lastResponseMs = data.lastResponseMs
    probe.lastCheckTime = data.triggeredAt

    const summary = dashboard.value.summary || {}
    if (summary[oldStatus] !== undefined) {
      summary[oldStatus] = Math.max(0, summary[oldStatus] - 1)
    }
    if (summary[data.newStatus] !== undefined) {
      summary[data.newStatus] = (summary[data.newStatus] || 0) + 1
    }
  }
}

watch(() => props.projectId, () => {
  loadDashboard()
  loadAllProbes()
  loadAPIs()
  connectWebSocket()
})

onMounted(() => {
  loadDashboard()
  loadAllProbes()
  loadAPIs()
  connectWebSocket()
  refreshTimer = setInterval(() => {
    loadDashboard()
    loadAllProbes()
  }, 8000)
})

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (ws) ws.close()
})
</script>

<style scoped>
.health-monitor {
  padding: 0;
}

.summary-cards {
  display: flex;
  gap: 16px;
  margin-bottom: 20px;
}

.summary-card {
  flex: 1;
  padding: 20px 24px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
}

.summary-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}

.summary-card.healthy {
  background: linear-gradient(135deg, #f0f9eb, #e1f3d8);
  border: 1px solid #c2e7b0;
}

.summary-card.degraded {
  background: linear-gradient(135deg, #fdf6ec, #faecd8);
  border: 1px solid #f5dab1;
}

.summary-card.unhealthy {
  background: linear-gradient(135deg, #fef0f0, #fde2e2);
  border: 1px solid #fbc4c4;
}

.card-number {
  font-size: 36px;
  font-weight: 700;
  line-height: 1;
}

.summary-card.healthy .card-number { color: #67c23a; }
.summary-card.degraded .card-number { color: #e6a23c; }
.summary-card.unhealthy .card-number { color: #f56c6c; }

.card-label {
  margin-top: 6px;
  font-size: 14px;
  color: #606266;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toolbar-hint {
  font-size: 13px;
  color: #909399;
}

.group-filter {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-label {
  font-size: 13px;
  color: #606266;
}

.batch-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  background: #ecf5ff;
  border: 1px solid #d9ecff;
  border-radius: 6px;
  margin-bottom: 12px;
}

.batch-info {
  font-size: 13px;
  color: #409eff;
  margin-right: 8px;
}

.api-path-text {
  font-weight: 600;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 13px;
}

.api-desc-text {
  color: #999;
  font-size: 12px;
  margin-left: 8px;
}

.group-tag {
  margin-left: 8px;
}

.time-text {
  font-size: 12px;
  color: #909399;
}

.text-danger { color: #f56c6c; }
.text-success { color: #67c23a; }

.expand-content {
  padding: 16px 24px;
  background: #fafbfc;
}

.expand-loading {
  padding: 24px;
  text-align: center;
  color: #909399;
}

.detail-section {
  margin-bottom: 20px;
}

.detail-section h4 {
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 12px;
  color: #303133;
}

.chart-container {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 8px;
  overflow-x: auto;
}

.timeline-chart canvas,
.availability-chart canvas {
  display: block;
}

.availability-summary {
  margin-top: 8px;
  font-size: 13px;
  color: #606266;
  text-align: center;
}

.availability-summary strong {
  color: #67c23a;
  font-size: 15px;
}

.config-row {
  display: flex;
  gap: 24px;
  font-size: 13px;
  color: #606266;
  margin-bottom: 12px;
}

.config-row strong {
  color: #303133;
}

.config-actions {
  display: flex;
  gap: 8px;
}

:deep(.row-status-unhealthy) {
  background: #fff5f5 !important;
}

:deep(.row-status-degraded) {
  background: #fffbf0 !important;
}
</style>
