import { invoke } from '@tauri-apps/api/core'
import type { ActionResult, ManagerInfo, Package, PackageTarget } from '@/model/types'

type CommandOptions = Record<string, string>

export function listManagers() {
    return invoke<ManagerInfo[]>('list_managers')
}

export function loadCachedPackages() {
    return invoke<Package[]>('load_cached_packages')
}

export function reloadPackages() {
    return invoke<void>('reload_packages')
}


export function searchPackages(keyword: string) {
    return invoke<Package[]>('search_packages', { keyword })
}

export function getPackageVersions(manager: Package['manager'], name: string) {
    return invoke<string[]>('get_package_versions', { manager, name })
}

export function installPackage(
    manager: Package['manager'],
    name: string,
    options?: CommandOptions
) {
    return invoke<ActionResult>('install_package', { manager, name, options })
}

export function uninstallPackage(
    manager: Package['manager'],
    name: string,
    options?: CommandOptions
) {
    return invoke<ActionResult>('uninstall_package', { manager, name, options })
}

export function upgradePackage(
    manager: Package['manager'],
    name: string,
    options?: CommandOptions
) {
    return invoke<ActionResult>('upgrade_package', { manager, name, options })
}

export function batchUninstallPackages(packages: PackageTarget[], options?: CommandOptions) {
    return invoke<ActionResult>('batch_uninstall_packages', { packages, options })
}

export function batchUpgradePackages(packages: PackageTarget[], options?: CommandOptions) {
    return invoke<ActionResult>('batch_upgrade_packages', { packages, options })
}
