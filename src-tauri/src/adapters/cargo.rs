use crate::adapters::{ensure_command_healthy, ensure_command_in_path, run_command, PackageAdapter};
use crate::models::{ActionResult, ManagerCapabilities, ManagerType, Package};
use async_trait::async_trait;
use std::collections::HashMap;

/// Cargo 适配器
pub struct CargoAdapter;

impl CargoAdapter {
    pub fn new() -> Self {
        Self
    }

    /// 执行 cargo 命令并获取输出
    async fn run_cargo(&self, args: &[&str]) -> Result<String, String> {
        run_command("cargo", args, "cargo").await
    }

    fn parse_install_list_output(output: &str) -> Vec<Package> {
        output
            .lines()
            .filter_map(|line| {
                let line = line.trim();
                if line.is_empty() || line.starts_with(' ') {
                    return None;
                }

                let (name, version) = line.split_once(" v")?;
                let version = version.trim_end_matches(':').trim().to_string();
                let name = name.trim().to_string();

                Some(Package {
                    name: name.clone(),
                    fullname: Some(name),
                    version: version.clone(),
                    latest_version: version,
                    manager: ManagerType::Cargo,
                    installed: true,
                    outdated: false,
                    is_gui: false,
                    description: None,
                })
            })
            .collect()
    }

    fn parse_search_output(output: &str) -> Vec<Package> {
        output
            .lines()
            .filter_map(|line| {
                let line = line.trim();
                if line.is_empty() {
                    return None;
                }

                let (left, description) = match line.split_once('#') {
                    Some((left, right)) => (left.trim(), Some(right.trim().to_string())),
                    None => (line, None),
                };

                let (name, version) = left.split_once('=')?;
                let name = name.trim().to_string();
                let version = version.trim().trim_matches('"').to_string();

                Some(Package {
                    name: name.clone(),
                    fullname: Some(name),
                    version: version.clone(),
                    latest_version: version,
                    manager: ManagerType::Cargo,
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
impl PackageAdapter for CargoAdapter {
    fn id(&self) -> &str {
        "cargo"
    }

    fn name(&self) -> &str {
        "Cargo"
    }

    fn capabilities(&self) -> ManagerCapabilities {
        ManagerCapabilities::default()
    }

    async fn preflight(&self) -> Result<(), String> {
        ensure_command_in_path(
            "cargo",
            "cargo is not installed or not in PATH. Please install Rust from https://rustup.rs/",
        )
        .await?;

        ensure_command_healthy("cargo", &["--version"], "cargo").await
    }

    async fn list_packages(&self) -> Result<Vec<Package>, String> {
        let output = self.run_cargo(&["install", "--list"]).await?;
        Ok(Self::parse_install_list_output(&output))
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

        // 检查是否指定了版本
        let version = options.and_then(|opts| opts.get("version"));

        // 添加所有包名
        for name in names {
            args.push(name.to_string());
        }

        // 如果指定了版本，添加 --version 参数
        if let Some(ver) = version {
            args.push("--version".to_string());
            args.push(ver.clone());
        }

        let args_ref: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
        let output = run_command("cargo", &args_ref, "cargo").await;

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

        let mut args = vec!["uninstall"];
        args.extend_from_slice(names);

        let output = run_command("cargo", &args, "cargo").await;

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
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        if names.is_empty() {
            return Err("No packages specified for upgrade".to_string());
        }

        let mut args = vec!["install", "--force"];

        // 添加所有包名
        for name in names {
            args.push(name);
        }

        let output = run_command("cargo", &args, "cargo").await;

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
        let output = self.run_cargo(&["search", keyword, "--limit", "10"]).await?;
        Ok(Self::parse_search_output(&output))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_adapter_info() {
        let adapter = CargoAdapter::new();

        assert_eq!(adapter.id(), "cargo");
        assert_eq!(adapter.name(), "Cargo");

        let caps = adapter.capabilities();
        assert!(caps.search);
        assert!(caps.list);
        assert!(caps.install);
        assert!(caps.uninstall);
        assert!(caps.update);
    }

    #[test]
    fn test_parse_install_list_output() {
        let output = r#"bat v0.24.0:
    bat
cargo-edit v0.13.0:
    cargo-add
    cargo-rm
"#;

        let packages = CargoAdapter::parse_install_list_output(output);

        assert_eq!(packages.len(), 2);
        assert!(packages.iter().any(|pkg| pkg.name == "bat" && pkg.version == "0.24.0"));
        assert!(packages.iter().all(|pkg| pkg.manager == ManagerType::Cargo));
    }

    #[test]
    fn test_parse_search_output() {
        let output = r#"ripgrep = "14.1.1" # recursively searches directories for a regex pattern
bat = "0.24.0" # a cat clone with wings"#;

        let packages = CargoAdapter::parse_search_output(output);

        assert_eq!(packages.len(), 2);
        assert_eq!(packages[0].name, "ripgrep");
        assert_eq!(packages[0].version, "14.1.1");
        assert_eq!(packages[0].manager, ManagerType::Cargo);
        assert!(!packages[0].installed);
    }
}
