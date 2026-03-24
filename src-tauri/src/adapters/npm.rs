use crate::adapters::{ensure_command_healthy, ensure_command_in_path, run_command, PackageAdapter};
use crate::models::{ActionResult, ManagerCapabilities, ManagerType, Package};
use async_trait::async_trait;
use serde::Deserialize;
use std::collections::HashMap;

/// npm list JSON 输出
#[derive(Debug, Deserialize)]
struct NpmListOutput {
    dependencies: Option<HashMap<String, NpmListDependency>>,
}

#[derive(Debug, Deserialize)]
struct NpmListDependency {
    version: Option<String>,
}

#[derive(Debug, Deserialize)]
struct NpmOutdatedPackage {
    current: String,
    latest: String,
}

/// npm 适配器
pub struct NpmAdapter;

impl NpmAdapter {
    pub fn new() -> Self {
        Self
    }

    /// 执行 npm 命令并获取输出
    async fn run_npm(&self, args: &[&str]) -> Result<String, String> {
        run_command("npm", args, "npm").await
    }

    fn parse_list_output(list_output: &str) -> Result<Vec<Package>, String> {
        let parsed: NpmListOutput = serde_json::from_str(list_output)
            .map_err(|e| format!("Failed to parse npm list output: {}", e))?;

        let packages = parsed
            .dependencies
            .unwrap_or_default()
            .into_iter()
            .map(|(name, dependency)| {
                let version = dependency.version.unwrap_or_else(|| "unknown".to_string());

                Package {
                    name: name.clone(),
                    fullname: Some(name),
                    version: version.clone(),
                    latest_version: version,
                    manager: ManagerType::Npm,
                    installed: true,
                    outdated: false,
                    is_gui: false,
                    description: None,
                }
            })
            .collect();

        Ok(packages)
    }

    fn apply_outdated_data(
        packages: &mut [Package],
        outdated_output: &str,
    ) -> Result<(), String> {
        if outdated_output.trim().is_empty() {
            return Ok(());
        }

        let parsed: HashMap<String, NpmOutdatedPackage> = serde_json::from_str(outdated_output)
            .map_err(|e| format!("Failed to parse npm outdated output: {}", e))?;

        for package in packages.iter_mut() {
            if let Some(outdated) = parsed.get(&package.name) {
                package.latest_version = outdated.latest.clone();
                package.outdated = outdated.current != outdated.latest;
            }
        }

        Ok(())
    }

    fn parse_search_output(search_output: &str) -> Result<Vec<Package>, String> {
        let search_results: Vec<serde_json::Value> = serde_json::from_str(search_output)
            .map_err(|e| format!("Failed to parse npm search output: {}", e))?;

        let packages = search_results
            .into_iter()
            .filter_map(|item| {
                let name = item.get("name")?.as_str()?.to_string();
                let desc = item
                    .get("description")
                    .and_then(|d| d.as_str())
                    .map(String::from);
                let version = item
                    .get("version")
                    .and_then(|v| v.as_str())
                    .unwrap_or("latest")
                    .to_string();

                Some(Package {
                    name: name.clone(),
                    fullname: Some(name),
                    version: version.clone(),
                    latest_version: version,
                    manager: ManagerType::Npm,
                    installed: false,
                    outdated: false,
                    is_gui: false,
                    description: desc,
                })
            })
            .collect();

        Ok(packages)
    }
}

#[async_trait]
impl PackageAdapter for NpmAdapter {
    fn id(&self) -> &str {
        "npm"
    }

    fn name(&self) -> &str {
        "npm"
    }

    fn capabilities(&self) -> ManagerCapabilities {
        ManagerCapabilities::default()
    }

    async fn preflight(&self) -> Result<(), String> {
        ensure_command_in_path(
            "npm",
            "npm is not installed or not in PATH. Please install Node.js from https://nodejs.org/",
        )
        .await?;

        ensure_command_healthy("npm", &["--version"], "npm").await
    }

    async fn list_packages(&self) -> Result<Vec<Package>, String> {
        // npm list -g --depth=0 --json 获取全局已安装包
        let list_output = self.run_npm(&["list", "-g", "--depth=0", "--json"]).await?;
        let mut packages = Self::parse_list_output(&list_output)?;

        if let Ok(outdated_output) = self.run_npm(&["outdated", "-g", "--json"]).await {
            let _ = Self::apply_outdated_data(&mut packages, &outdated_output);
        }

        Ok(packages)
    }

