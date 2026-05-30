# UniPac

> Work in Progress

UniPac is a desktop application for managing packages from multiple package managers in one place.

UniPac 是一个用于统一管理多个包管理器的软件包的桌面应用，目标是为常见的 CLI 工具和部分 GUI 应用提供一致的管理体验。

## Overview / 项目简介

- Target platform: `macOS`
- Supported managers: `Homebrew`, `npm (global)`, `pip/pip3 (global)`, `cargo`
- Tech stack: `Vue 3 + TypeScript` for the frontend, `Tauri + Rust` for the desktop backend

- 目标平台：`macOS`
- 当前支持的包管理器：`Homebrew`、`npm (global)`、`pip/pip3 (global)`、`cargo`
- 技术栈：前端使用 `Vue 3 + TypeScript`，桌面后端使用 `Tauri + Rust`

## Current Capabilities / 当前能力

- List installed packages and show basic version information
- Search packages across supported managers
- Install, uninstall, and upgrade packages
- Load cached package data and refresh runtime state

- 列出已安装的软件包并展示基础版本信息
- 在已支持的包管理器范围内搜索软件包
- 安装、卸载、升级软件包
- 加载缓存的软件包数据并刷新运行时状态

## Development / 开发说明

### Requirements / 环境要求

- `Node.js 18+`
- `pnpm`
- `Rust 1.70+`
- `macOS`

### Install Dependencies / 安装依赖

```bash
pnpm install
```

### Start Development / 启动开发环境

```bash
pnpm dev
```

The Tauri backend lives in [`src-tauri/`](E:/java%20workspace/unipac/inspect-unipac-2/src-tauri).

Tauri 后端代码位于 [`src-tauri/`](E:/java%20workspace/unipac/inspect-unipac-2/src-tauri)。

### Build / 构建

```bash
pnpm build
```

## Status And Limitations / 当前状态与限制

- UniPac is still under active development.
- The feature set is not final and behavior may change as the project evolves.
- The current README only documents behavior that is directly visible from the repository.

- UniPac 仍处于持续开发阶段。
- 当前功能集合尚未稳定，后续实现和交互可能继续调整。
- 本 README 仅描述当前仓库中可以直接确认的能力，不额外承诺未落地的功能。

## License / 许可证

MIT
