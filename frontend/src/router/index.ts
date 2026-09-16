import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('../views/HomePage.vue'),
    },
    {
      // 服务详情仍复用首页（内联展开），但用随机 hash 作为可分享的 URL，
      // 避免公开页出现自增 ID
      path: '/services/:hash',
      name: 'service-detail',
      component: () => import('../views/HomePage.vue'),
    },
    {
      // 使用随机 hash 而非自增 ID 访问事件详情页
      path: '/incidents/:hash',
      name: 'incident-detail',
      component: () => import('../views/IncidentDetailPage.vue'),
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginPage.vue'),
    },
    {
      path: '/setup',
      name: 'setup',
      component: () => import('../views/SetupPage.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('../views/DashboardPage.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    next('/login')
  } else {
    next()
  }
})

export default router
