import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '../src/stores/auth'

describe('Auth Store', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('starts logged out', () => {
    const store = useAuthStore()
    expect(store.isLoggedIn).toBe(false)
    expect(store.isSuperAdmin).toBe(false)
    expect(store.isBuildingAdmin).toBe(false)
  })

  it('login sets auth data', () => {
    const store = useAuthStore()
    store.login(
      { username: 'admin', role: 'super_admin', building_id: '1' },
      'test-token',
      'test-refresh'
    )
    expect(store.isLoggedIn).toBe(true)
    expect(store.isSuperAdmin).toBe(true)
    expect(store.username).toBe('admin')
    expect(store.token).toBe('test-token')
  })

  it('logout clears all data', () => {
    const store = useAuthStore()
    store.login(
      { username: 'admin', role: 'super_admin' },
      'token',
      'refresh'
    )
    store.logout()
    expect(store.isLoggedIn).toBe(false)
    expect(store.token).toBe('')
    expect(store.username).toBe('')
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('getDashboardPath returns correct path', () => {
    const store = useAuthStore()
    store.login(
      { username: 'super', role: 'super_admin' },
      't', 'r'
    )
    expect(store.getDashboardPath()).toBe('/admin/buildings')

    store.logout()
    store.login(
      { username: 'landlord', role: 'building_admin' },
      't', 'r'
    )
    expect(store.getDashboardPath()).toBe('/landlord/rooms')
  })

  it('isBuildingAdmin works for admin and building_admin roles', () => {
    const store = useAuthStore()
    store.login({ username: 'a', role: 'admin' }, 't', 'r')
    expect(store.isBuildingAdmin).toBe(true)

    store.logout()
    store.login({ username: 'b', role: 'building_admin' }, 't', 'r')
    expect(store.isBuildingAdmin).toBe(true)

    store.logout()
    store.login({ username: 'c', role: 'super_admin' }, 't', 'r')
    expect(store.isBuildingAdmin).toBe(false)
  })
})
