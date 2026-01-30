// Prevents additional console window on Windows in release
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::fs;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::Mutex;
use std::thread;
use std::time::Duration;

use serde::{Deserialize, Serialize};
use tauri::{Manager, RunEvent, State, Url, Webview};
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
    #[cfg(unix)]
    pgid: Mutex<Option<i32>>,
    port: Mutex<Option<u16>>,
}

fn loading_html(port: Option<u16>) -> String {
    let port_js = port.map(|p| p.to_string()).unwrap_or_else(|| "null".to_string());
    format!(
        r#"<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Invest Log</title><style>:root{{color-scheme:light}}html,body{{height:100%;margin:0;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica,Arial,sans-serif;background:#f8fafc;color:#0f172a}}body{{display:flex;align-items:center;justify-content:center}}.wrap{{display:flex;flex-direction:column;align-items:center;gap:12px;text-align:center;padding:24px 32px}}.spinner{{width:44px;height:44px;border-radius:50%;border:4px solid #e2e8f0;border-top-color:#2563eb;animation:spin 1s linear infinite}}.title{{font-size:20px;font-weight:700;letter-spacing:.3px}}.status{{font-size:14px;font-weight:600;color:#1e293b}}.detail{{font-size:12px;color:#64748b}}@keyframes spin{{to{{transform:rotate(360deg)}}}}@media (prefers-reduced-motion: reduce){{.spinner{{animation:none}}}}</style></head><body><div class="wrap"><div class="spinner"></div><div class="title">Invest Log</div><div id="status" class="status">系统初始化中…</div><div id="detail" class="detail">正在准备环境</div></div><script>(function(){{const statusEl=document.getElementById("status");const detailEl=document.getElementById("detail");const startAt=Date.now();let port=null;let attempts=0;let stopped=false;function setPort(value){{const parsed=Number(value);if(!Number.isFinite(parsed))return;port=parsed;attempts=0;detailEl.textContent="正在启动后台服务";if(!stopped)ping();}}function markTimeout(){{statusEl.textContent="启动超时";detailEl.textContent="请检查数据目录中的日志后重试";stopped=true;}}async function ping(){{if(!port||stopped)return;const url=`http://127.0.0.1:${{port}}/api/health`;try{{await fetch(url,{{mode:"no-cors",cache:"no-store"}});const target=`http://127.0.0.1:${{port}}/?t=${{Date.now()}}`;window.location.replace(target);return;}}catch(e){{attempts+=1;if(attempts%10===0){{const seconds=Math.floor((Date.now()-startAt)/1000);detailEl.textContent=`已等待 ${{seconds}}s，仍在启动…`;}}if(attempts>120){{markTimeout();return;}}setTimeout(ping,500);}}}}window.__INVEST_LOG_SET_PORT__=setPort;window.__INVEST_LOG_PORT__={port_js};if(window.__INVEST_LOG_PORT__!==null){{setPort(window.__INVEST_LOG_PORT__);}}}})();</script></body></html>"#,
        port_js = port_js
    )
}

fn show_loading_window(window: &tauri::WebviewWindow, port: Option<u16>) {
    let html = loading_html(port);
    if let Ok(html_json) = serde_json::to_string(&html) {
        let js = format!("document.open();document.write({});document.close();", html_json);
        let _ = window.eval(js);
    }
}

fn show_loading_webview(webview: &Webview, port: Option<u16>) {
    let html = loading_html(port);
    if let Ok(html_json) = serde_json::to_string(&html) {
        let js = format!("document.open();document.write({});document.close();", html_json);
        let _ = webview.eval(js);
    }
}

fn notify_loader_port(window: &tauri::WebviewWindow, port: u16) {
    let js = format!(
        "window.__INVEST_LOG_PORT__ = {0}; window.__INVEST_LOG_SET_PORT__ && window.__INVEST_LOG_SET_PORT__({0});",
        port
    );
    let _ = window.eval(js);
}

