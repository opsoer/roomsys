<template>
  <div>
    <el-card shadow="never" style="margin-bottom: 16px; background: #fafafa;">
      <div style="display: flex; gap: 12px; flex-wrap: wrap; align-items: center;">
        <el-select v-model="filterStatus" placeholder="公寓状态" clearable style="width:130px" @change="$emit('search')">
          <el-option label="正常" value="normal" />
          <el-option label="即将到期" value="expiring" />
          <el-option label="已到期" value="expired" />
        </el-select>
        <el-input v-model="keyword" placeholder="搜索公寓名或房东电话" clearable style="width:260px" @keyup.enter="$emit('search')" @clear="$emit('search')">
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-button @click="$emit('search')">搜索</el-button>
      </div>
    </el-card>

    <div v-loading="loading">
      <div v-if="buildings.length === 0 && !loading" style="text-align:center;padding:60px 0;color:#999;">
        暂无公寓数据
      </div>
      <div v-if="loading" class="skeleton-list">
        <div v-for="n in 3" :key="n" class="skeleton-item">
          <el-skeleton :rows="2" animated>
            <template #template>
              <div style="display:flex;gap:12px;align-items:flex-start;">
                <el-skeleton-item variant="rect" style="width:60px;height:60px;border-radius:8px;" />
                <div style="flex:1">
                  <el-skeleton-item variant="h3" style="width:40%;margin-bottom:8px;" />
                  <el-skeleton-item variant="text" style="width:80%;margin-bottom:6px;" />
                  <el-skeleton-item variant="text" style="width:60%;" />
                </div>
              </div>
            </template>
          </el-skeleton>
        </div>
      </div>
      <el-card v-for="b in buildings" :key="b.id" shadow="hover" style="margin-bottom: 12px;">
        <div style="display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-start;">
          <div v-if="b.cover_image" style="width:120px;height:90px;border-radius:8px;overflow:hidden;flex-shrink:0;background:#f0f0f0;">
            <img :src="mediaUrl(b.cover_image)" :alt="b.name" style="width:100%;height:100%;object-fit:cover;" />
          </div>
          <div style="flex: 1; min-width: 240px;">
            <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 8px;">
              <span style="font-size: 18px; font-weight: 600;">{{ b.name }}</span>
              <el-tag v-if="b.building_status === 'normal'" size="small" type="success">正常</el-tag>
              <el-tag v-else-if="b.building_status === 'expiring'" size="small" type="warning">即将到期</el-tag>
              <el-tag v-else size="small" type="danger">已到期</el-tag>
              <el-tag :type="b.package === 'full' ? 'primary' : 'info'" size="small" effect="plain">{{ b.package === 'full' ? '全套餐' : '基础套餐' }}</el-tag>
            </div>
            <div style="font-size: 13px; color: #555; line-height: 1.8;">
              <div v-if="b.district">
                <span style="color:#999;">📍</span>
                {{ b.district }} {{ b.street }} {{ b.village }} {{ b.building_no }}
              </div>
              <div v-if="b.landlords?.length">
                <span style="color:#999;">👤</span>
                <template v-for="(l, i) in b.landlords" :key="l.id">
                  {{ l.name }} {{ l.phone }}<span v-if="i < b.landlords.length - 1"> / </span>
                </template>
              </div>
              <div>
                <span style="color:#999;">🚪</span>
                房间 {{ b.room_count }} 间
                <el-tag v-if="b.vacant_count > 0" size="small" type="success" style="margin-left:6px;">可租 {{ b.vacant_count }}</el-tag>
                <span v-else style="margin-left:6px;color:#999;">已满</span>
              </div>
              <div v-if="b.contract_date">
                <span style="color:#999;">📅</span>
                签约 {{ b.contract_date }} → {{ b.expired_at || '未设置' }}
              </div>
            </div>
          </div>
          <div style="display: flex; flex-wrap: wrap; gap: 6px; align-items: flex-start;">
            <el-button size="small" @click="$emit('edit', b)">编辑</el-button>
            <el-button size="small" @click="$emit('upgrade', b)">升级套餐</el-button>
            <el-button size="small" @click="$emit('copy-link', b)">复制登录链接</el-button>
            <el-button size="small" @click="$emit('create-admin', b)">创建管理员</el-button>
            <el-popconfirm title="确定删除?" @confirm="$emit('delete', b.id)">
              <template #reference>
                <el-button size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { mediaUrl } from '../../utils/format'

defineProps({
  buildings: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
})

defineEmits(['search', 'edit', 'upgrade', 'copy-link', 'create-admin', 'delete'])

const filterStatus = ref('')
const keyword = ref('')

function getFilter() {
  return { status: filterStatus.value, keyword: keyword.value }
}

defineExpose({ getFilter })
</script>

<style scoped>
.skeleton-list { padding: 12px 0; }
.skeleton-item { padding: 12px; margin-bottom: 12px; background: #fff; border-radius: 8px; border: 1px solid #eee; }
</style>
