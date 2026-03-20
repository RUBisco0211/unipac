use serde::{Deserialize, Serialize};

/// 包管理器类型
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ManagerType {
    Brew,
    Npm,
    Pip,
    Cargo,
}

impl ManagerType {
    pub fn as_str(&self) -> &'static str {
        match self {
            ManagerType::Brew => "brew",
            ManagerType::Npm => "npm",
            ManagerType::Pip => "pip",
            ManagerType::Cargo => "cargo",
        }
    }
}

/// 包管理器能力
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ManagerCapabilities {
    pub search: bool,
    pub list: bool,
    pub install: bool,
    pub uninstall: bool,
    pub update: bool,
}

impl Default for ManagerCapabilities {
    fn default() -> Self {
        Self {
            search: true,
            list: true,
            install: true,
            uninstall: true,
            update: true,
        }
    }
}

/// 包管理器信息
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ManagerInfo {
    pub id: String,
    pub name: String,
    pub capabilities: ManagerCapabilities,
    pub enabled: bool,
}

/// 软件包信息
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Package {
    pub name: String,
    pub fullname: Option<String>,
    pub version: String,
    pub latest_version: String,
    pub manager: ManagerType,
    pub installed: bool,
    pub outdated: bool,
    pub is_gui: bool,  // 是否是 GUI 应用（cask）
    pub description: Option<String>,
}

/// 操作结果
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ActionResult {
    pub success: bool,
    pub message: String,
}

impl ActionResult {
    pub fn success(message: impl Into<String>) -> Self {
        Self {
            success: true,
            message: message.into(),
        }
    }

    pub fn error(message: impl Into<String>) -> Self {
        Self {
            success: false,
            message: message.into(),
        }
    }
}

/// 生命周期阶段
#[allow(dead_code)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LifecyclePhase {
    Preflight,
    Setup,
    Ready,
}