<template>
  <div class="evolution-view">
    <!-- Tab 切换 -->
    <v-card class="mb-4" elevation="0" border>
      <v-tabs v-model="activeTab" color="primary" density="compact">
        <v-tab value="prompts">Prompt</v-tab>
        <v-tab value="skills">技能</v-tab>
        <v-tab value="defects">缺陷</v-tab>
        <v-tab value="traces">轨迹</v-tab>
        <v-tab value="shared">共享学习</v-tab>
        <v-tab value="search">搜索</v-tab>
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

    <!-- 技能管理 -->
    <div v-if="activeTab === 'skills'">
      <v-card class="mb-4 pa-3" elevation="0" border>
        <v-row align="center" dense>
          <v-select v-model="skillFilter.category" :items="['', 'general', 'system', 'evolution']" label="分类" variant="outlined" density="compact" class="mr-2" style="max-width:140px" />
          <v-select v-model="skillFilter.status" :items="['', 'active', 'draft', 'archived']" label="状态" variant="outlined" density="compact" class="mr-2" style="max-width:140px" />
          <v-spacer />
          <v-btn color="primary" prepend-icon="mdi-plus" @click="openSkillCreate">新增技能</v-btn>
        </v-row>
      </v-card>

      <v-card elevation="0" border>
        <v-list v-if="skills.length" lines="three">
          <v-list-item
            v-for="s in skills"
            :key="s.id"
            :title="s.name"
            :subtitle="`v${s.version} · ${s.category} · ${s.author}`"
          >
            <template #prepend>
              <v-chip size="small" :color="s.status === 'active' ? 'success' : s.status === 'draft' ? 'warning' : 'grey'" variant="flat">
                {{ s.status }}
              </v-chip>
            </template>
            <template #append>
              <v-btn icon size="small" variant="text" @click="openSkillEdit(s)">
                <v-icon size="18">mdi-pencil</v-icon>
              </v-btn>
              <v-btn icon size="small" variant="text" color="error" @click="confirmSkillDelete(s)">
                <v-icon size="18">mdi-delete</v-icon>
              </v-btn>
            </template>
          </v-list-item>
        </v-list>
        <v-card-text v-else class="text-center text-medium-emphasis py-8">
          暂无技能。
        </v-card-text>
      </v-card>

      <!-- Skill 编辑对话框 -->
      <v-dialog v-model="skillDialogOpen" max-width="700">
        <v-card>
          <v-card-title>{{ skillEditingId ? '编辑技能' : '新增技能' }}</v-card-title>
          <v-divider />
          <v-card-text class="pt-4">
            <v-text-field v-model="skillForm.name" label="名称" variant="outlined" density="compact" class="mb-3" />
            <v-text-field v-model="skillForm.description" label="描述" variant="outlined" density="compact" class="mb-3" />
            <v-select v-model="skillForm.category" :items="['general', 'system', 'evolution']" label="分类" variant="outlined" density="compact" class="mb-3" />
            <v-select v-model="skillForm.status" :items="['draft', 'active', 'archived']" label="状态" variant="outlined" density="compact" class="mb-3" />
            <v-textarea v-model="skillForm.content" label="内容 (SKILL.md 格式)" variant="outlined" rows="10" class="font-mono" />
          </v-card-text>
          <v-divider />
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="skillDialogOpen = false">取消</v-btn>
            <v-btn color="primary" :loading="skillSaveLoading" @click="saveSkill">保存</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>

      <!-- Skill 删除确认 -->
      <v-dialog v-model="skillDeleteDialogOpen" max-width="400">
        <v-card>
          <v-card-title>确认删除</v-card-title>
          <v-card-text>确定要删除此技能吗？</v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="skillDeleteDialogOpen = false">取消</v-btn>
            <v-btn color="error" :loading="skillDeleteLoading" @click="deleteSkillConfirm">删除</v-btn>
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
              <v-chip size="small" :color="d.patched ? 'success' : 'warning'" variant="flat" class="mr-1">
                {{ d.patched ? '已修复' : '待修复' }}
              </v-chip>
              <v-btn v-if="!d.patched" icon size="small" variant="text" color="info" @click="publishDefect(d)" :title="'发布到 OpenViking'">
                <v-icon size="18">mdi-upload</v-icon>
              </v-btn>
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

    <!-- 共享学习记录 -->
    <div v-if="activeTab === 'shared'">
      <v-card elevation="0" border>
        <v-list v-if="sharedRecords.length" lines="two">
          <v-list-item
            v-for="r in sharedRecords"
            :key="r.id"
            :title="`[${r.source_type}] #${r.source_id}`"
            :subtitle="r.target_uri"
          >
            <template #prepend>
              <v-chip size="small" :color="r.published ? 'success' : 'error'" variant="flat">
                {{ r.published ? '已发布' : '失败' }}
              </v-chip>
            </template>
            <template v-if="r.error_message" #append>
              <v-tooltip :text="r.error_message" location="left">
                <v-icon color="error" size="18">mdi-alert-circle-outline</v-icon>
              </v-tooltip>
            </template>
          </v-list-item>
        </v-list>
        <v-card-text v-else class="text-center text-medium-emphasis py-8">
          暂无共享学习记录。
        </v-card-text>
      </v-card>
    </div>

    <!-- OpenViking 搜索 -->
    <div v-if="activeTab === 'search'">
      <v-card class="mb-4 pa-3" elevation="0" border>
        <v-row align="center" dense>
          <v-text-field
            v-model="searchQuery"
            label="搜索 OpenViking 知识库"
            variant="outlined"
            density="compact"
            hide-details
            class="mr-2"
            style="flex:1"
            @keydown.enter="doSearch"
          />
          <v-btn color="primary" prepend-icon="mdi-magnify" :loading="searchLoading" @click="doSearch">搜索</v-btn>
        </v-row>
      </v-card>
      <v-card v-if="searchResult" elevation="0" border>
        <v-card-text class="font-mono" style="white-space:pre-wrap;font-size:13px">
          {{ searchResult }}
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
import api, { type EvolutionPrompt, type EvolutionDefect, type EvolutionTrace, type EvolutionSkill, type SharedLearningRecord } from '@/services/api'

