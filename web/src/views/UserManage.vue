<template>
  <div>
    <h3 style="margin-bottom: 20px">管理员管理</h3>
    <el-card>
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
        <h4>管理员列表</h4>
        <!-- 楼栋管理员只能给自己公寓创建普通管理员；超管创建楼下管理员请到「公寓管理」 -->
        <el-button v-if="!isSuperAdmin" type="primary" @click="openAddDialog">添加管理员</el-button>
      </div>
      <div class="desktop-table">
        <el-table :data="users" border stripe v-loading="loading">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="username" label="用户名" />
          <el-table-column label="姓名" width="120">
            <template #default="{ row }">{{ row.display_name || '—' }}</template>
          </el-table-column>
          <el-table-column label="角色" width="140">
            <template #default="{ row }">
              <el-tag :type="row.role === 'super_admin' ? 'danger' : 'primary'" size="small">
                {{ roleLabel(row.role) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="备注" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">{{ row.remark || '—' }}</template>
          </el-table-column>
          <el-table-column label="创建时间">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="160">
            <template #default="{ row }">
              <el-button v-if="canManage(row)" size="small" type="primary" text @click="openEditDialog(row)">编辑</el-button>
              <el-button v-if="canManage(row)" size="small" type="danger" text :disabled="row.role === 'super_admin'" @click="handleDelete(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div class="mobile-cards" v-loading="loading">
        <div v-if="loading" class="skeleton-list">
          <div v-for="n in 3" :key="n" class="skeleton-item">
            <el-skeleton :rows="2" animated>
              <template #template>
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px;">
                  <el-skeleton-item variant="h3" style="width:30%;" />
                  <el-skeleton-item variant="rect" style="width:80px;height:24px;border-radius:12px;" />
                </div>
                <el-skeleton-item variant="text" style="width:50%;margin-bottom:4px;" />
                <el-skeleton-item variant="text" style="width:40%;" />
              </template>
            </el-skeleton>
          </div>
        </div>
        <template v-else>
        <div v-for="item in users" :key="item.id" class="user-card">
          <div class="uc-head">
            <span class="uc-name">{{ item.username }}</span>
            <el-tag :type="item.role === 'super_admin' ? 'danger' : 'primary'" size="small" effect="dark" round>
              {{ roleLabel(item.role) }}
            </el-tag>
          </div>
          <div v-if="item.display_name" class="uc-field">
            <span class="uc-label">姓名</span>
            <span class="uc-value">{{ item.display_name }}</span>
          </div>
          <div v-if="item.remark" class="uc-field">
            <span class="uc-label">备注</span>
            <span class="uc-value">{{ item.remark }}</span>
          </div>
          <div class="uc-field">
            <span class="uc-label">ID</span>
            <span class="uc-value">{{ item.id }}</span>
          </div>
          <div class="uc-field">
            <span class="uc-label">创建时间</span>
            <span class="uc-value">{{ formatTime(item.created_at) }}</span>
          </div>
          <div class="uc-foot">
            <el-button v-if="canManage(item)" size="small" type="primary" text @click="openEditDialog(item)">编辑</el-button>
            <el-button v-if="canManage(item)" size="small" type="danger" text :disabled="item.role === 'super_admin'" @click="handleDelete(item)">删除</el-button>
          </div>
        </div>
        <div v-if="!loading && users.length === 0" class="empty-text">暂无管理员</div>
        </template>
      </div>
    </el-card>

    <el-dialog v-model="showDialog" :title="editingId ? '编辑管理员' : '添加管理员'" width="400px">
      <el-form ref="formRef" :model="form" label-width="90px">
        <el-form-item v-if="!editingId" label="用户名" prop="username" :rules="[{ required: true, message: '请输入用户名' }, { pattern: /^[A-Za-z0-9_]{2,30}$/, message: '用户名为2-30位字母、数字或下划线' }]">
          <el-input v-model="form.username" maxlength="30" />
        </el-form-item>
        <!-- 密码：新建时必填；编辑时只有超管能改（公寓接口只管姓名/电话/备注） -->
        <el-form-item v-if="!editingId || isSuperAdmin" label="密码" prop="password" :rules="editingId ? [] : [{ required: true, message: '请输入密码' }]">
          <el-input v-model="form.password" type="password" show-password :placeholder="editingId ? '留空则不修改' : '请输入密码'" />
        </el-form-item>
        <el-form-item v-if="!editingId || isSuperAdmin" label="确认密码" prop="confirmPassword" :rules="[{ required: !editingId, message: '请再次输入密码' }, { validator: validatePasswordConfirm, trigger: 'blur' }]">
          <el-input v-model="form.confirmPassword" type="password" show-password :placeholder="editingId ? '修改密码时需再次输入' : '再次输入密码'" />
        </el-form-item>
        <el-alert v-if="editingId && !isSuperAdmin" type="info" :closable="false" show-icon
          title="只能修改姓名/电话/备注；忘记密码请删除该账号后重新创建" style="margin-bottom:12px" />
        <el-form-item v-if="isSuperAdmin" label="角色" prop="role">
          <el-radio-group v-model="form.role">
            <el-radio value="admin">普通管理员</el-radio>
            <el-radio value="super_admin">超级管理员</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="editingId">
          <el-form-item label="姓名">
            <el-input v-model="form.display_name" maxlength="50" placeholder="选填：真实姓名/称呼" />
          </el-form-item>
          <el-form-item label="电话">
            <el-input v-model="form.phone" maxlength="20" placeholder="选填" />
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="form.remark" type="textarea" :rows="2" maxlength="500" show-word-limit
              placeholder="选填：岗位、负责范围等（仅后台可见）" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import dayjs from 'dayjs'
import { buildingCreateAdmin, buildingGetUsers, buildingUpdateUser, buildingDeleteUser, adminListUsers, adminUpdateUser, adminDeleteUser } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

// 角色显示名（楼栋管理员与普通管理员区分展示）
function roleLabel(role) {
  if (role === 'super_admin') return '超级管理员'
  if (role === 'building_admin') return '楼栋管理员'
  return '普通管理员'
}

// 创建时间格式化为 YYYY-MM-DD HH:mm
function formatTime(t) {
  if (!t) return '-'
  const d = dayjs(t)
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm') : t
}

const users = ref([])
const loading = ref(false)
const showDialog = ref(false)
const submitting = ref(false)
const editingId = ref(null)
const form = ref({ username: '', password: '', confirmPassword: '', role: 'admin', display_name: '', phone: '', remark: '' })
const formRef = ref(null)
const currentUserRole = localStorage.getItem('role')
// 超管走平台接口（可跨公寓、可改角色、可看备注）；楼栋管理员走公寓接口
const isSuperAdmin = currentUserRole === 'super_admin'

// 能否编辑/删除某个账号：
//   - 超管：能管所有人（超管行本身后端也不允许删除）；
//   - 楼栋管理员：只能管本公寓的普通管理员（role=admin），
//     与自己（building_admin）和超管同级，后端也会拒绝。
function canManage(row) {
  if (!row) return false
  return isSuperAdmin || row.role === 'admin'
}

async function fetchUsers() {
  loading.value = true
  try {
    const res = isSuperAdmin ? await adminListUsers() : await buildingGetUsers()
    users.value = res.data.users
  } finally {
    loading.value = false
  }
}

function openAddDialog() {
  editingId.value = null
  form.value = { username: '', password: '', confirmPassword: '', role: 'admin', display_name: '', phone: '', remark: '' }
  showDialog.value = true
}

function openEditDialog(row) {
  editingId.value = row.id
  form.value = {
    username: row.username, password: '', confirmPassword: '', role: row.role,
    display_name: row.display_name || '', phone: row.phone || '', remark: row.remark || '',
  }
  showDialog.value = true
}

// 确认密码仅需与密码一致，不做强度/长度限制；编辑时留空密码表示不修改
function validatePasswordConfirm(rule, value, callback) {
  if (!form.value.password) return callback()
  if (value !== form.value.password) return callback(new Error('两次输入的密码不一致'))
  callback()
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) { ElMessage.warning('请先完善表单中标红的内容'); return }
  submitting.value = true
  try {
    if (editingId.value) {
      if (isSuperAdmin) {
        // 超管：平台接口，可改角色与密码
        const data = {
          role: form.value.role,
          display_name: form.value.display_name,
          phone: form.value.phone,
          remark: form.value.remark,
        }
        if (form.value.password) data.password = form.value.password
        await adminUpdateUser(editingId.value, data)
      } else {
        // 楼栋管理员：公寓接口，只能改本公寓账号的姓名/电话/备注
        // （角色与密码不在该接口的管辖范围内，改不了）
        await buildingUpdateUser(editingId.value, {
          display_name: form.value.display_name,
          phone: form.value.phone,
          remark: form.value.remark,
        })
      }
      ElMessage.success('修改成功')
    } else {
      await buildingCreateAdmin({ username: form.value.username, password: form.value.password })
      ElMessage.success('管理员创建成功')
    }
    showDialog.value = false
    await fetchUsers()
  } catch {
    ElMessage.error(editingId.value ? '修改失败' : '创建失败')
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确认删除管理员「${row.username}」？`, '提示')
    if (isSuperAdmin) {
      await adminDeleteUser(row.id)
    } else {
      // 楼栋管理员只能删本公寓的普通管理员
      await buildingDeleteUser(row.id)
    }
    ElMessage.success('删除成功')
    await fetchUsers()
  } catch (e) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('删除失败')
    }
  }
}

onMounted(() => {
  fetchUsers()
})
</script>

<style scoped>
.desktop-table {
  display: block;
}
.mobile-cards {
  display: none;
}
.user-card {
  background: #fff;
  border-radius: 10px;
  padding: 14px;
  margin-bottom: 12px;
  border: 1px solid #eee;
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.uc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.uc-name {
  font-size: 15px;
  font-weight: 600;
  color: #333;
}
.uc-field {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.uc-label {
  font-size: 12px;
  color: #999;
  min-width: 56px;
}
.uc-value {
  font-size: 13px;
  color: #666;
}
.uc-foot {
  display: flex;
  gap: 12px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid #f5f5f5;
}
.empty-text {
  text-align: center;
  padding: 32px 0;
  color: #999;
  font-size: 14px;
}
.skeleton-list { padding: 12px 0; }
.skeleton-item { padding: 14px; margin-bottom: 12px; background: #fff; border-radius: 10px; border: 1px solid #eee; }

@media (max-width: 768px) {
  .desktop-table {
    display: none;
  }
  .mobile-cards {
    display: block;
  }
  .el-card { padding: 12px; }
}
</style>
