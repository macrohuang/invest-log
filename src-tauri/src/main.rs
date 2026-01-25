// Prevents additional console window on Windows in release
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::fs;
use std::path::PathBuf;
use std::process::{Command, Child, Stdio};
use std::sync::Mutex;

use serde::{Deserialize, Serialize};
use tauri::{Manager, State};
use directories::ProjectDirs;

/// Application configuration stored in the user's config directory
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
struct AppConfig {
    #[serde(default)]
    setup_complete: bool,
    #[serde(default)]
    use_icloud: bool,
    #[serde(default)]
    data_dir: Option<String>,
    #[serde(default = "default_db_name")]
    db_name: String,
}

fn default_db_name() -> String {
    "transactions.db".to_string()
}

/// State for managing the Python sidecar process
struct SidecarState {
    child: Mutex<Option<Child>>,
}

/// Get the application config directory path
fn get_config_dir() -> PathBuf {
    if let Some(proj_dirs) = ProjectDirs::from("com", "investlog", "InvestLog") {
        proj_dirs.config_dir().to_path_buf()
    } else {
        dirs::home_dir()
            .map(|h| h.join(".investlog"))
            .unwrap_or_else(|| PathBuf::from(".investlog"))
    }
}

/// Get the config file path
fn get_config_path() -> PathBuf {
    get_config_dir().join("config.json")
}

/// Load application configuration
fn load_config() -> AppConfig {
    let config_path = get_config_path();
    if config_path.exists() {
        if let Ok(content) = fs::read_to_string(&config_path) {
            if let Ok(config) = serde_json::from_str(&content) {
                return config;
            }
        }
    }
    AppConfig::default()
}

/// Save application configuration
fn save_config(config: &AppConfig) -> Result<(), String> {
    let config_dir = get_config_dir();
    fs::create_dir_all(&config_dir).map_err(|e| e.to_string())?;
    
    let config_path = get_config_path();
    let content = serde_json::to_string_pretty(config).map_err(|e| e.to_string())?;
    fs::write(&config_path, content).map_err(|e| e.to_string())?;
    
    Ok(())
}

/// Check if iCloud Drive is available (macOS only)
#[cfg(target_os = "macos")]
fn is_icloud_available() -> bool {
    let icloud_path = dirs::home_dir()
        .map(|h| h.join("Library/Mobile Documents/com~apple~CloudDocs"))
        .unwrap_or_default();
    icloud_path.exists()
}

#[cfg(not(target_os = "macos"))]
fn is_icloud_available() -> bool {
    false
}

/// Get the iCloud app folder path
#[cfg(target_os = "macos")]
fn get_icloud_app_folder() -> Option<PathBuf> {
    dirs::home_dir()
        .map(|h| h.join("Library/Mobile Documents/com~apple~CloudDocs/InvestLog"))
}

#[cfg(not(target_os = "macos"))]
fn get_icloud_app_folder() -> Option<PathBuf> {
    None
}

/// Get the data directory based on configuration
fn get_data_dir(config: &AppConfig) -> PathBuf {
    if let Some(ref data_dir) = config.data_dir {
        PathBuf::from(data_dir)
    } else if config.use_icloud {
        get_icloud_app_folder().unwrap_or_else(get_config_dir)
    } else {
        get_config_dir()
    }
}

/// Get the sidecar binary path
fn get_sidecar_path() -> PathBuf {
    let exe_path = std::env::current_exe().expect("Failed to get executable path");
    let exe_dir = exe_path.parent().expect("Failed to get executable directory");
    
    #[cfg(target_os = "windows")]
    let sidecar_name = "invest-log-backend.exe";
    
    #[cfg(not(target_os = "windows"))]
    let sidecar_name = "invest-log-backend";
    
    exe_dir.join(sidecar_name)
}

/// Tauri command: Check if this is the first run
#[tauri::command]
fn is_first_run() -> bool {
    let config = load_config();
    !config.setup_complete
}

/// Tauri command: Get setup status and platform info
#[tauri::command]
fn get_setup_info() -> serde_json::Value {
    let config = load_config();
    serde_json::json!({
        "setup_complete": config.setup_complete,
        "is_macos": cfg!(target_os = "macos"),
        "is_windows": cfg!(target_os = "windows"),
        "icloud_available": is_icloud_available(),
        "icloud_path": get_icloud_app_folder().map(|p| p.to_string_lossy().to_string()),
        "default_path": get_config_dir().to_string_lossy().to_string(),
        "current_data_dir": get_data_dir(&config).to_string_lossy().to_string(),
    })
}

/// Tauri command: Complete setup with the selected storage option
#[tauri::command]
fn complete_setup(use_icloud: bool, custom_path: Option<String>) -> Result<String, String> {
    let mut config = load_config();
    
    let data_dir = if use_icloud && is_icloud_available() {
        config.use_icloud = true;
        config.data_dir = None;
        get_icloud_app_folder().ok_or("iCloud not available")?
    } else if let Some(path) = custom_path {
        config.use_icloud = false;
        config.data_dir = Some(path.clone());
        PathBuf::from(path)
    } else {
        config.use_icloud = false;
        config.data_dir = None;
        get_config_dir()
    };
    
    fs::create_dir_all(&data_dir).map_err(|e| e.to_string())?;
    
    config.setup_complete = true;
    save_config(&config)?;
    
    Ok(data_dir.to_string_lossy().to_string())
}

/// Tauri command: Open folder picker dialog
#[tauri::command]
async fn pick_folder(app: tauri::AppHandle) -> Result<Option<String>, String> {
    use tauri_plugin_dialog::DialogExt;
    
    let folder = app.dialog()
        .file()
        .set_title("Select Data Folder")
        .blocking_pick_folder();
    
    Ok(folder.map(|p| p.to_string()))
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_fs::init())
        .manage(SidecarState {
            child: Mutex::new(None),
        })
        .setup(|app| {
            let config = load_config();
            let data_dir = get_data_dir(&config);
            
            // Ensure data directory exists
            let _ = fs::create_dir_all(&data_dir);
            
            // Get sidecar path
            let sidecar_path = get_sidecar_path();
            println!("[Tauri] Sidecar path: {:?}", sidecar_path);
            
            if !sidecar_path.exists() {
                eprintln!("[Tauri] ERROR: Sidecar not found at: {:?}", sidecar_path);
                return Err(format!("Backend not found at: {:?}", sidecar_path).into());
            }
            
            // Start the Python sidecar
            let child = Command::new(&sidecar_path)
                .args(["--data-dir", &data_dir.to_string_lossy(), "--port", "8000"])
                .stdout(Stdio::piped())
                .stderr(Stdio::piped())
                .spawn()
                .map_err(|e| format!("Failed to start backend: {}", e))?;
            
            println!("[Tauri] Started backend with PID: {}", child.id());
            
            // Store the child process handle
            {
                let state: State<SidecarState> = app.state();
                *state.child.lock().unwrap() = Some(child);
            }
            
            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { .. } = event {
                // Kill the sidecar when the window is closed
                let state: State<SidecarState> = window.state();
                let mut child_guard = state.child.lock().unwrap();
                if let Some(ref mut child) = *child_guard {
                    println!("[Tauri] Stopping backend...");
                    let _ = child.kill();
                    let _ = child.wait();
                }
            }
        })
        .invoke_handler(tauri::generate_handler![
            is_first_run,
            get_setup_info,
            complete_setup,
            pick_folder,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
