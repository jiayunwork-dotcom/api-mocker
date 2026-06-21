<template>
  <div class="app-container">
    <div
      v-if="breakAlert.show"
      class="break-alert"
      @click="handleAlertClick"
    >
      <div class="alert-content">
        <el-icon class="alert-icon"><Warning /></el-icon>
        <span>
          <strong>破坏性变更告警：</strong>
          接口 <code>{{ breakAlert.apiPath }}</code> 的变更影响了
          <strong>{{ breakAlert.affectedCount }}</strong> 个下游接口，
          <span class="alert-link">点击查看详情</span>
        </span>
      </div>
      <el-button class="alert-close" text @click.stop="breakAlert.show = false">
        <el-icon><Close /></el-icon>
      </el-button>
    </div>
    <router-view />
  </div>
</template>

<script setup>
import { ref, provide, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Warning, Close } from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()

const breakAlert = ref({
  show: false,
  apiPath: '',
  affectedCount: 0,
  reportId: '',
  projectId: ''
})

const wsConnections = ref({})
const wsReconnectAttempts = ref({})
const wsReconnectTimers = ref({})

function handleAlertClick() {
  if (breakAlert.value.projectId && breakAlert.value.reportId) {
    router.push({
      path: `/project/${breakAlert.value.projectId}`,
      query: { tab: 'dependency', report: breakAlert.value.reportId }
    })
    breakAlert.value.show = false
  }
}

function connectWebSocket(projectId) {
  if (wsConnections.value[projectId]) {
    return wsConnections.value[projectId]
  }

  clearWsReconnect(projectId)

  const token = localStorage.getItem('token')
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/projects/${projectId}/probes/ws?token=${token}`

  const ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    console.log('[WebSocket] Connection opened for project:', projectId)
    wsReconnectAttempts.value[projectId] = 0
  }

  ws.onmessage = (event) => {
    console.log('[WebSocket] Message received:', event.data)
    try {
      const data = JSON.parse(event.data)
      console.log('[WebSocket] Parsed message:', data)
      if (data.eventType === 'dependency_break') {
        console.log('[WebSocket] dependency_break event, showing alert')
        breakAlert.value = {
          show: true,
          apiPath: data.changedApiPath,
          affectedCount: data.affectedCount,
          reportId: data.reportId,
          projectId: data.projectId
        }

        if (route.params.id === data.projectId) {
          const event = new CustomEvent('dependency-break', { detail: data })
          window.dispatchEvent(event)
        }
      }
    } catch (e) {
      console.error('[WebSocket] Failed to parse message:', e, event.data)
    }
  }

  ws.onerror = (error) => {
    console.error('[WebSocket] Connection error:', error)
  }

  ws.onclose = () => {
    console.log('[WebSocket] Connection closed for project:', projectId)
    delete wsConnections.value[projectId]
    scheduleWsReconnect(projectId)
  }

  wsConnections.value[projectId] = ws
  return ws
}

function scheduleWsReconnect(projectId) {
  if (wsReconnectTimers.value[projectId]) return
  if (!wsReconnectAttempts.value[projectId]) wsReconnectAttempts.value[projectId] = 0
  const delay = Math.min(1000 * Math.pow(2, wsReconnectAttempts.value[projectId]), 30000)
  wsReconnectAttempts.value[projectId]++
  console.log(`[WebSocket] Reconnecting project ${projectId} in ${delay}ms (attempt ${wsReconnectAttempts.value[projectId]})`)
  wsReconnectTimers.value[projectId] = setTimeout(() => {
    delete wsReconnectTimers.value[projectId]
    connectWebSocket(projectId)
  }, delay)
}

function clearWsReconnect(projectId) {
  if (wsReconnectTimers.value[projectId]) {
    clearTimeout(wsReconnectTimers.value[projectId])
    delete wsReconnectTimers.value[projectId]
  }
}

function closeWebSocket(projectId) {
  clearWsReconnect(projectId)
  if (wsConnections.value[projectId]) {
    wsConnections.value[projectId].onclose = null
    wsConnections.value[projectId].close()
    delete wsConnections.value[projectId]
  }
}

provide('wsConnect', connectWebSocket)
provide('wsClose', closeWebSocket)

onMounted(() => {
  window.addEventListener('dependency-break', (e) => {
    console.log('Received dependency break event:', e.detail)
  })
})
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  background-color: #f5f7fa;
  color: #333;
}

.page-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px;
}

.card {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  margin-bottom: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h2 {
  font-size: 22px;
  font-weight: 600;
  color: #1a1a2e;
}

.method-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  color: #fff;
}

.method-get { background: #61affe; }
.method-post { background: #49cc90; }
.method-put { background: #fca130; }
.method-patch { background: #fca130; }
.method-delete { background: #f93e3e; }
.method-head { background: #9012fe; }
.method-options { background: #9012fe; }

.app-container {
  position: relative;
  min-height: 100vh;
}

.break-alert {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 9999;
  background: linear-gradient(90deg, #fef2f2, #fff5f5);
  border-bottom: 2px solid #f56c6c;
  padding: 12px 24px;
  cursor: pointer;
  animation: slideDown 0.3s ease-out;
  box-shadow: 0 2px 12px rgba(245, 108, 108, 0.2);
}

@keyframes slideDown {
  from {
    transform: translateY(-100%);
  }
  to {
    transform: translateY(0);
  }
}

.alert-content {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  gap: 12px;
  color: #f56c6c;
  font-size: 14px;
}

.alert-icon {
  font-size: 20px;
  color: #f56c6c;
}

.alert-content code {
  background: #fde2e2;
  padding: 2px 8px;
  border-radius: 4px;
  font-family: 'SF Mono', 'Fira Code', monospace;
  margin: 0 4px;
}

.alert-link {
  color: #409eff;
  text-decoration: underline;
  margin-left: 4px;
}

.alert-close {
  position: absolute;
  right: 24px;
  top: 50%;
  transform: translateY(-50%);
  color: #f56c6c;
  padding: 4px;
}

.alert-close:hover {
  color: #d9363e;
}
</style>
