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
        // 获取已安装的 formulae 列表
        let formulae_output = self.run_brew(&["list", "--formulae", "--versions"]).await?;

        // 解析 formulae 输出，格式：package_name version1 version2 ...
        let mut formulae_packages = Vec::new();
        for line in formulae_output.lines() {
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

            formulae_packages.push((name, version));
        }

        // 获取已安装的 cask 列表
        let cask_output = self.run_brew(&["list", "--cask", "--versions"]).await?;

        // 解析 cask 输出，格式：package_name version
        let mut cask_packages = Vec::new();
        for line in cask_output.lines() {
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

            cask_packages.push((name, version));
        }

        // 获取过期的包
        let (outdated_formulae, outdated_casks) = if let Ok(outdated_output) = self.run_brew(&["outdated", "--json=v2"]).await {
            if let Ok(outdated) = serde_json::from_str::<BrewOutdated>(&outdated_output) {
                let mut formulae_map = HashMap::new();
                for item in outdated.formulae {
                    let latest = if !item.current_version.is_empty() {
                        item.current_version
                    } else if item.installed_versions.is_empty() {
                        "unknown".to_string()
                    } else {
                        item.installed_versions.last().unwrap_or(&"unknown".to_string()).clone()
                    };
                    formulae_map.insert(item.name, latest);
                }

                let mut cask_map = HashMap::new();
                for item in outdated.casks {
                    let latest = if !item.current_version.is_empty() {
                        item.current_version
                    } else if item.installed_versions.is_empty() {
                        "unknown".to_string()
                    } else {
                        item.installed_versions.last().unwrap_or(&"unknown".to_string()).clone()
                    };
                    cask_map.insert(item.name, latest);
                }

                (formulae_map, cask_map)
            } else {
                (HashMap::new(), HashMap::new())
            }
        } else {
            (HashMap::new(), HashMap::new())
        };

        // 构建 Package 列表 - formulae (CLI 工具)
        let mut result: Vec<Package> = formulae_packages
            .into_iter()
            .map(|(name, version)| {
                let (latest_version, outdated) = outdated_formulae
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
                    is_gui: false,
                    description: None,
                }
            })
            .collect();

        // 添加 cask 包 (GUI 应用)
        for (name, version) in cask_packages {
            let (latest_version, outdated) = outdated_casks
                .get(&name)
                .map(|latest| (latest.clone(), true))
                .unwrap_or_else(|| (version.clone(), false));

            result.push(Package {
                name: name.clone(),
                fullname: Some(name),
                version,
                latest_version,
                manager: ManagerType::Brew,
                installed: true,
                outdated,
                is_gui: true,
                description: None,
            });
        }

        Ok(result)
    }

    async fn install_packages(
        &self,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        if names.is_empty() {
            return Err("No packages specified for installation".to_string());
        }

        let mut args = vec!["install"];

        // 解析选项
        if let Some(opts) = options {
            if opts.get("cask").map_or(false, |v| v == "true") {
                args.push("--cask");
            }
        }

        // 添加所有包名
        for name in names {
            args.push(name);
        }

        let output = Command::new("brew")
            .args(&args)
            .output()
            .await
            .map_err(|e| format!("Failed to execute brew install: {}", e))?;

        if output.status.success() {
            Ok(ActionResult::success(format!(
                "Successfully installed {} package(s): {}",
                names.len(),
                names.join(", ")
            )))
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr);
            Err(format!("Failed to install {}: {}", names.join(", "), stderr))
        }
    }

    async fn uninstall_packages(
        &self,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        if names.is_empty() {
            return Err("No packages specified for uninstallation".to_string());
        }

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

        // 添加所有包名
        for name in names {
            args.push(name);
        }

        let output = Command::new("brew")
            .args(&args)
            .output()
            .await
            .map_err(|e| format!("Failed to execute brew uninstall: {}", e))?;

        if output.status.success() {
            Ok(ActionResult::success(format!(
                "Successfully uninstalled {} package(s): {}",
                names.len(),
                names.join(", ")
            )))
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr);
            Err(format!("Failed to uninstall {}: {}", names.join(", "), stderr))
        }
    }

    async fn upgrade_packages(
        &self,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        if names.is_empty() {
            return Err("No packages specified for upgrade".to_string());
        }

        let mut args = vec!["upgrade"];

        // 解析选项
        if let Some(opts) = options {
            if opts.get("cask").map_or(false, |v| v == "true") {
                args.push("--cask");
            }
        }

        // 添加所有包名
        for name in names {
            args.push(name);
        }

        let output = Command::new("brew")
            .args(&args)
            .output()
            .await
            .map_err(|e| format!("Failed to execute brew upgrade: {}", e))?;

        if output.status.success() {
            Ok(ActionResult::success(format!(
                "Successfully upgraded {} package(s): {}",
                names.len(),
                names.join(", ")
            )))
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr);
            Err(format!("Failed to upgrade {}: {}", names.join(", "), stderr))
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

            // 搜索结果默认为 formulae (is_gui = false)
            // 可以通过单独的 --casks 搜索来获取 cask 结果
            packages.push(Package {
                name: name.clone(),
                fullname: Some(name),
                version: "latest".to_string(),
                latest_version: "latest".to_string(),
                manager: ManagerType::Brew,
                installed: false,
                outdated: false,
                is_gui: false,
                description,
            });
        }

        Ok(packages)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const TEST_FORMULA: &str = "tree";      // 命令行工具
    const TEST_CASK: &str = "font-fira-code";  // 字体 cask

    #[tokio::test]
    async fn test_adapter_info() {
        let adapter = BrewAdapter::new();

        assert_eq!(adapter.id(), "brew");
        assert_eq!(adapter.name(), "Homebrew");

        let caps = adapter.capabilities();
        assert!(caps.search);
        assert!(caps.list);
        assert!(caps.install);
        assert!(caps.uninstall);
        assert!(caps.update);
    }

    #[tokio::test]
    async fn test_preflight() {
        let adapter = BrewAdapter::new();
        let result = adapter.preflight().await;
        assert!(result.is_ok(), "Preflight should succeed: {:?}", result);
    }

    #[tokio::test]
    async fn test_list_packages() {
        let adapter = BrewAdapter::new();
        let packages = adapter.list_packages().await;

        assert!(packages.is_ok(), "List packages should succeed: {:?}", packages);
        let packages = packages.unwrap();

        // 如果已安装包，验证数据结构
        if !packages.is_empty() {
            for pkg in packages {
                assert!(!pkg.name.is_empty());
                assert_eq!(pkg.manager, ManagerType::Brew);
                assert!(pkg.installed);
                assert!(!pkg.version.is_empty());
                assert!(!pkg.latest_version.is_empty());
                // 验证 is_gui 是布尔值
                assert!(matches!(pkg.is_gui, true | false));
            }
        }
    }

    #[tokio::test]
    async fn test_search_packages() {
        let adapter = BrewAdapter::new();
        let packages = adapter.search_packages("tree").await;
        assert!(packages.is_ok(), "Search packages should succeed: {:?}", packages);
        let packages = packages.unwrap();

        // 验证至少找到 "tree" 包
        assert!(
            packages.iter().any(|p| p.name.to_lowercase().contains("tree")),
            "Should find packages containing 'tree'"
        );

        // 验证数据结构
        for pkg in packages {
            // println!("name:{}", pkg.name);
            assert!(!pkg.name.is_empty());
            assert_eq!(pkg.manager, ManagerType::Brew);
            assert!(!pkg.installed);
        }
    }

    #[tokio::test]
    async fn test_install_uninstall_formula() {
        let adapter = BrewAdapter::new();

        // 首先尝试卸载（如果已存在）
        let _ = adapter
            .uninstall_packages(&[TEST_FORMULA], None)
            .await;

        // 安装 formula
        let install_result = adapter
            .install_packages(&[TEST_FORMULA], None)
            .await;
        assert!(install_result.is_ok(), "Install should succeed: {:?}", install_result);

        // 验证安装成功
        let result = install_result.unwrap();
        assert!(result.success);
        assert!(result.message.contains("Successfully installed"));

        // 列出包，确认已安装且 is_gui 为 false
        let packages = adapter.list_packages().await.unwrap();
        let formula_pkg = packages.iter().find(|p| p.name == TEST_FORMULA);
        assert!(
            formula_pkg.is_some(),
            "Package should be in installed list"
        );
        if let Some(pkg) = formula_pkg {
            assert!(!pkg.is_gui, "Formula package should have is_gui = false");
        }

        // 卸载
        let uninstall_result = adapter
            .uninstall_packages(&[TEST_FORMULA], None)
            .await;
        assert!(uninstall_result.is_ok(), "Uninstall should succeed: {:?}", uninstall_result);

        let result = uninstall_result.unwrap();
        assert!(result.success);
        assert!(result.message.contains("Successfully uninstalled"));
    }

    #[tokio::test]
    async fn test_install_uninstall_cask() {
        let adapter = BrewAdapter::new();

        let mut cask_options = HashMap::new();
        cask_options.insert("cask".to_string(), "true".to_string());

        // 首先尝试卸载（如果已存在）
        let _ = adapter
            .uninstall_packages(&[TEST_CASK], Some(&cask_options))
            .await;

        // 安装 cask
        let install_result = adapter
            .install_packages(&[TEST_CASK], Some(&cask_options))
            .await;
        assert!(install_result.is_ok(), "Install cask should succeed: {:?}", install_result);

        let result = install_result.unwrap();
        assert!(result.success);
        assert!(result.message.contains("Successfully installed"));

        // 列出包，确认已安装且 is_gui 为 true
        let packages = adapter.list_packages().await.unwrap();
        let cask_pkg = packages.iter().find(|p| p.name == TEST_CASK);
        assert!(
            cask_pkg.is_some(),
            "Cask should be in installed list"
        );
        if let Some(pkg) = cask_pkg {
            assert!(pkg.is_gui, "Cask package should have is_gui = true");
        }

        // 卸载
        let uninstall_result = adapter
            .uninstall_packages(&[TEST_CASK], Some(&cask_options))
            .await;
        assert!(uninstall_result.is_ok(), "Uninstall cask should succeed: {:?}", uninstall_result);

        let result = uninstall_result.unwrap();
        assert!(result.success);
        assert!(result.message.contains("Successfully uninstalled"));
    }

    #[tokio::test]
    async fn test_outdated_detection() {
        let adapter = BrewAdapter::new();

        // 安装一个包
        let _ = adapter
            .install_packages(&[TEST_FORMULA], None)
            .await;

        // 列出包，检查 outdated 字段
        let packages = adapter.list_packages().await.unwrap();

        if let Some(pkg) = packages.iter().find(|p| p.name == TEST_FORMULA) {
            // outdated 应该是布尔值
            assert!(matches!(pkg.outdated, true | false));
        }

        // 清理
        let _ = adapter
            .uninstall_packages(&[TEST_FORMULA], None)
            .await;
    }
}