import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue')
  },
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('../views/Dashboard.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/workspace/:id',
    name: 'WorkspaceDetail',
    component: () => import('../views/WorkspaceDetail.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/project/:id',
    name: 'ProjectDetail',
    component: () => import('../views/ProjectDetail.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/project/:projectId/api/:apiId?',
    name: 'ApiEditor',
    component: () => import('../views/ApiEditor.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/project/:projectId/models',
    name: 'ModelEditor',
    component: () => import('../views/ModelEditor.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/project/:projectId/api/:apiId/versions',
    name: 'VersionHistory',
    component: () => import('../views/VersionHistory.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/project/:projectId/codegen',
    name: 'CodeGen',
    component: () => import('../views/CodeGen.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/project/:projectId/export',
    name: 'Export',
    component: () => import('../views/Export.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/join',
    name: 'JoinWorkspace',
    component: () => import('../views/JoinWorkspace.vue')
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    next('/login')
  } else {
    next()
  }
})

export default router