fn stop_sidecar(state: &State<SidecarState>) {
    #[cfg(unix)]
    let pgid = state.pgid.lock().unwrap().take();
    let mut child_guard = state.child.lock().unwrap();
    if let Some(mut child) = child_guard.take() {
        println!("[Tauri] Stopping backend...");
        #[cfg(unix)]
        if let Some(pgid) = pgid {
            unsafe {
                let _ = libc::killpg(pgid, libc::SIGTERM);
            }
            for _ in 0..10 {
                let alive = unsafe { libc::killpg(pgid, 0) == 0 };
                if !alive {
                    break;
                }
                thread::sleep(Duration::from_millis(100));
            }
            let alive = unsafe { libc::killpg(pgid, 0) == 0 };
            if alive {
                unsafe {
                    let _ = libc::killpg(pgid, libc::SIGKILL);
                }
            }
        }
        let _ = child.kill();
        let _ = child.wait();
    }
}

fn pick_port() -> u16 {
    if std::net::TcpListener::bind("127.0.0.1:8000").is_ok() {
        return 8000;
    }
    std::net::TcpListener::bind("127.0.0.1:0")
        .and_then(|listener| listener.local_addr())
        .map(|addr| addr.port())
        .unwrap_or(8000)
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

/// Tauri command: Open file picker dialog for existing DB file
#[tauri::command]
async fn pick_db_file(app: tauri::AppHandle) -> Result<Option<String>, String> {
    use tauri_plugin_dialog::DialogExt;

    let file = app.dialog()
        .file()
        .add_filter("SQLite Database", &["db", "sqlite", "sqlite3"])
        .blocking_pick_file();

    Ok(file.map(|p| p.to_string()))
}

fn main() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_fs::init())
        .manage(SidecarState {
            child: Mutex::new(None),
            #[cfg(unix)]
            pgid: Mutex::new(None),
            port: Mutex::new(None),
        })
        .on_page_load(|webview, payload| {
            if webview.label() != "main" {
                return;
            }
            let scheme = payload.url().scheme();
            if scheme == "http" || scheme == "https" {
                return;
            }
            show_loading_webview(webview, None);
            let _ = webview.window().show();
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

            let data_dir_str = data_dir.to_string_lossy().to_string();
            let port = pick_port();
            let port_str = port.to_string();
            {
                let state: State<SidecarState> = app.state();
                *state.port.lock().unwrap() = Some(port);
            }

            let app_handle = app.handle().clone();
            let port_for_loader = port;
            let port_for_nav = port;
            let app_handle_nav = app.handle().clone();
            let app_handle_start = app.handle().clone();
            let sidecar_path_start = sidecar_path.clone();
            let data_dir_start = data_dir_str.clone();
            let port_str_start = port_str.clone();

            // Start the Python sidecar in a background thread to avoid blocking UI
            thread::spawn(move || {
                println!("[Tauri] Starting backend...");
                let mut cmd = Command::new(&sidecar_path_start);
                cmd.env("INVEST_LOG_DATA_DIR", &data_dir_start)
                    .env("INVEST_LOG_PARENT_WATCH", "1")
                    .args(["--data-dir", &data_dir_start, "--port", &port_str_start])
                    .stdout(Stdio::inherit())
                    .stderr(Stdio::inherit());

                let child = match cmd.spawn() {
                    Ok(child) => child,
                    Err(e) => {
                        if let Some(window) = app_handle_start.get_webview_window("main") {
                            let _ = window.eval(&format!(
                                "document.body.innerHTML = '<h2>后台启动失败</h2><p>{}</p>';",
                                e.to_string().replace('\'', "")
                            ));
                        }
                        return;
                    }
                };

                println!("[Tauri] Started backend with PID: {}", child.id());
                let child_pid = child.id() as i32;

                // Store the child process handle
                {
                    let state: State<SidecarState> = app_handle_start.state();
                    *state.child.lock().unwrap() = Some(child);
                    #[cfg(unix)]
                    {
                        unsafe {
                            let _ = libc::setpgid(child_pid, child_pid);
                        }
                        let pgid = unsafe { libc::getpgid(child_pid) };
                        if pgid == child_pid {
                            *state.pgid.lock().unwrap() = Some(child_pid);
                        }
                    }
                }

                // Watch backend process exit and surface error early
                let app_handle_exit = app_handle_start.clone();
                thread::spawn(move || loop {
                    let exited = {
                        let state: State<SidecarState> = app_handle_exit.state();
                        let mut guard = state.child.lock().unwrap();
                        if let Some(child) = guard.as_mut() {
                            match child.try_wait() {
                                Ok(Some(_status)) => true,
                                Ok(None) => false,
                                Err(_) => true,
                            }
                        } else {
                            false
                        }
                    };
                    if exited {
                        if let Some(window) = app_handle_exit.get_webview_window("main") {
                            let _ = window.eval(
                                "document.body.innerHTML = '<h2>后台启动失败</h2><p>请检查数据目录中的日志后重试。</p>';"
                            );
                        }
                        break;
                    }
                    thread::sleep(Duration::from_millis(500));
                });
            });

            // Notify loader page about the backend port as soon as the window exists
            thread::spawn(move || {
                for _ in 0..200 {
                    let window = app_handle
                        .get_webview_window("main")
                        .or_else(|| app_handle.webview_windows().into_iter().next().map(|(_, w)| w));
                    if let Some(window) = window {
                        show_loading_window(&window, Some(port_for_loader));
                        let _ = window.show();
                        for _ in 0..50 {
                            notify_loader_port(&window, port_for_loader);
                            thread::sleep(Duration::from_millis(200));
                        }
                        break;
                    }
                    thread::sleep(Duration::from_millis(100));
                }
            });

            // Fallback: navigate to backend once it is ready even if loader did not run
            thread::spawn(move || {
                for i in 1..=120 {
                    thread::sleep(Duration::from_millis(500));
                    if std::net::TcpStream::connect(("127.0.0.1", port_for_nav)).is_ok() {
                        let mut window = None;
                        for _ in 0..20 {
                            window = app_handle_nav
                                .get_webview_window("main")
                                .or_else(|| app_handle_nav.webview_windows().into_iter().next().map(|(_, w)| w));
                            if window.is_some() {
                                break;
                            }
                            thread::sleep(Duration::from_millis(100));
                        }
                        if let Some(window) = window {
                            let target = format!("http://127.0.0.1:{}/?t={}", port_for_nav, i);
                            if let Ok(url) = Url::parse(&target) {
                                let _ = window.navigate(url);
                            } else {
                                let js = format!("window.location.replace('{}');", target);
                                let _ = window.eval(js);
                            }
                        }
                        return;
                    }
                }
                if let Some(window) = app_handle_nav.get_webview_window("main") {
                    let _ = window.eval(
                        "document.body.innerHTML = '<h2>后台启动超时</h2><p>请检查数据目录中的日志后重试。</p>';"
                    );
                }
            });

            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                // Kill the sidecar when the window is closed
                let state: State<SidecarState> = window.state();
                stop_sidecar(&state);
                api.prevent_close();
                window.app_handle().exit(0);
            }
        })
        .invoke_handler(tauri::generate_handler![
            is_first_run,
            get_setup_info,
            complete_setup,
            pick_folder,
            pick_db_file,
        ])
        .build(tauri::generate_context!())
        .expect("error while building tauri application");

    app.run(|app_handle, event| {
        match event {
            RunEvent::Ready => {
                if let Some(window) = app_handle.get_webview_window("main") {
                    let port = {
                        let state: State<SidecarState> = app_handle.state();
                        let guard = state.port.lock().unwrap();
                        *guard
                    };
                    show_loading_window(&window, port);
                    let _ = window.show();
                    if let Some(port) = port {
                        notify_loader_port(&window, port);
                    }
                }
            }
            RunEvent::ExitRequested { .. } | RunEvent::Exit => {
                let state: State<SidecarState> = app_handle.state();
                stop_sidecar(&state);
            }
            _ => {}
        }
    });
}
