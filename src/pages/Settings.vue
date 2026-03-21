<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { AlertCircle, CheckCircle2 } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import { listManagers } from '@/lib/api'
import { enabledCapabilities } from '@/lib/format'
import type { ManagerInfo } from '@/model/types'

const loading = ref(false)
const managers = ref<ManagerInfo[]>([])
const errorMessage = ref('')
const infoMessage = ref('')

const enabledCount = computed(() => managers.value.filter(manager => manager.enabled).length)

async function loadManagers() {
  loading.value = true
  errorMessage.value = ''

  try {
    managers.value = await listManagers()
  } catch (error) {
    errorMessage.value = `Failed to load package managers: ${String(error)}`
  } finally {
    loading.value = false
  }
}

function saveSettings() {
  infoMessage.value =
    'Manager enable/disable is not persisted yet. This page now shows live backend state.'
}

onMounted(() => {
  void loadManagers()
})
</script>

<template>
  <div class="flex-1 flex flex-col h-full">
    <header class="theme-header h-14 flex items-center px-6 border-b shrink-0">
      <h1 class="theme-text text-lg font-semibold">Settings</h1>
    </header>
    <div class="theme-app p-6 flex-1 overflow-auto">
      <div class="max-w-3xl">
        <div class="mb-8">
          <h2 class="theme-text text-base font-semibold mb-1">Appearance</h2>
          <p class="theme-text-muted text-sm mb-4">
            The current frontend theme comes from your template and local theme composable.
          </p>

          <div class="theme-panel theme-border p-4 rounded-lg border shadow-sm">
            <div class="flex items-center justify-between">
              <div>
                <label class="theme-text font-medium text-sm">Theme</label>
                <p class="theme-text-muted text-sm">
                  Theme switching is still controlled by the template side and not by backend
                  settings.
                </p>
              </div>
            </div>
          </div>
        </div>

        <div class="mb-8">
          <h2 class="theme-text text-base font-semibold mb-1">Package Managers</h2>
          <p class="theme-text-muted text-sm mb-4">
            Live status from the Rust registry. {{ enabledCount }}/{{ managers.length }} managers
            are currently enabled.
          </p>

          <div
            v-if="errorMessage"
            class="theme-alert-danger mb-4 flex items-center rounded-md border px-4 py-3 text-sm"
          >
            <AlertCircle class="mr-2 h-4 w-4 shrink-0" />
            {{ errorMessage }}
          </div>

          <div
            v-if="infoMessage"
            class="theme-alert-success mb-4 flex items-center rounded-md border px-4 py-3 text-sm"
          >
            <CheckCircle2 class="mr-2 h-4 w-4 shrink-0" />
            {{ infoMessage }}
          </div>

          <div class="space-y-4">
            <div
              v-for="manager in managers"
              :key="manager.id"
              class="theme-panel theme-border flex items-start justify-between space-x-4 p-4 rounded-lg border shadow-sm"
            >
              <div class="flex-1">
                <div class="flex items-center gap-3">
                  <label class="theme-text font-medium text-sm">
                    {{ manager.name }}
                  </label>
                  <span
                    class="inline-flex items-center rounded-full border px-2 py-0.5 text-xs"
                    :class="
                      manager.enabled
                        ? 'theme-alert-success'
                        : 'theme-panel-subtle theme-border theme-text-secondary'
                    "
                  >
                    {{ manager.enabled ? 'Enabled' : 'Disabled' }}
                  </span>
                </div>
                <p class="theme-text-muted mt-1 text-sm">id: {{ manager.id }}</p>
                <p class="theme-text-muted mt-2 text-sm">
                  capabilities: {{ enabledCapabilities(manager.capabilities).join(', ') || 'none' }}
                </p>
              </div>
            </div>
            <div
              v-if="!loading && managers.length === 0"
              class="theme-text-muted theme-border p-6 rounded-lg border border-dashed text-sm"
            >
              No package managers are currently registered.
            </div>
          </div>
        </div>

        <div class="theme-border pt-4 border-t flex justify-end">
          <div class="flex gap-2">
            <Button variant="outline" :disabled="loading" @click="loadManagers">Refresh</Button>
            <Button @click="saveSettings">Save Changes</Button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
