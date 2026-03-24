use crate::adapters::{brew::BrewAdapter, cargo::CargoAdapter, npm::NpmAdapter, pip::PipAdapter, PackageAdapter};
use crate::models::{ActionResult, ManagerInfo, Package, PackageTarget};
use std::collections::HashMap;
use std::sync::Arc;

/// 包管理器注册表
pub struct ManagerRegistry {
    adapters: HashMap<String, Arc<dyn PackageAdapter>>,
}

impl ManagerRegistry {
    /// 创建新的注册表并初始化所有适配器（通过预检的才会被注册）
    pub async fn new() -> Self {
        let mut adapters: HashMap<String, Arc<dyn PackageAdapter>> = HashMap::new();

        // 尝试注册各适配器，只注册预检通过的
        Self::try_register_adapter(&mut adapters, "brew", Arc::new(BrewAdapter::new())).await;
        Self::try_register_adapter(&mut adapters, "npm", Arc::new(NpmAdapter::new())).await;
        Self::try_register_adapter(&mut adapters, "pip", Arc::new(PipAdapter::new())).await;
        Self::try_register_adapter(&mut adapters, "cargo", Arc::new(CargoAdapter::new())).await;

        Self { adapters }
    }

    /// 尝试注册适配器，预检失败则跳过
    async fn try_register_adapter(
        adapters: &mut HashMap<String, Arc<dyn PackageAdapter>>,
        id: &str,
        adapter: Arc<dyn PackageAdapter>,
    ) {
        match adapter.preflight().await {
            Ok(()) => {
                adapters.insert(id.to_string(), adapter);
                tracing::info!("Registered adapter: {}", id);
            }
            Err(e) => {
                tracing::warn!("Skipping adapter '{}': {}", id, e);
            }
        }
    }

    /// 获取所有包管理器信息
    pub async fn list_managers(&self) -> Result<Vec<ManagerInfo>, String> {
        let mut infos = Vec::new();

        for adapter in self.adapters.values() {
            let info = ManagerInfo {
                id: adapter.id().to_string(),
                name: adapter.name().to_string(),
                capabilities: adapter.capabilities(),
                enabled: true, // TODO: 从配置读取
            };
            infos.push(info);
        }

        Ok(infos)
    }

    /// 列出所有已安装包（并发获取）
    pub async fn list_installed_packages(&self) -> Result<Vec<Package>, String> {
        let mut all_packages = Vec::new();
        let mut tasks = Vec::new();

        // 为每个适配器创建并发任务
        for (manager_id, adapter) in &self.adapters {
            let adapter = Arc::clone(adapter);
            let manager_id = manager_id.clone();
            let task = tokio::spawn(async move {
                adapter.list_packages().await.map(|packages| (manager_id, packages))
            });
            tasks.push(task);
        }

        // 等待所有任务完成
        for task in tasks {
            match task.await {
                Ok(Ok((manager_id, packages))) => {
                    // 标记每个包的所属管理器
                    tracing::debug!("[{}] Found {} packages", manager_id, packages.len());
                    all_packages.extend(packages);
                }
                Ok(Err(e)) => {
                    tracing::warn!("Failed to list packages: {}", e);
                    // 继续处理其他适配器，不中断整个流程
                }
                Err(e) => {
                    tracing::error!("Task panicked: {}", e);
                }
            }
        }

        Ok(all_packages)
    }

    /// 安装包（单个）
    pub async fn install_package(
        &self,
        manager_id: &str,
        name: &str,
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        self.install_packages(manager_id, &[name], options).await
    }

    /// 安装包（多个）
    pub async fn install_packages(
        &self,
        manager_id: &str,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        let adapter = self.get_adapter(manager_id)?;
        adapter.install_packages(names, options).await
    }

    /// 卸载包（单个）
    pub async fn uninstall_package(
        &self,
        manager_id: &str,
        name: &str,
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        self.uninstall_packages(manager_id, &[name], options).await
    }

    /// 卸载包（多个）
    pub async fn uninstall_packages(
        &self,
        manager_id: &str,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        let adapter = self.get_adapter(manager_id)?;
        adapter.uninstall_packages(names, options).await
    }

    /// 升级包（单个）
    pub async fn upgrade_package(
        &self,
        manager_id: &str,
        name: &str,
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        self.upgrade_packages(manager_id, &[name], options).await
    }

    /// 升级包（多个）
    pub async fn upgrade_packages(
        &self,
        manager_id: &str,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        let adapter = self.get_adapter(manager_id)?;
        adapter.upgrade_packages(names, options).await
    }

    pub async fn batch_uninstall_packages(
        &self,
        packages: &[PackageTarget],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        self.run_grouped_package_action(packages, options, |registry, manager_id, names, options| async move {
            registry.uninstall_packages(manager_id, &names, options).await
        })
        .await
    }

    pub async fn batch_upgrade_packages(
        &self,
        packages: &[PackageTarget],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        self.run_grouped_package_action(packages, options, |registry, manager_id, names, options| async move {
            registry.upgrade_packages(manager_id, &names, options).await
        })
        .await
    }

    /// 搜索包（并发搜索）
    pub async fn search_packages(&self, keyword: &str) -> Result<Vec<Package>, String> {
        let mut all_results = Vec::new();
        let mut tasks = Vec::new();

        for adapter in self.adapters.values() {
            let adapter = Arc::clone(adapter);
            let keyword = keyword.to_string();
            let task = tokio::spawn(async move {
                adapter.search_packages(&keyword).await
            });
            tasks.push(task);
        }

        for task in tasks {
            match task.await {
                Ok(Ok(packages)) => {
                    all_results.extend(packages);
                }
                Ok(Err(e)) => {
                    tracing::warn!("Failed to search packages: {}", e);
                }
                Err(e) => {
                    tracing::error!("Task panicked: {}", e);
                }
            }
        }

        Ok(all_results)
    }

    /// 获取包的可用版本列表
    pub async fn get_package_versions(&self, manager_id: &str, name: &str) -> Result<Vec<String>, String> {
        let adapter = self.get_adapter(manager_id)?;
        adapter.get_package_versions(name).await
    }

    /// 获取指定管理器的适配器
    fn get_adapter(&self, manager_id: &str) -> Result<Arc<dyn PackageAdapter>, String> {
        self.adapters
            .get(manager_id)
            .cloned()
            .ok_or_else(|| format!("Package manager '{}' not found", manager_id))
    }

    async fn run_grouped_package_action<'a, F, Fut>(
        &'a self,
        packages: &'a [PackageTarget],
        options: Option<&'a HashMap<String, String>>,
        action: F,
    ) -> Result<ActionResult, String>
    where
        F: Fn(&'a Self, &'a str, Vec<&'a str>, Option<&'a HashMap<String, String>>) -> Fut,
        Fut: std::future::Future<Output = Result<ActionResult, String>>,
    {
        if packages.is_empty() {
            return Ok(ActionResult::success("No packages selected"));
        }

        let mut grouped: HashMap<&str, Vec<&str>> = HashMap::new();
        for package in packages {
            grouped
                .entry(package.manager.as_str())
                .or_default()
                .push(package.name.as_str());
        }

        let mut total_processed = 0usize;
        let mut summaries = Vec::new();

        for (manager_id, names) in grouped {
            let count = names.len();
            let result = action(self, manager_id, names, options).await?;
            total_processed += count;
            summaries.push(format!("{}: {}", manager_id, result.message));
        }

        Ok(ActionResult::success(format!(
            "Processed {} packages. {}",
            total_processed,
            summaries.join(" ")
        )))
    }
}
