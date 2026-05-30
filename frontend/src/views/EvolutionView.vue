<template>
  <div class="evolution-view">
    <!-- Tab 切换 -->
    <v-card class="mb-4" elevation="0" border>
      <v-tabs v-model="activeTab" color="primary" density="compact">
        <v-tab value="prompts">Prompt</v-tab>
        <v-tab value="defects">缺陷</v-tab>
        <v-tab value="traces">轨迹</v-tab>
      </v-tabs>
    </v-card>

    <!-- Prompt 管理 -->
    <div v-if="activeTab === 'prompts'">
      <v-card class="mb-4 pa-3" elevation="0" border>
        <v-row align="center" dense>
          <v-spacer />
          <v-btn color="primary" prepend-icon="mdi-plus" @click="openPromptCreate">新增 Prompt</v-btn>
          <v-btn color="warning" variant="tonal" prepend-icon="mdi-shield-check" class="ml-2" :loading="auditLoading" @click="triggerAudit">审计</v-btn>
          <v-btn color="info" variant="tonal" prepend-icon="mdi-chart-bell-curve" class="ml-2" :loading="analyzeLoading" @click="triggerAnalyze">分析</v-btn>
        </v-row>
      </v-card>

      <v-card elevation="0" border>
        <v-list v-if="prompts.length" lines="two">
          <v-list-item
            v-for="p in prompts"
            :key="p.id"
            :title="p.name"
            :subtitle="`v${p.version} · ${p.category} · ${p.status} · ${formatDate(p.updated_at)}`"
          >
            <template #prepend>
              <v-chip size="small" :color="p.status === 'active' ? 'success' : 'warning'" variant="flat">
                {{ p.status }}
              </v-chip>
            </template>
            <template #append>
              <v-btn icon size="small" variant="text" @click="openPromptEdit(p)">
                <v-icon size="18">mdi-pencil</v-icon>
              </v-btn>
              <v-btn icon size="small" variant="text" color="error" @click="confirmPromptDelete(p)">
                <v-icon size="18">mdi-delete</v-icon>
              </v-btn>
            </template>
          </v-list-item>
        </v-list>
        <v-card-text v-else class="text-center text-medium-emphasis py-8">
          暂无 Prompt，点击「新增 Prompt」创建。
        </v-card-text>
      </v-card>

      <!-- Prompt 编辑对话框 -->
      <v-dialog v-model="promptDialogOpen" max-width="700">
        <v-card>
          <v-card-title>{{ promptEditingId ? '编辑 Prompt' : '新增 Prompt' }}</v-card-title>
          <v-divider />
          <v-card-text class="pt-4">
            <v-text-field v-model="promptForm.name" label="名称" variant="outlined" density="compact" class="mb-3" />
            <v-select v-model="promptForm.category" :items="['system', 'skill', 'other']" label="类别" variant="outlined" density="compact" class="mb-3" />
            <v-textarea v-model="promptForm.content" label="内容" variant="outlined" rows="8" class="font-mono" />
          </v-card-text>
          <v-divider />
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="promptDialogOpen = false">取消</v-btn>
            <v-btn color="primary" :loading="promptSaveLoading" @click="savePrompt">保存</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>

      <!-- Prompt 删除确认 -->
      <v-dialog v-model="promptDeleteDialogOpen" max-width="400">
        <v-card>
          <v-card-title>确认删除</v-card-title>
          <v-card-text>确定要删除此 Prompt 吗？</v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="promptDeleteDialogOpen = false">取消</v-btn>
            <v-btn color="error" :loading="promptDeleteLoading" @click="deletePromptConfirm">删除</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>
    </div>

    <!-- 缺陷列表 -->
    <div v-if="activeTab === 'defects'">
      <v-card elevation="0" border>
        <v-list v-if="defects.length" lines="three">
          <v-list-item
            v-for="d in defects"
            :key="d.id"
            :title="`[${d.severity}] ${d.rule_id}`"
            :subtitle="d.description"
          >
            <template #prepend>
              <v-icon :color="d.severity === 'critical' ? 'error' : d.severity === 'high' ? 'warning' : 'info'">
                {{ d.severity === 'critical' ? 'mdi-alert-circle' : d.severity === 'high' ? 'mdi-alert' : 'mdi-information' }}
              </v-icon>
            </template>
            <template #append>
              <v-chip size="small" :color="d.patched ? 'success' : 'warning'" variant="flat">
                {{ d.patched ? '已修复' : '待修复' }}
              </v-chip>
            </template>
          </v-list-item>
        </v-list>
        <v-card-text v-else class="text-center text-medium-emphasis py-8">
          暂无缺陷，点击「分析」或「审计」触发检测。
        </v-card-text>
      </v-card>
    </div>

    <!-- 执行轨迹 -->
    <div v-if="activeTab === 'traces'">
      <v-card elevation="0" border>
        <v-list v-if="traces.length" lines="two">
          <v-list-item
            v-for="t in traces"
            :key="t.id"
            :title="`Prompt #${t.prompt_id} · User ${t.user_id}`"
            :subtitle="`${t.request_summary} · ${formatDate(t.created_at)}`"
          >
            <template #prepend>
              <v-chip size="small" :color="t.success ? 'success' : 'error'" variant="flat">
                {{ t.success ? '成功' : '失败' }}
              </v-chip>
            </template>
          </v-list-item>
        </v-list>
        <v-card-text v-else class="text-center text-medium-emphasis py-8">
          暂无执行轨迹。
        </v-card-text>
      </v-card>
    </div>

    <v-snackbar v-model="snackbar" :color="snackbarColor" timeout="3000">
      {{ snackbarText }}
    </v-snackbar>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import api, { type EvolutionPrompt, type EvolutionDefect, type EvolutionTrace } from '@/services/api'

