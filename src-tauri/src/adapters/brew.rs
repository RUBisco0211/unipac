use crate::adapters::PackageAdapter;
use crate::models::{ActionResult, ManagerCapabilities, ManagerType, Package};
use async_trait::async_trait;
use serde::Deserialize;
use std::collections::HashMap;
use tokio::process::Command;

/// Homebrew outdated JSON 输出
#[derive(Debug, Deserialize)]
struct BrewOutdated {
    formulae: Vec<BrewOutdatedItem>,
    casks: Vec<BrewOutdatedItem>,
}

#[derive(Debug, Deserialize)]
struct BrewOutdatedItem {
    name: String,
    installed_versions: Vec<String>,
    current_version: String,
}

/// Homebrew 适配器
pub struct BrewAdapter;

impl BrewAdapter {
    pub fn new() -> Self {
        Self
    }

    /// 执行 brew 命令并获取输出
    async fn run_brew(&self, args: &[&str]) -> Result<String, String> {
        let output = Command::new("brew")
            .args(args)
            .output()
            .await
            .map_err(|e| format!("Failed to execute brew: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("Brew command failed: {}", stderr));
        }

        Ok(String::from_utf8_lossy(&output.stdout).to_string())
    }
}

#[async_trait]
impl PackageAdapter for BrewAdapter {
    fn id(&self) -> &str {
        "brew"
    }

    fn name(&self) -> &str {
        "Homebrew"
    }

    fn capabilities(&self) -> ManagerCapabilities {
        ManagerCapabilities::default()
    }

    async fn preflight(&self) -> Result<(), String> {
        // 检查 brew 命令是否存在
        let result = Command::new("which")
            .arg("brew")
            .output()
            .await
            .map_err(|e| format!("Failed to check for brew: {}", e))?;

        if !result.status.success() {
            return Err(
                "Homebrew is not installed or not in PATH. Please install it from https://brew.sh/"
                    .to_string(),
            );
        }

        // 检查 brew 是否可用
        let version_result = Command::new("brew")
            .arg("--version")
            .output()
            .await;

        match version_result {
            Ok(output) if output.status.success() => Ok(()),
            Ok(_) => Err("Homebrew command exists but is not functional".to_string()),
            Err(e) => Err(format!("Failed to run Homebrew: {}", e)),
        }
    }

    async fn list_packages(&self) -> Result<Vec<Package>, String> {
        // 获取已安装的包列表
        let list_output = self.run_brew(&["list", "--versions"]).await?;

        // 解析列表输出，格式：package_name version1 version2 ...
        let mut packages = Vec::new();
        for line in list_output.lines() {
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.is_empty() {
                continue;
            }

            let name = parts[0].to_string();
            let version = if parts.len() > 1 {
                parts[1].to_string()
            } else {
                "unknown".to_string()
            };

            packages.push((name, version));
        }

        // 获取过期的包
        let outdated_map = if let Ok(outdated_output) = self.run_brew(&["outdated", "--json=v2"]).await {
            if let Ok(outdated) = serde_json::from_str::<BrewOutdated>(&outdated_output) {
                let mut map = HashMap::new();
                for item in outdated.formulae {
                    let latest = if !item.current_version.is_empty() {
                        item.current_version
                    } else if item.installed_versions.is_empty() {
                        "unknown".to_string()
                    } else {
                        item.installed_versions.last().unwrap_or(&"unknown".to_string()).clone()
                    };
                    map.insert(item.name, latest);
                }
                for item in outdated.casks {
                    let latest = if !item.current_version.is_empty() {
                        item.current_version
                    } else if item.installed_versions.is_empty() {
                        "unknown".to_string()
                    } else {
                        item.installed_versions.last().unwrap_or(&"unknown".to_string()).clone()
                    };
                    map.insert(item.name, latest);
                }
                map
            } else {
                HashMap::new()
            }
        } else {
            HashMap::new()
        };

        // 构建 Package 列表
        let result: Vec<Package> = packages
            .into_iter()
            .map(|(name, version)| {
                let (latest_version, outdated) = outdated_map
                    .get(&name)
                    .map(|latest| (latest.clone(), true))
                    .unwrap_or_else(|| (version.clone(), false));

                Package {
                    name: name.clone(),
                    fullname: Some(name),
                    version,
                    latest_version,
                    manager: ManagerType::Brew,
                    installed: true,
                    outdated,
                    description: None,
                }
            })
            .collect();

        Ok(result)
    }

    async fn install_package(
        &self,
        name: &str,
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        let mut args = vec!["install"];

        // 解析选项
        if let Some(opts) = options {
            if opts.get("cask").map_or(false, |v| v == "true") {
                args.push("--cask");
            }
        }

        args.push(name);

        let output = Command::new("brew")
            .args(&args)
            .output()
            .await
            .map_err(|e| format!("Failed to execute brew install: {}", e))?;

        if output.status.success() {
            Ok(ActionResult::success(format!(
                "Successfully installed {}",
                name
            )))
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr);
            Err(format!("Failed to install {}: {}", name, stderr))
        }
    }

    async fn uninstall_package(
        &self,
        name: &str,
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        let mut args = vec!["uninstall"];

        // 解析选项
        if let Some(opts) = options {
            if opts.get("cask").map_or(false, |v| v == "true") {
                args.push("--cask");
            }
            if opts.get("zap").map_or(false, |v| v == "true") {
                args.push("--zap");
            }
        }

        args.push(name);

        let output = Command::new("brew")
            .args(&args)
            .output()
            .await
            .map_err(|e| format!("Failed to execute brew uninstall: {}", e))?;

        if output.status.success() {
            Ok(ActionResult::success(format!(
                "Successfully uninstalled {}",
                name
            )))
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr);
            Err(format!("Failed to uninstall {}: {}", name, stderr))
        }
    }

    async fn upgrade_package(
        &self,
        name: &str,
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        let mut args = vec!["upgrade"];

        // 解析选项
        if let Some(opts) = options {
            if opts.get("cask").map_or(false, |v| v == "true") {
                args.push("--cask");
            }
        }

        args.push(name);

        let output = Command::new("brew")
            .args(&args)
            .output()
            .await
            .map_err(|e| format!("Failed to execute brew upgrade: {}", e))?;

        if output.status.success() {
            Ok(ActionResult::success(format!(
                "Successfully upgraded {}",
                name
            )))
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr);
            Err(format!("Failed to upgrade {}: {}", name, stderr))
        }
    }

    async fn search_packages(&self, keyword: &str) -> Result<Vec<Package>, String> {
        let output = self.run_brew(&["search", keyword]).await?;

        let mut packages = Vec::new();
        for line in output.lines() {
            let line = line.trim();
            if line.is_empty() {
                continue;
            }

            // 过滤掉 "=>" 开头的行（别名引用）
            if line.starts_with("=>") {
                continue;
            }

            // Homebrew search 输出格式可能包含描述
            let (name, description) = if let Some(idx) = line.find(':') {
                (line[..idx].trim().to_string(), Some(line[idx + 1..].trim().to_string()))
            } else {
                (line.to_string(), None)
            };

            packages.push(Package {
                name: name.clone(),
                fullname: Some(name),
                version: "latest".to_string(),
                latest_version: "latest".to_string(),
                manager: ManagerType::Brew,
                installed: false,
                outdated: false,
                description,
            });
        }

        Ok(packages)
    }
}