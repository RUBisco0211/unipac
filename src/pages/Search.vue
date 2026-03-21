<script setup lang="ts">
import { ref } from 'vue'
import { Search as SearchIcon, Download, AlertCircle } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Badge from '@/components/ui/Badge.vue'
import { installPackage, searchPackages } from '@/lib/api'
import { managerLabelMap } from '@/lib/format'
import type { Package } from '@/model/types'

const searchQuery = ref('')
const isSearching = ref(false)
const hasSearched = ref(false)
const installingKey = ref('')
const errorMessage = ref('')
const feedbackMessage = ref('')
const searchResults = ref<Package[]>([])

async function handleSearch() {
  if (!searchQuery.value.trim()) return

  isSearching.value = true
  hasSearched.value = true
  errorMessage.value = ''
  feedbackMessage.value = ''

  try {
    searchResults.value = await searchPackages(searchQuery.value.trim())
  } catch (error) {
    errorMessage.value = `Search failed: ${String(error)}`
    searchResults.value = []
  } finally {
    isSearching.value = false
  }
}

async function handleInstall(pkg: Package) {
  installingKey.value = `${pkg.manager}:${pkg.name}`
  errorMessage.value = ''
  feedbackMessage.value = ''

  try {
    const result = await installPackage(pkg.manager, pkg.name)
    if (!result.success) {
      throw new Error(result.message)
    }
    feedbackMessage.value = result.message
  } catch (error) {
    errorMessage.value = `Install failed for ${pkg.name}: ${String(error)}`
  } finally {
    installingKey.value = ''
  }
}
</script>

<template>
  <div class="flex-1 flex flex-col h-full">
    <header class="theme-header h-14 flex items-center px-6 border-b shrink-0">
      <h1 class="theme-text text-lg font-semibold">Search Packages</h1>
    </header>
    <div class="theme-app p-6 flex-1 overflow-auto">
      <div class="max-w-2xl mx-auto mb-8">
        <form @submit.prevent="handleSearch" class="flex space-x-2">
          <div class="relative flex-1">
            <SearchIcon class="theme-text-muted absolute left-3 top-3 h-4 w-4" />
            <Input
              v-model="searchQuery"
              placeholder="Search across all package managers..."
              class="pl-10 h-10"
            />
          </div>
          <Button type="submit" class="h-10" :disabled="isSearching">
            {{ isSearching ? 'Searching...' : 'Search' }}
          </Button>
        </form>
      </div>

      <div
        v-if="errorMessage"
        class="theme-alert-danger mb-4 flex items-center rounded-md border px-4 py-3 text-sm"
      >
        <AlertCircle class="mr-2 h-4 w-4 shrink-0" />
        {{ errorMessage }}
      </div>

      <div
        v-if="feedbackMessage"
        class="theme-alert-success mb-4 rounded-md border px-4 py-3 text-sm"
      >
        {{ feedbackMessage }}
      </div>

      <div
        v-if="hasSearched && !isSearching"
        class="theme-panel theme-border rounded-md border shadow-sm"
      >
        <table class="w-full text-sm text-left">
          <thead class="theme-text-muted theme-panel-subtle theme-border border-b text-xs">
            <tr>
              <th class="h-10 px-4 font-medium align-middle">Name</th>
              <th class="h-10 px-4 font-medium align-middle">Description</th>
              <th class="h-10 px-4 font-medium align-middle">Version</th>
              <th class="h-10 px-4 font-medium align-middle">Manager</th>
              <th class="h-10 px-4 font-medium align-middle text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="pkg in searchResults"
              :key="`${pkg.manager}-${pkg.name}`"
              class="theme-border theme-panel-hover border-b last:border-0 transition-colors"
            >
              <td class="theme-text p-4 align-middle font-medium">{{ pkg.name }}</td>
              <td
                class="theme-text-secondary p-4 align-middle truncate max-w-xs"
                :title="pkg.description"
              >
                {{ pkg.description }}
              </td>
              <td class="theme-text-secondary p-4 align-middle">
                {{ pkg.latest_version || pkg.version }}
              </td>
              <td class="p-4 align-middle">
                <Badge variant="outline" :class="`manager-badge-${pkg.manager}`">
                  {{ managerLabelMap[pkg.manager] }}
                </Badge>
              </td>
              <td class="p-4 align-middle text-right">
                <Button
                  size="sm"
                  class="h-8"
                  :disabled="installingKey !== ''"
                  @click="handleInstall(pkg)"
                >
                  <Download class="w-3.5 h-3.5 mr-1.5" />
                  {{ installingKey === `${pkg.manager}:${pkg.name}` ? 'Installing...' : 'Install' }}
                </Button>
              </td>
            </tr>
            <tr v-if="searchResults.length === 0">
              <td colspan="5" class="theme-text-muted p-8 text-center">
                No packages found matching "{{ searchQuery }}".
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="!hasSearched"
        class="theme-text-muted flex flex-col items-center justify-center h-64"
      >
        <SearchIcon class="w-12 h-12 mb-4 text-[var(--color-border-strong)]" />
        <p>Search for packages to install from Homebrew, npm, pip, and more.</p>
      </div>
    </div>
  </div>
</template>