const activeTab = ref('prompts')

const prompts = ref<EvolutionPrompt[]>([])
const defects = ref<EvolutionDefect[]>([])
const traces = ref<EvolutionTrace[]>([])

const snackbar = ref(false)
const snackbarText = ref('')
const snackbarColor = ref('success')
const auditLoading = ref(false)
const analyzeLoading = ref(false)

// Prompt form
const promptDialogOpen = ref(false)
const promptEditingId = ref<number | null>(null)
const promptDeleteDialogOpen = ref(false)
const promptDeleteTarget = ref<EvolutionPrompt | null>(null)
const promptSaveLoading = ref(false)
const promptDeleteLoading = ref(false)
const promptForm = ref({ name: '', content: '', category: 'system' })

const showSnack = (text: string, color = 'success') => {
  snackbarText.value = text
  snackbarColor.value = color
  snackbar.value = true
}

const formatDate = (d: string) => new Date(d).toLocaleString()

const loadPrompts = async () => {
  try { prompts.value = (await api.getEvolutionPrompts(100)).prompts || [] }
  catch (e: unknown) { showSnack('加载 Prompt 失败: ' + (e as Error).message, 'error') }
}

const loadDefects = async () => {
  try { defects.value = (await api.getEvolutionDefects()).defects || [] }
  catch (e: unknown) { showSnack('加载缺陷失败: ' + (e as Error).message, 'error') }
}

const loadTraces = async () => {
  try { traces.value = (await api.getEvolutionTraces(100)).traces || [] }
  catch (e: unknown) { showSnack('加载轨迹失败: ' + (e as Error).message, 'error') }
}

const openPromptCreate = () => {
  promptEditingId.value = null
  promptForm.value = { name: '', content: '', category: 'system' }
  promptDialogOpen.value = true
}

const openPromptEdit = (p: EvolutionPrompt) => {
  promptEditingId.value = p.id
  promptForm.value = { name: p.name, content: p.content, category: p.category }
  promptDialogOpen.value = true
}

const savePrompt = async () => {
  if (!promptForm.value.name.trim() || !promptForm.value.content.trim()) return
  promptSaveLoading.value = true
  try {
    if (promptEditingId.value) {
      await api.updateEvolutionPrompt(promptEditingId.value, promptForm.value)
      showSnack('Prompt 已更新')
    } else {
      await api.createEvolutionPrompt(promptForm.value)
      showSnack('Prompt 已创建')
    }
    promptDialogOpen.value = false
    await loadPrompts()
  } catch (e: unknown) {
    showSnack('保存失败: ' + (e as Error).message, 'error')
  } finally {
    promptSaveLoading.value = false
  }
}

const confirmPromptDelete = (p: EvolutionPrompt) => {
  promptDeleteTarget.value = p
  promptDeleteDialogOpen.value = true
}

const deletePromptConfirm = async () => {
  if (!promptDeleteTarget.value) return
  promptDeleteLoading.value = true
  try {
    await api.deleteEvolutionPrompt(promptDeleteTarget.value.id)
    showSnack('Prompt 已删除')
    promptDeleteDialogOpen.value = false
    await loadPrompts()
  } catch (e: unknown) {
    showSnack('删除失败: ' + (e as Error).message, 'error')
  } finally {
    promptDeleteLoading.value = false
  }
}

const triggerAudit = async () => {
  auditLoading.value = true
  try {
    const res = await api.triggerEvolutionAudit()
    showSnack(`审计完成: ${res.defects_found} 个缺陷 · ${res.passed}/${res.total_rules} 通过`)
    await loadDefects()
  } catch (e: unknown) {
    showSnack('审计失败: ' + (e as Error).message, 'error')
  } finally {
    auditLoading.value = false
  }
}

const triggerAnalyze = async () => {
  analyzeLoading.value = true
  try {
    const res = await api.triggerEvolutionAnalyze()
    showSnack(`分析完成: ${res.traces_analyzed} 条轨迹 · ${res.defects_found} 个缺陷`)
    await loadDefects()
  } catch (e: unknown) {
    showSnack('分析失败: ' + (e as Error).message, 'error')
  } finally {
    analyzeLoading.value = false
  }
}

watch(activeTab, (val) => {
  if (val === 'prompts') loadPrompts()
  else if (val === 'defects') loadDefects()
  else if (val === 'traces') loadTraces()
})

onMounted(loadPrompts)
</script>

<style scoped>
.font-mono :deep(textarea) {
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
}
</style>
