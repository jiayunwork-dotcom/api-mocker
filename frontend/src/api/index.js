import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000
})

api.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  response => response.data,
  error => {
    if (error.response) {
      const { status, data } = error.response
      if (status === 401) {
        localStorage.removeItem('token')
        localStorage.removeItem('user')
        router.push('/login')
        ElMessage.error('登录已过期，请重新登录')
      } else if (data && data.error) {
        ElMessage.error(data.error)
      } else {
        ElMessage.error('请求失败')
      }
    } else {
      ElMessage.error('网络错误')
    }
    return Promise.reject(error)
  }
)

export const authAPI = {
  register: (data) => api.post('/register', data),
  login: (data) => api.post('/login', data),
  getMe: () => api.get('/me')
}

export const workspaceAPI = {
  list: () => api.get('/workspaces'),
  create: (data) => api.post('/workspaces', data),
  get: (id) => api.get(`/workspaces/${id}`),
  update: (id, data) => api.put(`/workspaces/${id}`, data),
  delete: (id) => api.delete(`/workspaces/${id}`),
  listMembers: (id) => api.get(`/workspaces/${id}/members`),
  inviteMember: (id, data) => api.post(`/workspaces/${id}/members/invite`, data),
  join: (data) => api.post('/workspaces/join', data),
  updateMemberRole: (wsId, memberId, data) => api.put(`/workspaces/${wsId}/members/${memberId}`, data),
  removeMember: (wsId, memberId) => api.delete(`/workspaces/${wsId}/members/${memberId}`)
}

export const projectAPI = {
  list: (wsId) => api.get(`/workspaces/${wsId}/projects`),
  create: (wsId, data) => api.post(`/workspaces/${wsId}/projects`, data),
  get: (wsId, id) => api.get(`/workspaces/${wsId}/projects/${id}`),
  update: (wsId, id, data) => api.put(`/workspaces/${wsId}/projects/${id}`, data),
  delete: (wsId, id) => api.delete(`/workspaces/${wsId}/projects/${id}`)
}

export const apiDefAPI = {
  list: (projectId) => api.get(`/projects/${projectId}/apis`),
  create: (projectId, data) => api.post(`/projects/${projectId}/apis`, data),
  import: (projectId, data) => api.post(`/projects/${projectId}/apis/import`, data),
  importFile: (projectId, file) => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post(`/projects/${projectId}/apis/import`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
  },
  get: (projectId, id) => api.get(`/projects/${projectId}/apis/${id}`),
  update: (projectId, id, data) => api.put(`/projects/${projectId}/apis/${id}`, data),
  delete: (projectId, id) => api.delete(`/projects/${projectId}/apis/${id}`)
}

export const scenarioAPI = {
  list: (projectId, apiId) => api.get(`/projects/${projectId}/apis/${apiId}/scenarios`),
  create: (projectId, apiId, data) => api.post(`/projects/${projectId}/apis/${apiId}/scenarios`, data),
  update: (projectId, apiId, id, data) => api.put(`/projects/${projectId}/apis/${apiId}/scenarios/${id}`, data),
  delete: (projectId, apiId, id) => api.delete(`/projects/${projectId}/apis/${apiId}/scenarios/${id}`)
}

export const versionAPI = {
  list: (projectId, apiId) => api.get(`/projects/${projectId}/apis/${apiId}/versions`),
  get: (projectId, apiId, id) => api.get(`/projects/${projectId}/apis/${apiId}/versions/${id}`),
  diff: (projectId, apiId, id) => api.get(`/projects/${projectId}/apis/${apiId}/versions/${id}/diff`),
  rollback: (projectId, apiId, id) => api.post(`/projects/${projectId}/apis/${apiId}/versions/${id}/rollback`)
}

export const modelAPI = {
  list: (projectId) => api.get(`/projects/${projectId}/models`),
  create: (projectId, data) => api.post(`/projects/${projectId}/models`, data),
  get: (projectId, id) => api.get(`/projects/${projectId}/models/${id}`),
  update: (projectId, id, data) => api.put(`/projects/${projectId}/models/${id}`, data),
  delete: (projectId, id) => api.delete(`/projects/${projectId}/models/${id}`)
}

export const codegenAPI = {
  generate: (data) => api.post('/codegen', data)
}

export const exportAPI = {
  openapi: (data) => api.post('/export/openapi', data),
  markdown: (data) => api.post('/export/markdown', data),
  curl: (data) => api.post('/export/curl', data)
}

export const activityAPI = {
  list: (projectId, params) => api.get(`/projects/${projectId}/activities`, { params })
}

export const probeAPI = {
  list: (projectId) => api.get(`/projects/${projectId}/probes`),
  create: (projectId, data) => api.post(`/projects/${projectId}/probes`, data),
  get: (projectId, probeId) => api.get(`/projects/${projectId}/probes/${probeId}`),
  update: (projectId, probeId, data) => api.put(`/projects/${projectId}/probes/${probeId}`, data),
  delete: (projectId, probeId) => api.delete(`/projects/${projectId}/probes/${probeId}`),
  dashboard: (projectId) => api.get(`/projects/${projectId}/probes/dashboard`),
  alerts: (projectId) => api.get(`/projects/${projectId}/probes/alerts`),
  getForAPI: (projectId, apiId) => api.get(`/projects/${projectId}/apis/${apiId}/probe`),
  createForAPI: (projectId, apiId, data) => api.post(`/projects/${projectId}/apis/${apiId}/probe`, data)
}

export default api
