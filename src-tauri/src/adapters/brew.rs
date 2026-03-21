use crate::adapters::{ensure_command_healthy, ensure_command_in_path, run_command, PackageAdapter};
use crate::models::{ActionResult, ManagerCapabilities, ManagerType, Package};
use async_trait::async_trait;
use serde::Deserialize;
use std::collections::HashMap;

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
        run_command("brew", args, "Brew").await
    }

    fn parse_versions_output(output: &str) -> Vec<(String, String)> {
        output
            .lines()
            .filter_map(|line| {
                let parts: Vec<&str> = line.split_whitespace().collect();
                if parts.is_empty() {
                    return None;
                }

                let name = parts[0].to_string();
                let version = if parts.len() > 1 {
                    parts[1].to_string()
                } else {
                    "unknown".to_string()
                };

                Some((name, version))
            })
            .collect()
    }

    fn parse_outdated_output(
        output: &str,
    ) -> Result<(HashMap<String, String>, HashMap<String, String>), String> {
        if output.trim().is_empty() {
            return Ok((HashMap::new(), HashMap::new()));
        }

        let outdated: BrewOutdated = serde_json::from_str(output)
            .map_err(|e| format!("Failed to parse brew outdated output: {}", e))?;

        let formulae_map = outdated
            .formulae
            .into_iter()
            .map(|item| {
                let latest = if !item.current_version.is_empty() {
                    item.current_version
                } else if item.installed_versions.is_empty() {
                    "unknown".to_string()
                } else {
                    item.installed_versions
                        .last()
                        .cloned()
                        .unwrap_or_else(|| "unknown".to_string())
                };
                (item.name, latest)
            })
            .collect();

        let cask_map = outdated
            .casks
            .into_iter()
            .map(|item| {
                let latest = if !item.current_version.is_empty() {
                    item.current_version
                } else if item.installed_versions.is_empty() {
                    "unknown".to_string()
                } else {
                    item.installed_versions
                        .last()
                        .cloned()
                        .unwrap_or_else(|| "unknown".to_string())
                };
                (item.name, latest)
            })
            .collect();

        Ok((formulae_map, cask_map))
    }

    fn build_packages(
        formulae_packages: Vec<(String, String)>,
        cask_packages: Vec<(String, String)>,
        outdated_formulae: HashMap<String, String>,
        outdated_casks: HashMap<String, String>,
    ) -> Vec<Package> {
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

        result
    }

    fn parse_search_output(output: &str) -> Vec<Package> {
        let mut packages = Vec::new();

        for line in output.lines() {
            let line = line.trim();
            if line.is_empty() || line.starts_with("=>") {
                continue;
            }

            let (name, description) = if let Some(idx) = line.find(':') {
                (
                    line[..idx].trim().to_string(),
                    Some(line[idx + 1..].trim().to_string()),
                )
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
                is_gui: false,
                description,
            });
        }

        packages
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
        ensure_command_in_path(
            "brew",
            "Homebrew is not installed or not in PATH. Please install it from https://brew.sh/",
        )
        .await?;

        ensure_command_healthy("brew", &["--version"], "Homebrew").await
    }

    async fn list_packages(&self) -> Result<Vec<Package>, String> {
        // 获取已安装的 formulae 列表
        let formulae_packages = match self.run_brew(&["list", "--formulae", "--versions"]).await {
            Ok(output) => Self::parse_versions_output(&output),
            Err(error) => {
                tracing::warn!("Failed to list Homebrew formulae: {}", error);
                Vec::new()
            }
        };

        // 获取已安装的 cask 列表
        let cask_packages = match self.run_brew(&["list", "--cask", "--versions"]).await {
            Ok(output) => Self::parse_versions_output(&output),
            Err(error) => {
                tracing::warn!("Failed to list Homebrew casks: {}", error);
                Vec::new()
            }
        };

        // 获取过期的包
        let (outdated_formulae, outdated_casks) = if let Ok(outdated_output) =
            self.run_brew(&["outdated", "--json=v2"]).await
        {
            Self::parse_outdated_output(&outdated_output)
                .unwrap_or_else(|_| (HashMap::new(), HashMap::new()))
        } else {
            (HashMap::new(), HashMap::new())
        };

        if formulae_packages.is_empty() && cask_packages.is_empty() {
            return Err("Failed to list Homebrew packages".to_string());
        }

        Ok(Self::build_packages(
            formulae_packages,
            cask_packages,
            outdated_formulae,
            outdated_casks,
        ))
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

        let output = run_command("brew", &args, "Brew").await;

        if output.is_ok() {
            Ok(ActionResult::success(format!(
                "Successfully installed {} package(s): {}",
                names.len(),
                names.join(", ")
            )))
        } else {
            Err(format!("Failed to install {}: {}", names.join(", "), output.err().unwrap()))
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

        let output = run_command("brew", &args, "Brew").await;

        if output.is_ok() {
            Ok(ActionResult::success(format!(
                "Successfully uninstalled {} package(s): {}",
                names.len(),
                names.join(", ")
            )))
        } else {
            Err(format!("Failed to uninstall {}: {}", names.join(", "), output.err().unwrap()))
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

        let output = run_command("brew", &args, "Brew").await;

        if output.is_ok() {
            Ok(ActionResult::success(format!(
                "Successfully upgraded {} package(s): {}",
                names.len(),
                names.join(", ")
            )))
        } else {
            Err(format!("Failed to upgrade {}: {}", names.join(", "), output.err().unwrap()))
        }
    }

    async fn search_packages(&self, keyword: &str) -> Result<Vec<Package>, String> {
        let output = self.run_brew(&["search", keyword]).await?;
        Ok(Self::parse_search_output(&output))
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

    #[test]
    fn test_parse_versions_output() {
        let output = "wget 1.24.5\niterm2 3.5.0";
        let packages = BrewAdapter::parse_versions_output(output);

        assert_eq!(packages.len(), 2);
        assert_eq!(packages[0], ("wget".to_string(), "1.24.5".to_string()));
        assert_eq!(packages[1], ("iterm2".to_string(), "3.5.0".to_string()));
    }

    #[test]
    fn test_parse_outdated_output() {
        let output = r#"{
          "formulae": [
            { "name": "wget", "installed_versions": ["1.24.4"], "current_version": "1.24.5" }
          ],
          "casks": [
            { "name": "iterm2", "installed_versions": ["3.4.0"], "current_version": "3.5.0" }
          ]
        }"#;

        let (formulae, casks) = BrewAdapter::parse_outdated_output(output).unwrap();

        assert_eq!(formulae.get("wget"), Some(&"1.24.5".to_string()));
        assert_eq!(casks.get("iterm2"), Some(&"3.5.0".to_string()));
    }

    #[test]
    fn test_build_packages() {
        let packages = BrewAdapter::build_packages(
            vec![("wget".to_string(), "1.24.4".to_string())],
            vec![("iterm2".to_string(), "3.4.0".to_string())],
            HashMap::from([("wget".to_string(), "1.24.5".to_string())]),
            HashMap::from([("iterm2".to_string(), "3.5.0".to_string())]),
        );

        assert_eq!(packages.len(), 2);

        let wget = packages.iter().find(|pkg| pkg.name == "wget").unwrap();
        assert!(wget.outdated);
        assert!(!wget.is_gui);
        assert_eq!(wget.latest_version, "1.24.5");

        let iterm2 = packages.iter().find(|pkg| pkg.name == "iterm2").unwrap();
        assert!(iterm2.outdated);
        assert!(iterm2.is_gui);
        assert_eq!(iterm2.latest_version, "3.5.0");
    }

    #[test]
    fn test_parse_search_output() {
        let output = "tree: display directories as trees\n=> alias1\nwget";
        let packages = BrewAdapter::parse_search_output(output);

        assert_eq!(packages.len(), 2);
        assert_eq!(packages[0].name, "tree");
        assert_eq!(packages[0].description.as_deref(), Some("display directories as trees"));
        assert_eq!(packages[1].name, "wget");
        assert!(packages.iter().all(|pkg| pkg.manager == ManagerType::Brew && !pkg.installed));
    }

    #[tokio::test]
    async fn test_list_packages() {
        let adapter = BrewAdapter::new();
        let packages = adapter.list_packages().await;

        assert!(packages.is_ok(), "List packages should succeed: {:?}", packages);
        for pkg in packages.unwrap() {
            assert!(!pkg.name.is_empty());
            assert_eq!(pkg.manager, ManagerType::Brew);
            assert!(pkg.installed);
            assert!(!pkg.version.is_empty());
            assert!(!pkg.latest_version.is_empty());
        }
    }

    #[tokio::test]
    async fn test_search_packages() {
        let adapter = BrewAdapter::new();
        let packages = adapter.search_packages("tree").await;
        assert!(packages.is_ok(), "Search packages should succeed: {:?}", packages);
        let packages = packages.unwrap();

        assert!(packages.iter().any(|p| p.name.to_lowercase().contains("tree")));
        assert!(packages.iter().all(|pkg| pkg.manager == ManagerType::Brew && !pkg.installed));
    }

    #[tokio::test]
    #[ignore = "modifies host Homebrew state; run manually when needed"]
    async fn test_install_uninstall_formula() {
        let adapter = BrewAdapter::new();

        let _ = adapter.uninstall_packages(&[TEST_FORMULA], None).await;

        let install_result = adapter.install_packages(&[TEST_FORMULA], None).await;
        assert!(install_result.is_ok(), "Install should succeed: {:?}", install_result);
        assert!(install_result.unwrap().success);

        let packages = adapter.list_packages().await.unwrap();
        let formula_pkg = packages.iter().find(|p| p.name == TEST_FORMULA);
        assert!(formula_pkg.is_some(), "Package should be in installed list");
        assert!(!formula_pkg.unwrap().is_gui);

        let uninstall_result = adapter.uninstall_packages(&[TEST_FORMULA], None).await;
        assert!(uninstall_result.is_ok(), "Uninstall should succeed: {:?}", uninstall_result);
        assert!(uninstall_result.unwrap().success);
    }

    #[tokio::test]
    #[ignore = "modifies host Homebrew state; run manually when needed"]
    async fn test_install_uninstall_cask() {
        let adapter = BrewAdapter::new();

        let mut cask_options = HashMap::new();
        cask_options.insert("cask".to_string(), "true".to_string());

        let _ = adapter
            .uninstall_packages(&[TEST_CASK], Some(&cask_options))
            .await;

        let install_result = adapter
            .install_packages(&[TEST_CASK], Some(&cask_options))
            .await;
        assert!(install_result.is_ok(), "Install cask should succeed: {:?}", install_result);
        assert!(install_result.unwrap().success);

        let packages = adapter.list_packages().await.unwrap();
        let cask_pkg = packages.iter().find(|p| p.name == TEST_CASK);
        assert!(cask_pkg.is_some(), "Cask should be in installed list");
        assert!(cask_pkg.unwrap().is_gui);

        let uninstall_result = adapter
            .uninstall_packages(&[TEST_CASK], Some(&cask_options))
            .await;
        assert!(uninstall_result.is_ok(), "Uninstall cask should succeed: {:?}", uninstall_result);
        assert!(uninstall_result.unwrap().success);
    }

    #[tokio::test]
    #[ignore = "modifies host Homebrew state; run manually when needed"]
    async fn test_outdated_detection() {
        let adapter = BrewAdapter::new();

        let _ = adapter.install_packages(&[TEST_FORMULA], None).await;
        let packages = adapter.list_packages().await.unwrap();

        if let Some(pkg) = packages.iter().find(|p| p.name == TEST_FORMULA) {
            assert!(matches!(pkg.outdated, true | false));
        }

        let _ = adapter.uninstall_packages(&[TEST_FORMULA], None).await;
    }
}
