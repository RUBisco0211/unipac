# 全局包管理器设计文档（Tauri 版本）

## 1. 软件用途

该软件用于统一管理 macOS 上通过不同包管理器安装的全局工具，包括 CLI 工具和 GUI 应用。

目标：

* 统一展示所有已安装工具
* 提供安装、卸载、升级能力
* 屏蔽不同包管理器差异
* 提供一致的用户操作体验

---

## 2. 功能范围

当前支持的包管理器：

* Homebrew
* npm（global）
* pip或pip3（global）
* cargo

核心功能：

* 列出所有已安装包（含更新状态）
* 安装包
* 卸载包
* 升级包
* 搜索包（可选，按 manager 能力决定）

---

## 3. 开发架构

```text
前端 (Vue)
   │
   ▼
Tauri invoke API
   │
   ▼
Rust 后端
   │
   ├── ManagerRegistry（全局调度）
   ├── Adapter Layer（各包管理器实现）
   ├── Lifecycle（preflight / setup / ready）
   │
   ▼
各包管理器 CLI
```

---

## 4. 数据模型（参考，具体实现时再商议）

以下数据模型虽然都以ts呈现，但在Rust中也需要定义，且主要逻辑在Rust中，且只是暂时设计，有需要修改请在你考虑清楚之后和我商议

### 4.1 Package（参考）

```typescript
type Package = {
  name: string
  fullname?: string
  version: string
  latestVersion: string
  manager: "brew" | "npm" | "pip" | "cargo"
  installed: boolean
  outdated: boolean
  description?: string
}
```

说明：

* `manager`：用于后端路由到对应 Adapter
* `outdated`：是否可更新（由后端计算）
* `latestVersion`：用于 UI 显示
* 不包含任何 manager-specific 字段

---

### 4.2 ManagerInfo（参考）

```ts
type ManagerInfo = {
  id: string
  name: string

  capabilities: {
    search: boolean
    list: boolean
    install: boolean
    uninstall: boolean
    update: boolean
  }
  enabled: boolean // 是否启用，用户可以手动关闭或启用部分包管理器
}
```

---

## 5. Tauri 前后端通信接口（参考，具体实现时再商议）

所有接口通过 `invoke` 调用，例如：

```ts
invoke("function_name", payload)
```

---

## 5.1 获取所有包管理器状态

### 前端

```ts
invoke("list_managers")
```

### 返回

```ts
ManagerInfo[]
```

---

## 5.2 获取所有已安装包

### 前端

```ts
invoke("list_installed_packages")
```

### 返回

```ts
Package[]
```

说明：
* 返回所有 manager 的合并结果，后端最好是对几个包管理器的指令执行和结果解析做并发执行然后合并
---

## 5.3 安装包（以下options字段具体开发时根据不同包管理器的不同选项，再具体和我商议）

### 前端

```ts
invoke("install_package", {
  package: Package,
  options?: Record<string, any>
})
```

### 返回

```ts
ActionResult
```

---

## 5.4 卸载包

### 前端

```ts
invoke("uninstall_package", {
  package: Package,
  options?: Record<string, any>
})
```

### 返回

```ts
ActionResult
```

---

## 5.5 升级包

### 前端

```ts
invoke("upgrade_package", {
  package: Package,
  options?: Record<string, any>
})
```

### 返回

```ts
ActionResult
```

---

## 5.6 网络搜索包（统一搜索，仅对支持的包管理器做）

### 前端

```ts
invoke("search_packages", {
  keyword: string
})
```

### 返回

```ts
Package[]
```

---

## 6. 后端设计规范（参考）

### 6.1 Adapter 设计

每个包管理器实现统一接口（Trait）：

* 负责 CLI 调用
* 负责输出解析
* 负责参数映射
* 不暴露差异给前端

---

### 6.2 生命周期

每个 Adapter 必须实现：

1. **preflight**

   * 检查命令是否存在
   * 检查依赖（如 cargo-update）

2. **setup**

   * 安装缺失依赖（可选）

---

### 6.3 ManagerRegistry

职责：

* 注册所有 Adapter（哈希表）
* 管理生命周期状态
* 分发调用请求

前端调用流程：

```text
invoke → registry → adapter → CLI
```

---

### 6.4 Package 构建规范

Adapter 必须：

* 将 CLI 输出解析为统一 `Package` 列表

---

### 6.5 错误处理

* 所有操作返回 `Result<T, String>`
* 前端统一转换为：

```ts
ActionResult
```

* 错误信息应：

  * 清晰
  * 可读
  * 不暴露底层实现细节

---

### 6.6 参数处理

* 前端仅传：

```ts
options?: Record<string, any>
```

* 后端 Adapter 负责：

  * 参数解析
  * 映射到 CLI 参数
* 不在接口层定义具体参数结构（延后设计）

---

## 7. 前端职责边界

前端：

* 只使用 `Package` 和 `ManagerInfo`
* 不处理包管理器差异
* 不解析 CLI 输出
* 根据 capabilities 控制 UI

后端：

* 处理所有差异
* 控制生命周期
* 解析数据
* 执行命令

---

## 8. 日志管理

需要提供统一的控制台和文件日志记录，要记录具体执行的cli命令和结果，且允许前端查看。具体要求开发时完善

---

## 9. 用户配置

- 系统代理（如果配置了就尽可能让包管理器在安装和搜索时用上）
- 允许配置各个包管理器的一些功能（开发时具体安排）
- 允许配置文件导出
- ... 其他的帮我想想

---

## 10. 前端设计

- ui参考 shadcn 的风格，使用vue + tailwind-css
- 实现根据系统的夜间模式进行同步切换
- 左侧侧边栏功能区：
  - 上方： 已安装页面（筛选、更新、安装和卸载，用表格呈现，要求有页面 keepalive 缓存）、搜索页面（网络搜索）
  - 下方： 用户设置页面（启用或禁用某个包管理器，包管理器具体配置、系统代理等）