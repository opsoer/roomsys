import { createRouter, createWebHistory } from 'vue-router'

const routes = [
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

  {
    path: '/apply',
    name: 'LandlordApply',
    component: () => import('../views/LandlordApply.vue'),
  },

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
      {
        path: 'recruit',
        name: 'AdminRecruit',
        component: () => import('../views/RecruitSettings.vue'),
      },
    ],
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
  { path: '/404', name: 'NotFound', component: () => import('../views/NotFound.vue') },
  { path: '/:pathMatch(.*)*', redirect: '/404' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _, next) => {
  const token = localStorage.getItem('token')
  const role = localStorage.getItem('role')

  if (to.name === 'Login' && token) {
    next(role === 'super_admin' ? '/admin/buildings' : '/landlord/rooms')
    return
  }

  if (to.path.startsWith('/landlord')) {
    if (!token) {
      next('/login')
      return
    }
    if (role !== 'building_admin' && role !== 'admin' && role !== 'super_admin') {
      next('/login')
      return
    }
  }

  if (to.path.startsWith('/admin')) {
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
