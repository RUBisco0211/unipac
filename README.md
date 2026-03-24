# [Work In Progress] UniPac 

Unipac 是一个用于统一管理 MacOS 上全局包管理器的桌面应用程序。整合了包括`Homebrew`、`npm`、`pip`、`cargo` 在内等软件包管理器，提供一致的用户操作体验。

## 功能特性

- 统一管理通过不同包管理器安装的 CLI 工具和 GUI 应用
- 已支持的包管理器：
  - `Homebrew`
  - `npm (global)`
  - `pip/pip3 (global)`
  - `cargo`
- 功能：
  - 列出所有已安装包及其更新状态
  - 安装、卸载、升级包
  - 网络搜索包（部分）
- 系统代理支持
- 包管理器配置管理
- 日志记录与查看

## 技术栈
- Vue3 + Typescript
- Tauri + Rust

## 开发/自行构建

### 环境要求

- Node.js 18+
- pnpm
- Rust 1.70+
- macOS

### 安装依赖

```bash
pnpm install
cd src-tauri
cargo build
```

### 开发模式

```bash
pnpm dev
```

### 构建

```bash
pnpm build
```

## TODO
- [ ] 用户配置
- [x] Sqlite 缓存优化
...


## 许可证

MIT