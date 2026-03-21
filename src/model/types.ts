export type ManagerType = 'brew' | 'npm' | 'pip' | 'cargo'

export interface ManagerCapabilities {
  search: boolean
  list: boolean
  install: boolean
  uninstall: boolean
  update: boolean
}

export interface ManagerInfo {
  id: string
  name: string
  capabilities: ManagerCapabilities
  enabled: boolean
}

export interface Package {
  name: string
  fullname?: string
  version: string
  latest_version: string
  manager: ManagerType
  installed: boolean
  outdated: boolean
  is_gui: boolean
  description?: string
}

export interface ActionResult {
  success: boolean
  message: string
}