    async fn install_packages(
        &self,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        if names.is_empty() {
            return Err("No packages specified for installation".to_string());
        }

        let mut args: Vec<String> = vec!["install".to_string(), "-g".to_string()];

        // 解析选项
        if let Some(opts) = options {
            if let Some(registry) = opts.get("registry") {
                args.push("--registry".to_string());
                args.push(registry.clone());
            }
        }

        // 检查是否指定了版本
        let version = options.and_then(|opts| opts.get("version"));

        // 添加所有包名
        for name in names {
            if let Some(ver) = version {
                args.push(format!("{}@{}", name, ver));
            } else {
                args.push(name.to_string());
            }
        }

        let args_ref: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
        let output = run_command("npm", &args_ref, "npm").await;

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
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        if names.is_empty() {
            return Err("No packages specified for uninstallation".to_string());
        }

        let mut args: Vec<String> = vec!["uninstall".to_string(), "-g".to_string()];

        // 添加所有包名
        for name in names {
            args.push(name.to_string());
        }

        let args_ref: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
        let output = run_command("npm", &args_ref, "npm").await;

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

        let mut args: Vec<String> = vec!["install".to_string(), "-g".to_string()];

        // 解析选项
        if let Some(opts) = options {
            if let Some(registry) = opts.get("registry") {
                args.push("--registry".to_string());
                args.push(registry.clone());
            }
        }

        // 为每个包添加 @latest 标记
        for name in names {
            args.push(format!("{}@latest", name));
        }

        let output = run_command("npm", &args.iter().map(String::as_str).collect::<Vec<_>>(), "npm").await;

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
        // npm search <keyword> --json
        let search_output = self.run_npm(&["search", keyword, "--json"]).await?;
        Self::parse_search_output(&search_output)
    }

    async fn get_package_versions(&self, name: &str) -> Result<Vec<String>, String> {
        // npm view <package> versions --json
        let output = self.run_npm(&["view", name, "versions", "--json"]).await?;
        let versions: Vec<String> = serde_json::from_str(&output)
            .map_err(|e| format!("Failed to parse npm versions output: {}", e))?;
        Ok(versions)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_adapter_info() {
        let adapter = NpmAdapter::new();

        assert_eq!(adapter.id(), "npm");
        assert_eq!(adapter.name(), "npm (global)");

        let caps = adapter.capabilities();
        assert!(caps.search);
        assert!(caps.list);
        assert!(caps.install);
        assert!(caps.uninstall);
        assert!(caps.update);
    }

    #[test]
    fn test_parse_list_output() {
        let output = r#"{
          "dependencies": {
            "pnpm": { "version": "9.0.0" },
            "typescript": { "version": "5.8.2" }
          }
        }"#;

        let packages = NpmAdapter::parse_list_output(output).unwrap();

        assert_eq!(packages.len(), 2);
        assert!(packages.iter().any(|pkg| pkg.name == "pnpm" && pkg.version == "9.0.0"));
        assert!(packages.iter().all(|pkg| pkg.manager == ManagerType::Npm));
    }

    #[test]
    fn test_apply_outdated_data() {
        let mut packages = vec![Package {
            name: "pnpm".to_string(),
            fullname: Some("pnpm".to_string()),
            version: "8.0.0".to_string(),
            latest_version: "8.0.0".to_string(),
            manager: ManagerType::Npm,
            installed: true,
            outdated: false,
            is_gui: false,
            description: None,
        }];

        let output = r#"{
          "pnpm": {
            "current": "8.0.0",
            "latest": "9.0.0"
          }
        }"#;

        NpmAdapter::apply_outdated_data(&mut packages, output).unwrap();

        assert_eq!(packages[0].latest_version, "9.0.0");
        assert!(packages[0].outdated);
    }

    #[test]
    fn test_parse_search_output() {
        let output = r#"[
          {
            "name": "pnpm",
            "version": "9.0.0",
            "description": "Fast package manager"
          }
        ]"#;

        let packages = NpmAdapter::parse_search_output(output).unwrap();

        assert_eq!(packages.len(), 1);
        assert_eq!(packages[0].name, "pnpm");
        assert_eq!(packages[0].version, "9.0.0");
        assert_eq!(packages[0].manager, ManagerType::Npm);
        assert!(!packages[0].installed);
    }
}
