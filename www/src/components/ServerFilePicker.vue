<template>
  <FormModal
    v-model="visible"
    :title="mode === 'directory' ? t('filePicker.titleFolder') : t('filePicker.titleFile')"
    :width="480"
  >
    <div class="file-picker-body">
      <div class="file-picker-path-row">
        <n-button quaternary circle size="small" :disabled="!parent" @click="goUp">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 19V5M5 12l7-7 7 7" />
          </svg>
        </n-button>
        <n-input
          v-model:value="pathInput"
          size="small"
          :placeholder="t('filePicker.pathPlaceholder')"
          @keydown.enter="load(pathInput)"
        />
        <n-button size="small" :loading="loading" @click="load(pathInput)">{{ t('filePicker.go') }}</n-button>
      </div>

      <n-scrollbar class="file-picker-list" style="max-height: 320px;">
        <div v-if="!loading && entries.length === 0" class="file-picker-empty">
          {{ t('filePicker.empty') }}
        </div>
        <div
          v-for="entry in visibleEntries"
          :key="entry.path"
          class="file-picker-row"
          :class="{ 'is-selected': entry.path === selectedPath }"
          @click="handleEntryClick(entry)"
        >
          <svg v-if="entry.isDir" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z" />
          </svg>
          <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z" />
            <polyline points="13 2 13 9 20 9" />
          </svg>
          <span>{{ entry.name }}</span>
        </div>
      </n-scrollbar>
    </div>

    <template #footer>
      <n-space justify="end">
        <n-button @click="visible = false">{{ t('common.cancel') }}</n-button>
        <n-button
          v-if="mode === 'directory'"
          type="primary"
          :disabled="!currentPath"
          @click="confirmSelection(currentPath)"
        >
          {{ t('filePicker.selectFolder') }}
        </n-button>
        <n-button
          v-else
          type="primary"
          :disabled="!selectedPath"
          @click="confirmSelection(selectedPath)"
        >
          {{ t('filePicker.selectFile') }}
        </n-button>
      </n-space>
    </template>
  </FormModal>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '@/utils/api'
import { useNotify } from '@/utils/notify'
import FormModal from '@/components/FormModal.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  // 'file' lets the user pick a file; 'directory' lets them pick a folder
  mode: { type: String, default: 'file' },
  initialPath: { type: String, default: '' }
})

const emit = defineEmits(['update:modelValue', 'select'])

const { t } = useI18n()
const notify = useNotify()

const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const currentPath = ref('')
const parent = ref('')
const entries = ref([])
const pathInput = ref('')
const selectedPath = ref('')
const loading = ref(false)

const visibleEntries = computed(() => {
  if (props.mode === 'directory') {
    return entries.value.filter((entry) => entry.isDir)
  }
  return entries.value
})

async function load(path) {
  loading.value = true
  try {
    const listing = await api.listDir(path || '')
    currentPath.value = listing.path
    parent.value = listing.parent || ''
    entries.value = listing.entries || []
    pathInput.value = listing.path
    selectedPath.value = ''
  } catch (error) {
    notify.error(error.message)
  } finally {
    loading.value = false
  }
}

function goUp() {
  if (parent.value) load(parent.value)
}

function handleEntryClick(entry) {
  if (entry.isDir) {
    load(entry.path)
    return
  }
  if (props.mode === 'file') {
    selectedPath.value = entry.path
  }
}

function confirmSelection(path) {
  if (!path) return
  emit('select', path)
  visible.value = false
}

watch(visible, (show) => {
  if (show) load(props.initialPath)
})
</script>

<style scoped>
.file-picker-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.file-picker-path-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.file-picker-path-row .n-input {
  flex: 1;
}

.file-picker-list {
  border: 1px solid var(--ref-color-white-8);
  border-radius: 6px;
}

.file-picker-empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--sys-color-text-secondary);
  font-size: var(--ref-font-size-sm);
}

.file-picker-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  cursor: pointer;
  font-size: var(--ref-font-size-sm);
}

.file-picker-row:hover {
  background: var(--ref-color-white-4);
}

.file-picker-row.is-selected {
  background: var(--sys-color-bg-brand);
  color: #fff;
}
</style>
