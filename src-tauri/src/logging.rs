use std::path::PathBuf;

/// 日志管理器
pub struct LogManager {
    log_dir: PathBuf,
}

impl LogManager {
    /// 创建新的日志管理器
    pub fn new() -> Result<Self, String> {
        // 获取用户数据目录
        let log_dir = dirs::data_local_dir()
            .ok_or("Failed to get local data directory")?
            .join("unipac")
            .join("logs");

        // 确保日志目录存在
        std::fs::create_dir_all(&log_dir)
            .map_err(|e| format!("Failed to create log directory: {}", e))?;

        Ok(Self { log_dir })
    }

    /// 初始化日志系统
    pub fn init(&self) -> Result<(), String> {
        use tracing_subscriber::{fmt, layer::SubscriberExt, util::SubscriberInitExt};

        let _log_file = self.log_dir.join("unipac.log");

        // 文件日志层
        let file_appender = tracing_appender::rolling::daily(&self.log_dir, "unipac.log");
        let (file_writer, _guard) = tracing_appender::non_blocking(file_appender);

        // 控制台日志层
        let console_layer = fmt::layer()
            .with_target(false)
            .with_level(true)
            .with_thread_ids(false)
            .with_file(false)
            .with_line_number(false);

        // 组合日志层
        tracing_subscriber::registry()
            .with(console_layer)
            .with(fmt::layer().with_writer(file_writer))
            .init();

        Ok(())
    }

    /// 记录 CLI 命令执行
    #[allow(dead_code)]
    pub fn log_command(&self, command: &str, args: &[&str], output: &str) {
        let cmd_str = if args.is_empty() {
            command.to_string()
        } else {
            format!("{} {}", command, args.join(" "))
        };
        tracing::info!("[EXEC] {} -> {}", cmd_str, output.trim());
    }

    /// 获取日志目录
    #[allow(dead_code)]
    pub fn log_dir(&self) -> &PathBuf {
        &self.log_dir
    }

    /// 读取日志内容
    #[allow(dead_code)]
    pub fn read_logs(&self, lines: Option<usize>) -> Result<String, String> {
        let log_file = self.log_dir.join("unipac.log");

        if !log_file.exists() {
            return Ok(String::new());
        }

        let content = std::fs::read_to_string(&log_file)
            .map_err(|e| format!("Failed to read log file: {}", e))?;

        if let Some(n) = lines {
            let lines_vec: Vec<&str> = content.lines().collect();
            let start = if lines_vec.len() > n { lines_vec.len() - n } else { 0 };
            Ok(lines_vec[start..].join("\n"))
        } else {
            Ok(content)
        }
    }
}

impl Default for LogManager {
    fn default() -> Self {
        Self::new().expect("Failed to create LogManager")
    }
}