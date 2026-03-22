use crate::adapters::{ensure_command_healthy, ensure_command_in_path, run_command, PackageAdapter};
use crate::models::{ActionResult, ManagerCapabilities, ManagerType, Package};
use async_trait::async_trait;
use serde::Deserialize;
use std::collections::HashMap;

#[derive(Debug, Deserialize)]
struct PipPackage {
    name: String,
    version: String,
    #[serde(default)]
    latest_version: Option<String>,
}

/// pip 适配器
pub struct PipAdapter;

impl PipAdapter {
    pub fn new() -> Self {
        Self
    }

    /// 获取可用的 pip 命令（pip3 或 pip）
    async fn get_pip_command(&self) -> Result<String, String> {
        if ensure_command_in_path("pip3", "").await.is_ok() {
            return Ok("pip3".to_string());
        }

        if ensure_command_in_path("pip", "").await.is_ok() {
            return Ok("pip".to_string());
        }

        Err("pip is not installed or not in PATH".to_string())
    }

    /// 执行 pip 命令并获取输出
    async fn run_pip(&self, args: &[&str]) -> Result<String, String> {
        let cmd = self.get_pip_command().await?;
        run_command(&cmd, args, &cmd).await
    }

    fn parse_list_output(list_output: &str) -> Result<Vec<Package>, String> {
        let parsed: Vec<PipPackage> = serde_json::from_str(list_output)
            .map_err(|e| format!("Failed to parse pip list output: {}", e))?;

        let packages = parsed
            .into_iter()
            .map(|pkg| Package {
                name: pkg.name.clone(),
                fullname: Some(pkg.name),
                version: pkg.version.clone(),
                latest_version: pkg.version,
                manager: ManagerType::Pip,
                installed: true,
                outdated: false,
                is_gui: false,
                description: None,
            })
            .collect();

        Ok(packages)
    }

    fn apply_outdated_data(packages: &mut [Package], outdated_output: &str) -> Result<(), String> {
        if outdated_output.trim().is_empty() {
            return Ok(());
        }

        let parsed: Vec<PipPackage> = serde_json::from_str(outdated_output)
            .map_err(|e| format!("Failed to parse pip outdated output: {}", e))?;
        let outdated_map: HashMap<String, String> = parsed
            .into_iter()
            .filter_map(|pkg| pkg.latest_version.map(|latest| (pkg.name, latest)))
            .collect();

        for package in packages.iter_mut() {
            if let Some(latest) = outdated_map.get(&package.name) {
                package.latest_version = latest.clone();
                package.outdated = &package.version != latest;
            }
        }

        Ok(())
    }

    fn parse_search_output(search_output: &str) -> Vec<Package> {
        search_output
            .lines()
            .filter_map(|line| {
                let line = line.trim();
                if line.is_empty() {
                    return None;
                }

                let (name, description) = match line.split_once(" - ") {
                    Some((left, right)) => (left.trim().to_string(), Some(right.trim().to_string())),
                    None => (line.to_string(), None),
                };

                let (name, version) = match name.rsplit_once('(') {
                    Some((pkg_name, version_part)) if version_part.ends_with(')') => (
                        pkg_name.trim().to_string(),
                        version_part.trim_end_matches(')').trim().to_string(),
                    ),
                    _ => (name, "latest".to_string()),
                };

                Some(Package {
                    name: name.clone(),
                    fullname: Some(name),
                    version: version.clone(),
                    latest_version: version,
                    manager: ManagerType::Pip,
                    installed: false,
                    outdated: false,
                    is_gui: false,
                    description,
                })
            })
            .collect()
    }
}

#[async_trait]
impl PackageAdapter for PipAdapter {
    fn id(&self) -> &str {
        "pip"
    }

    fn name(&self) -> &str {
        "pip (global)"
    }

    fn capabilities(&self) -> ManagerCapabilities {
        ManagerCapabilities {
            search: false,
            ..ManagerCapabilities::default()
        }
    }

    async fn preflight(&self) -> Result<(), String> {
        let cmd = self.get_pip_command().await?;
        ensure_command_healthy(&cmd, &["--version"], &cmd).await
    }

