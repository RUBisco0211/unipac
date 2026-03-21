<script setup lang="ts">
import ManagerPill from '@/components/common/ManagerPill.vue'
import type { Package } from '@/model/types'

defineProps<{
  packages: Package[]
  busy?: boolean
  allowInstall?: boolean
}>()

const emit = defineEmits<{
  refresh: []
  install: [pkg: Package]
  uninstall: [pkg: Package]
  upgrade: [pkg: Package]
}>()
</script>

<template>
  <Card>
    <template #content>
      <div class="table-toolbar">
        <Button
          label="刷新"
          icon="pi pi-refresh"
          severity="secondary"
          :loading="busy"
          @click="emit('refresh')"
        />
      </div>

      <DataTable :value="packages" :loading="busy" size="small" stripedRows>
        <Column field="name" header="名称" sortable>
          <template #body="{ data }">
            <div class="name-cell">
              <strong>{{ data.name }}</strong>
              <small>{{ data.description || data.fullname || '暂无描述' }}</small>
            </div>
          </template>
        </Column>
        <Column header="管理器">
          <template #body="{ data }">
            <ManagerPill :manager="data.manager" />
          </template>
        </Column>
        <Column field="version" header="当前版本" />
        <Column field="latest_version" header="最新版本" />
        <Column header="类型">
          <template #body="{ data }">{{ data.is_gui ? 'GUI' : 'CLI' }}</template>
        </Column>
        <Column header="状态">
          <template #body="{ data }">
            <Tag
              :severity="data.outdated ? 'warning' : 'success'"
              :value="data.outdated ? '可更新' : '已是最新'"
              rounded
            />
          </template>
        </Column>
        <Column header="操作" style="width: 140px">
          <template #body="{ data }">
            <div class="actions">
              <template v-if="allowInstall">
                <Button
                  icon="pi pi-download"
                  severity="primary"
                  size="sm"
                  rounded
                  :disabled="busy"
                  @click="emit('install', data)"
                />
              </template>
              <template v-else>
                <Button
                  icon="pi pi-arrow-up"
                  severity="info"
                  size="sm"
                  rounded
                  :disabled="busy || !data.outdated"
                  @click="emit('upgrade', data)"
                />
                <Button
                  icon="pi pi-trash"
                  severity="danger"
                  size="sm"
                  rounded
                  :disabled="busy"
                  @click="emit('uninstall', data)"
                />
              </template>
            </div>
          </template>
        </Column>
      </DataTable>
    </template>
  </Card>
</template>

<style scoped>
.table-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.name-cell small {
  color: var(--text-color-secondary);
}

.actions {
  display: flex;
  gap: 8px;
}
</style>
