import {
    BatchUninstallPackages,
    BatchUpdatePackages,
    GetCachedPackages,
    ListManagers,
    UninstallPackage,
    UpdateCache,
    UpdatePackage,
} from '@/../wailsjs/go/main/App'
import type { manager } from '@/../wailsjs/go/models'
import type { ActionResult, ManagerInfo, Package, PackageTarget } from '@/model/types'

type CommandOptions = Record<string, string>

function normalizeCapabilities(capabilities?: manager.Capabilities): ManagerInfo['capabilities'] {
    return {
        list: Boolean(capabilities?.list_installed),
        search: Boolean(capabilities?.search),
        install: Boolean(capabilities?.install),
        uninstall: Boolean(capabilities?.uninstall),
        update: Boolean(capabilities?.update),
        listVersions: Boolean(capabilities?.list_versions),
    }
}

function normalizeManager(info: manager.Info): ManagerInfo {
    return {
        id: info.id ?? '',
        name: info.name ?? info.id ?? '',
        capabilities: normalizeCapabilities(info.capabilities),
        enabled: Boolean(info.enabled),
    }
}

function normalizePackage(pkg: manager.Package): Package {
    return {
        name: pkg.name ?? '',
        fullname: pkg.fullname || undefined,
        version: pkg.version ?? '',
        latest_version: pkg.latest_version ?? '',
        manager: pkg.manager ?? '',
        installed: pkg.installed,
        outdated: pkg.outdated,
        is_gui: pkg.is_gui,
        description: pkg.description || undefined,
    }
}

function unsupported(feature: string): never {
    throw new Error(`${feature} is not exposed by the Wails backend yet`)
}

export async function listManagers(): Promise<ManagerInfo[]> {
    const managers = await ListManagers()
    return managers.map(normalizeManager)
}

export async function loadCachedPackages(): Promise<Package[]> {
    const packages = await GetCachedPackages()
    return packages.map(normalizePackage)
}

export async function reloadPackages(): Promise<void> {
    await UpdateCache()
}

export async function searchPackages(_keyword: string): Promise<Package[]> {
    unsupported('Package search')
}

export async function getPackageVersions(
    _manager: Package['manager'],
    _name: string
): Promise<string[]> {
    unsupported('Package version lookup')
}

export async function installPackage(
    _manager: Package['manager'],
    _name: string,
    _options?: CommandOptions
): Promise<ActionResult> {
    unsupported('Package install')
}

export async function uninstallPackage(
    managerId: Package['manager'],
    name: string,
    _options?: CommandOptions
): Promise<ActionResult> {
    const result = await UninstallPackage(managerId, name, {})
    return { success: Boolean(result.success), message: result.message ?? '' }
}

export async function upgradePackage(
    managerId: Package['manager'],
    name: string,
    _options?: CommandOptions
): Promise<ActionResult> {
    const result = await UpdatePackage(managerId, name, {})
    return { success: Boolean(result.success), message: result.message ?? '' }
}

export async function batchUninstallPackages(
    packages: PackageTarget[],
    _options?: CommandOptions
): Promise<ActionResult> {
    const targets = packages.map(p => ({ manager: p.manager, name: p.name, installed: false, outdated: false, is_gui: false }))
    const result = await BatchUninstallPackages(targets, {})
    return { success: Boolean(result.success), message: result.message ?? '' }
}

export async function batchUpgradePackages(
    packages: PackageTarget[],
    _options?: CommandOptions
): Promise<ActionResult> {
    const targets = packages.map(p => ({ manager: p.manager, name: p.name, installed: false, outdated: false, is_gui: false }))
    const result = await BatchUpdatePackages(targets, {})
    return { success: Boolean(result.success), message: result.message ?? '' }
}