    async fn list_packages(&self) -> Result<Vec<Package>, String> {
        let list_output = self.run_pip(&["list", "--format=json"]).await?;
        let mut packages = Self::parse_list_output(&list_output)?;

        if let Ok(outdated_output) = self.run_pip(&["list", "--outdated", "--format=json"]).await {
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

        let mut args: Vec<String> = vec!["install".to_string()];

        // 解析选项
        if let Some(opts) = options {
            if opts.get("user").map_or(false, |v| v == "true") {
                args.push("--user".to_string());
            }
            if let Some(index_url) = opts.get("index_url") {
                args.push("--index-url".to_string());
                args.push(index_url.clone());
            }
        }

        // 检查是否指定了版本
        let version = options.and_then(|opts| opts.get("version"));

        // 添加所有包名
        for name in names {
            if let Some(ver) = version {
                args.push(format!("{}=={}", name, ver));
            } else {
                args.push(name.to_string());
            }
        }

        let cmd = self.get_pip_command().await?;
        let args_ref: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
        let output = run_command(&cmd, &args_ref, &cmd).await;

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

        let mut args = vec!["uninstall", "-y"];

        // 解析选项
        if let Some(opts) = options {
            if opts.get("user").map_or(false, |v| v == "true") {
                args.push("--user");
            }
        }

        // 添加所有包名
        for name in names {
            args.push(name);
        }

        let cmd = self.get_pip_command().await?;
        let output = run_command(&cmd, &args, &cmd).await;

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

        let mut args = vec!["install", "--upgrade"];

        // 解析选项
        if let Some(opts) = options {
            if opts.get("user").map_or(false, |v| v == "true") {
                args.push("--user");
            }
            if let Some(index_url) = opts.get("index_url") {
                args.push("--index-url");
                args.push(index_url);
            }
        }

        // 添加所有包名
        for name in names {
            args.push(name);
        }

        let cmd = self.get_pip_command().await?;
        let output = run_command(&cmd, &args, &cmd).await;

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
        // `pip search` 在很多环境中已不可用，这里尽量兼容旧环境，
        // 若命令失败则返回空结果，让调用方基于 capability 决定 UI 呈现。
        match self.run_pip(&["search", keyword]).await {
            Ok(output) => Ok(Self::parse_search_output(&output)),
            Err(_) => Ok(Vec::new()),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_adapter_info() {
        let adapter = PipAdapter::new();

        assert_eq!(adapter.id(), "pip");
        assert_eq!(adapter.name(), "pip (global)");

        let caps = adapter.capabilities();
        assert!(!caps.search);
        assert!(caps.list);
        assert!(caps.install);
        assert!(caps.uninstall);
        assert!(caps.update);
    }

    #[test]
    fn test_parse_list_output() {
        let output = r#"[
          {"name":"requests","version":"2.31.0"},
          {"name":"pytest","version":"8.3.0"}
        ]"#;

        let packages = PipAdapter::parse_list_output(output).unwrap();

        assert_eq!(packages.len(), 2);
        assert!(packages.iter().all(|pkg| pkg.manager == ManagerType::Pip));
        assert!(packages.iter().any(|pkg| pkg.name == "requests" && pkg.version == "2.31.0"));
    }

    #[test]
    fn test_apply_outdated_data() {
        let mut packages = vec![Package {
            name: "requests".to_string(),
            fullname: Some("requests".to_string()),
            version: "2.31.0".to_string(),
            latest_version: "2.31.0".to_string(),
            manager: ManagerType::Pip,
            installed: true,
            outdated: false,
            is_gui: false,
            description: None,
        }];

        let output = r#"[
          {"name":"requests","version":"2.31.0","latest_version":"2.32.0"}
        ]"#;

        PipAdapter::apply_outdated_data(&mut packages, output).unwrap();

        assert_eq!(packages[0].latest_version, "2.32.0");
        assert!(packages[0].outdated);
    }

    #[test]
    fn test_parse_search_output() {
        let output = "requests (2.32.0) - Python HTTP for Humans.\npytest (8.3.0) - testing framework";
        let packages = PipAdapter::parse_search_output(output);

        assert_eq!(packages.len(), 2);
        assert_eq!(packages[0].name, "requests");
        assert_eq!(packages[0].version, "2.32.0");
        assert_eq!(packages[0].manager, ManagerType::Pip);
        assert!(!packages[0].installed);
    }
}
