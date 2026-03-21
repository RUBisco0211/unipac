import type { ManagerCapabilities, ManagerInfo, ManagerType, Package } from '@/model/types'

export const managerLabelMap: Record<ManagerType, string> = {
    brew: 'Homebrew',
    npm: 'npm',
    pip: 'pip',
    cargo: 'cargo',
}

export function countInstalledSummary(packages: Package[]) {
    return {
        total: packages.length,
        gui: packages.filter(pkg => pkg.is_gui).length,
        outdated: packages.filter(pkg => pkg.outdated).length,
    }
}

export function getManagerName(managerId: string, managers: ManagerInfo[]) {
    return managers.find(manager => manager.id === managerId)?.name ?? managerId
}

export function enabledCapabilities(capabilities: ManagerCapabilities) {
    return [
        capabilities.list && 'list',
        capabilities.search && 'search',
        capabilities.install && 'install',
        capabilities.uninstall && 'uninstall',
        capabilities.update && 'update',
    ].filter(Boolean) as string[]
}