const activeTab = ref('prompts')

const prompts = ref<EvolutionPrompt[]>([])
const defects = ref<EvolutionDefect[]>([])
const traces = ref<EvolutionTrace[]>([])
const skills = ref<EvolutionSkill[]>([])
const sharedRecords = ref<SharedLearningRecord[]>([])

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

// Skill form
const skillDialogOpen = ref(false)
const skillEditingId = ref<number | null>(null)
const skillDeleteDialogOpen = ref(false)
const skillDeleteTarget = ref<EvolutionSkill | null>(null)
const skillSaveLoading = ref(false)
const skillDeleteLoading = ref(false)
const skillFilter = ref({ category: '', status: '' })
const skillForm = ref({ name: '', description: '', content: '', category: 'general', status: 'draft' })

// Search
const searchQuery = ref('')
const searchResult = ref('')
const searchLoading = ref(false)

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

const loadSkills = async () => {
  try {
    const res = await api.getEvolutionSkills(skillFilter.value.category || undefined, skillFilter.value.status || undefined, 100)
    skills.value = res.skills || []
  } catch (e: unknown) { showSnack('加载技能失败: ' + (e as Error).message, 'error') }
}

const loadShared = async () => {
  try { sharedRecords.value = (await api.getSharedLearning(50)).records || [] }
  catch (e: unknown) { showSnack('加载共享学习记录失败: ' + (e as Error).message, 'error') }
}

// --- Prompt CRUD ---

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

// --- Skill CRUD ---

const openSkillCreate = () => {
  skillEditingId.value = null
  skillForm.value = { name: '', description: '', content: '', category: 'general', status: 'draft' }
  skillDialogOpen.value = true
}

const openSkillEdit = (s: EvolutionSkill) => {
  skillEditingId.value = s.id
  skillForm.value = { name: s.name, description: s.description, content: s.content, category: s.category, status: s.status }
  skillDialogOpen.value = true
}

const saveSkill = async () => {
  if (!skillForm.value.name.trim() || !skillForm.value.content.trim()) return
  skillSaveLoading.value = true
  try {
    if (skillEditingId.value) {
      await api.updateEvolutionSkill(skillEditingId.value, skillForm.value)
      showSnack('技能已更新')
    } else {
      await api.createEvolutionSkill(skillForm.value)
      showSnack('技能已创建')
    }
    skillDialogOpen.value = false
    await loadSkills()
  } catch (e: unknown) {
    showSnack('保存失败: ' + (e as Error).message, 'error')
  } finally {
    skillSaveLoading.value = false
  }
}

const confirmSkillDelete = (s: EvolutionSkill) => {
  skillDeleteTarget.value = s
  skillDeleteDialogOpen.value = true
}

const deleteSkillConfirm = async () => {
  if (!skillDeleteTarget.value) return
  skillDeleteLoading.value = true
  try {
    await api.deleteEvolutionSkill(skillDeleteTarget.value.id)
    showSnack('技能已删除')
    skillDeleteDialogOpen.value = false
    await loadSkills()
  } catch (e: unknown) {
    showSnack('删除失败: ' + (e as Error).message, 'error')
  } finally {
    skillDeleteLoading.value = false
  }
}

// --- Actions ---

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

const publishDefect = async (d: EvolutionDefect) => {
  try {
    const res = await api.publishDefect(d.id)
    if (res.published) {
      showSnack(`缺陷 #${d.id} 已发布到 OpenViking: ${res.target_uri}`)
    } else {
      showSnack(`发布失败: ${res.error_message || '未知错误'}`, 'error')
    }
  } catch (e: unknown) {
    showSnack('发布失败: ' + (e as Error).message, 'error')
  }
}

const doSearch = async () => {
  if (!searchQuery.value.trim()) return
  searchLoading.value = true
  try {
    const res = await api.searchOpenViking(searchQuery.value)
    searchResult.value = res.results || res.result || '(无结果)'
  } catch (e: unknown) {
    showSnack('搜索失败: ' + (e as Error).message, 'error')
  } finally {
    searchLoading.value = false
  }
}

watch(activeTab, (val) => {
  if (val === 'prompts') loadPrompts()
  else if (val === 'skills') loadSkills()
  else if (val === 'defects') loadDefects()
  else if (val === 'traces') loadTraces()
  else if (val === 'shared') loadShared()
})

watch(skillFilter, () => { if (activeTab.value === 'skills') loadSkills() }, { deep: true })

onMounted(loadPrompts)
</script>

<style scoped>
.font-mono :deep(textarea) {
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
}
</style>
