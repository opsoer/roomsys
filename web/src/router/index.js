import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  // ===== 公共页面 =====
  {
    path: '/',
    name: 'Home',
    component: () => import('../views/BuildingList.vue'),
  },
  {
    path: '/building/:id',
    name: 'BuildingPublic',
    component: () => import('../views/BuildingPublic.vue'),
  },
  {
    path: '/building/:bid/room/:id',
    name: 'BuildingPublicRoom',
    component: () => import('../views/BuildingPublicRoom.vue'),
  },

  // ===== 平台管理员后台 =====
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
  },
  {
    path: '/admin',
    component: () => import('../views/AdminLayout.vue'),
    redirect: '/admin/buildings',
    children: [
      {
        path: 'buildings',
        name: 'AdminBuildings',
        component: () => import('../views/AdminBuildings.vue'),
      },
    ],
  },

  // ===== 公寓管理后台 =====
  {
    path: '/landlord/login',
    name: 'LandlordLogin',
    component: () => import('../views/Login.vue'),
  },
  {
    path: '/landlord',
    component: () => import('../views/BuildingLayout.vue'),
    redirect: '/landlord/rooms',
    children: [
      { path: 'rooms', name: 'LandlordRooms', component: () => import('../views/RoomList.vue') },
      { path: 'rooms/:id', name: 'LandlordRoomDetail', component: () => import('../views/RoomDetail.vue') },
      { path: 'bills', name: 'LandlordBills', component: () => import('../views/Bills.vue') },
      { path: 'dividends', name: 'LandlordDividends', component: () => import('../views/Dividends.vue') },
      { path: 'users', name: 'LandlordUsers', component: () => import('../views/UserManage.vue') },
      { path: 'tasks', name: 'LandlordTasks', component: () => import('../views/Tasks.vue') },
      { path: 'settings', name: 'LandlordSettings', component: () => import('../views/BuildingSettings.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _, next) => {
  const token = localStorage.getItem('token')
  const role = localStorage.getItem('role')

  // 公寓管理后台需要登录
  if (to.path.startsWith('/landlord') && to.path !== '/landlord/login') {
    if (!token) {
      next('/landlord/login')
      return
    }
    if (role !== 'building_admin' && role !== 'admin' && role !== 'super_admin') {
      next('/landlord/login')
      return
    }
  }

  // 平台管理后台需要 super_admin
  if (to.path.startsWith('/admin') && to.path !== '/login') {
    if (!token) {
      next('/login')
      return
    }
    if (role !== 'super_admin') {
      next('/login')
      return
    }
  }

  next()
})

export default router
