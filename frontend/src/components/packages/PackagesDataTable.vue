<script setup lang="ts">
import {
    FlexRender,
    getCoreRowModel,
    getSortedRowModel,
    useVueTable,
    type ColumnDef,
    type RowSelectionState,
    type SortingState,
} from '@tanstack/vue-table'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Package } from '@/model/types'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'

const props = withDefaults(
    defineProps<{
        columns: ColumnDef<Package, unknown>[]
        data: Package[]
        emptyText?: string
        rowSelection?: RowSelectionState
    }>(),
    {
        rowSelection: () => ({}),
    }
)

const emit = defineEmits<{
    'update:rowSelection': [value: RowSelectionState]
}>()

const { t } = useI18n()
const sorting = ref<SortingState>([])

const table = useVueTable({
    get data() {
        return props.data
    },
    get columns() {
        return props.columns
    },
    getRowId: row => `${row.manager}:${row.name}`,
    enableRowSelection: true,
    state: {
        get rowSelection() {
            return props.rowSelection
        },
        get sorting() {
            return sorting.value
        },
    },
    onRowSelectionChange: updater => {
        const nextValue = typeof updater === 'function' ? updater(props.rowSelection) : updater
        emit('update:rowSelection', nextValue)
    },
    onSortingChange: updater => {
        sorting.value = typeof updater === 'function' ? updater(sorting.value) : updater
    },
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
})
</script>

<template>
    <div class="bg-transparent">
        <Table>
            <TableHeader>
                <TableRow
                    v-for="headerGroup in table.getHeaderGroups()"
                    :key="headerGroup.id"
                    class="hover:bg-transparent"
                >
                    <TableHead
                        v-for="header in headerGroup.headers"
                        :key="header.id"
                        :aria-sort="
                            header.column.getIsSorted() === 'asc'
                                ? 'ascending'
                                : header.column.getIsSorted() === 'desc'
                                  ? 'descending'
                                  : undefined
                        "
                        :class="[
                            header.column.id === 'actions' ? 'text-right' : '',
                            header.column.getCanSort()
                                ? 'cursor-pointer select-none hover:text-[hsl(var(--foreground))]'
                                : '',
                        ]"
                        @click="header.column.getToggleSortingHandler()?.($event)"
                    >
                        <div
                            v-if="!header.isPlaceholder"
                            class="inline-flex items-center gap-1"
                            :class="header.column.id === 'actions' ? 'justify-end' : ''"
                        >
                            <FlexRender
                                :render="header.column.columnDef.header"
                                :props="header.getContext()"
                            />
                            <span
                                v-if="header.column.getCanSort()"
                                class="w-3 text-[10px] text-[hsl(var(--muted-foreground))]"
                            >
                                {{
                                    header.column.getIsSorted() === 'asc'
                                        ? '▲'
                                        : header.column.getIsSorted() === 'desc'
                                          ? '▼'
                                          : ''
                                }}
                            </span>
                        </div>
                    </TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                <template v-if="table.getRowModel().rows.length">
                    <TableRow
                        v-for="row in table.getRowModel().rows"
                        :key="row.id"
                        :data-state="row.getIsSelected() ? 'selected' : undefined"
                    >
                        <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                            <FlexRender
                                :render="cell.column.columnDef.cell"
                                :props="cell.getContext()"
                            />
                        </TableCell>
                    </TableRow>
                </template>
                <TableRow v-else>
                    <TableCell
                        :colspan="columns.length"
                        class="h-16 text-center text-[hsl(var(--muted-foreground))]"
                    >
                        {{ emptyText || t('table.noResults') }}
                    </TableCell>
                </TableRow>
            </TableBody>
        </Table>
    </div>
</template>
