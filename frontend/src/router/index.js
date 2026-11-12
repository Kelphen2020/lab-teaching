import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/login', component: () => import('../views/Login.vue') },
  { path: '/student-exam', component: () => import('../views/StudentExam.vue') },
  { path: '/403', component: () => import('../views/Forbidden.vue') },
  { path: '/404', component: () => import('../views/NotFound.vue') },
  {
    path: '/',
    component: () => import('../views/Layout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', component: () => import('../views/Dashboard.vue') },
      { path: 'rooms', component: () => import('../views/Rooms.vue') },
      { path: 'agents', component: () => import('../views/Agents.vue') },
      { path: 'broadcast', component: () => import('../views/Broadcast.vue') },
      { path: 'files', component: () => import('../views/Files.vue') },
      { path: 'remote', component: () => import('../views/Remote.vue') },
      { path: 'exam', component: () => import('../views/Exam.vue') },
      { path: 'courses', component: () => import('../views/Courses.vue') },
      { path: 'clazzes', component: () => import('../views/Clazzes.vue') },
      { path: 'students', component: () => import('../views/Students.vue') },
      { path: 'schedules', component: () => import('../views/Schedules.vue') },
      { path: 'campuses', component: () => import('../views/Campuses.vue') },
      { path: 'edusync', component: () => import('../views/EduSync.vue') },
      { path: 'access', component: () => import('../views/Access.vue') },
      { path: 'license', component: () => import('../views/License.vue') },
      { path: 'ops', component: () => import('../views/Ops.vue') },
      { path: 'monitor', component: () => import('../views/Monitor.vue') },
      { path: 'wall', component: () => import('../views/MonitoringWall.vue') },
      { path: 'power-tasks', component: () => import('../views/PowerTasks.vue') },
      { path: 'collect', component: () => import('../views/CollectTasks.vue') },
      { path: 'api-docs', component: () => import('../views/ApiDocs.vue') },
      { path: 'audits', component: () => import('../views/Audits.vue') },
      { path: 'screenshots', component: () => import('../views/Screenshots.vue') },
      { path: 'desktop-services', component: () => import('../views/DesktopServices.vue') },
      { path: 'settings', component: () => import('../views/Settings.vue') }
    ]
  },
  // 兜底：未匹配路由 → 404
  { path: '/:pathMatch(.*)*', component: () => import('../views/NotFound.vue') }
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach((to, from, next) => {
  const auth = useAuthStore()
  const publicPaths = ['/login', '/student-exam']
  if (!publicPaths.includes(to.path) && !auth.isLogin) next('/login')
  else next()
})

export default router
