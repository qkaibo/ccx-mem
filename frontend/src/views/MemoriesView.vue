<template>
  <div class="memories-view">
    <!-- 顶部操作栏 -->
    <v-card class="mb-4 pa-3" elevation="0" border>
      <v-row align="center" dense>
        <v-col cols="12" sm="4">
          <v-text-field
            v-model="searchQuery"
            label="搜索记忆"
            prepend-inner-icon="mdi-magnify"
            variant="outlined"
            density="compact"
            hide-details
            clearable
            @keyup.enter="searchMemories"
            @click:clear="loadMemories"
          />
        </v-col>
        <v-col cols="12" sm="3">
          <v-select
            v-model="layerFilter"
            :items="['', 'indexed', 'core']"
            label="层级"
            variant="outlined"
            density="compact"
            hide-details
            @update:model-value="searchMemories"
          />
        </v-col>
        <v-col cols="12" sm="5" class="d-flex gap-2 justify-end">
          <v-btn color="primary" prepend-icon="mdi-plus" @click="openCreate">新增</v-btn>
          <v-btn color="warning" variant="tonal" prepend-icon="mdi-auto-fix" :loading="dreamLoading" @click="triggerDream">Dream</v-btn>
          <v-btn variant="tonal" prepend-icon="mdi-refresh" @click="loadMemories">刷新</v-btn>
        </v-col>
      </v-row>
    </v-card>

    <!-- 记忆列表 -->
    <v-card elevation="0" border>
      <v-list v-if="memories.length" lines="two">
        <v-list-item
          v-for="mem in memories"
          :key="mem.id"
          :title="mem.content.length > 80 ? mem.content.slice(0, 80) + '…' : mem.content"
          :subtitle="`${mem.layer} · ${mem.source} · ${formatDate(mem.updated_at)}`"
        >
          <template #prepend>
            <v-chip size="small" :color="mem.layer === 'core' ? 'error' : 'info'" variant="flat">
              {{ mem.layer }}
            </v-chip>
          </template>
          <template #append>
            <v-btn icon size="small" variant="text" @click="openEdit(mem)">
              <v-icon size="18">mdi-pencil</v-icon>
            </v-btn>
            <v-btn icon size="small" variant="text" color="error" @click="confirmDelete(mem)">
              <v-icon size="18">mdi-delete</v-icon>
            </v-btn>
          </template>
        </v-list-item>
      </v-list>
      <v-card-text v-else class="text-center text-medium-emphasis py-8">
        暂无记忆，点击「新增」创建或「Dream」自动提取。
      </v-card-text>
    </v-card>

    <!-- 新建/编辑对话框 -->
    <v-dialog v-model="dialogOpen" max-width="600">
      <v-card>
        <v-card-title>{{ editingId ? '编辑记忆' : '新增记忆' }}</v-card-title>
        <v-divider />
        <v-card-text class="pt-4">
          <v-textarea
            v-model="form.content"
            label="内容"
            variant="outlined"
            rows="5"
            required
          />
          <v-row dense class="mt-2">
            <v-col cols="6">
              <v-select v-model="form.layer" :items="['indexed', 'core']" label="层级" variant="outlined" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.tags" label="标签 (逗号分隔)" variant="outlined" density="compact" />
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider />
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialogOpen = false">取消</v-btn>
          <v-btn color="primary" :loading="saveLoading" @click="saveMemory">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 删除确认 -->
    <v-dialog v-model="deleteDialogOpen" max-width="400">
      <v-card>
        <v-card-title>确认删除</v-card-title>
        <v-card-text>确定要删除这条记忆吗？此操作不可撤销。</v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="deleteDialogOpen = false">取消</v-btn>
          <v-btn color="error" :loading="deleteLoading" @click="deleteMemory">删除</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 提示条 -->
    <v-snackbar v-model="snackbar" :color="snackbarColor" timeout="3000">
      {{ snackbarText }}
    </v-snackbar>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api, { type MemoryRecord } from '@/services/api'

const memories = ref<MemoryRecord[]>([])
const searchQuery = ref('')
const layerFilter = ref('')
const dialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const editingId = ref<number | null>(null)
const deleteTarget = ref<MemoryRecord | null>(null)
const saveLoading = ref(false)
const deleteLoading = ref(false)
const dreamLoading = ref(false)
const snackbar = ref(false)
const snackbarText = ref('')
const snackbarColor = ref('success')

const form = ref({ content: '', layer: 'indexed', tags: '' })

const showSnack = (text: string, color = 'success') => {
  snackbarText.value = text
  snackbarColor.value = color
  snackbar.value = true
}

const formatDate = (d: string) => new Date(d).toLocaleString()

const loadMemories = async () => {
  try {
    const res = await api.getMemories(searchQuery.value, layerFilter.value || undefined)
    memories.value = res.memories || []
  } catch (e: unknown) {
    showSnack('加载失败: ' + (e as Error).message, 'error')
  }
}

const searchMemories = () => loadMemories()

const openCreate = () => {
  editingId.value = null
  form.value = { content: '', layer: 'indexed', tags: '' }
  dialogOpen.value = true
}

const openEdit = (mem: MemoryRecord) => {
  editingId.value = mem.id
  form.value = { content: mem.content, layer: mem.layer, tags: mem.tags }
  dialogOpen.value = true
}

const saveMemory = async () => {
  if (!form.value.content.trim()) return
  saveLoading.value = true
  try {
    if (editingId.value) {
      await api.updateMemory(editingId.value, form.value)
      showSnack('记忆已更新')
    } else {
      await api.createMemory(form.value)
      showSnack('记忆已创建')
    }
    dialogOpen.value = false
    await loadMemories()
  } catch (e: unknown) {
    showSnack('保存失败: ' + (e as Error).message, 'error')
  } finally {
    saveLoading.value = false
  }
}

const confirmDelete = (mem: MemoryRecord) => {
  deleteTarget.value = mem
  deleteDialogOpen.value = true
}

const deleteMemory = async () => {
  if (!deleteTarget.value) return
  deleteLoading.value = true
  try {
    await api.deleteMemory(deleteTarget.value.id)
    showSnack('记忆已删除')
    deleteDialogOpen.value = false
    await loadMemories()
  } catch (e: unknown) {
    showSnack('删除失败: ' + (e as Error).message, 'error')
  } finally {
    deleteLoading.value = false
  }
}

const triggerDream = async () => {
  dreamLoading.value = true
  try {
    const res = await api.triggerDream()
    showSnack(`Dream 完成: 提取 ${res.extracted} 条记忆`)
    await loadMemories()
  } catch (e: unknown) {
    showSnack('Dream 失败: ' + (e as Error).message, 'error')
  } finally {
    dreamLoading.value = false
  }
}

onMounted(loadMemories)
</script>

<style scoped>
.gap-2 { gap: 8px; }
</style>
