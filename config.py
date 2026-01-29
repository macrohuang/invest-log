"""
Application Configuration

Centralizes configuration settings including database path.
Supports desktop app mode with dynamic data directory configuration.
"""

import json
from pathlib import Path
import os
import sys

# Project root directory (PyInstaller uses a temp bundle path)
if getattr(sys, "frozen", False) and hasattr(sys, "_MEIPASS"):
    BASE_DIR = Path(sys._MEIPASS)
else:
    BASE_DIR = Path(__file__).parent.absolute()

# Platform detection
IS_MACOS = sys.platform == "darwin"
IS_WINDOWS = sys.platform == "win32"

# iCloud Drive path on macOS
ICLOUD_BASE = Path.home() / "Library" / "Mobile Documents" / "com~apple~CloudDocs"
ICLOUD_APP_FOLDER = ICLOUD_BASE / "InvestLog"

# App config directory (for desktop app settings)
if IS_MACOS:
    APP_CONFIG_DIR = Path.home() / "Library" / "Application Support" / "InvestLog"
elif IS_WINDOWS:
    APP_CONFIG_DIR = Path(os.environ.get("APPDATA", Path.home())) / "InvestLog"
else:
    APP_CONFIG_DIR = Path.home() / ".config" / "investlog"

# Config file paths
USER_CONFIG_FILE = BASE_DIR / "config.json"  # Legacy: project-local config
APP_CONFIG_FILE = APP_CONFIG_DIR / "config.json"  # Desktop app config

# Runtime state (can be modified by CLI args)
_runtime_data_dir: Path | None = None
_runtime_port: int = 8000


def get_app_config_path() -> Path:
    """Get the path to the app configuration file."""
    return APP_CONFIG_FILE


def get_resource_path(relative_path: str | Path) -> Path:
    """Resolve resource paths for both dev and PyInstaller builds."""
    return BASE_DIR / relative_path


def is_first_run() -> bool:
    """Check if this is the first run (no config file exists)."""
    return not APP_CONFIG_FILE.exists()


def is_icloud_available() -> bool:
    """Check if iCloud Drive is available on this system."""
    return IS_MACOS and ICLOUD_BASE.exists()


def get_icloud_app_folder() -> Path:
    """Get the iCloud app folder path."""
    return ICLOUD_APP_FOLDER


def set_runtime_data_dir(data_dir: str | Path | None) -> None:
    """Set the runtime data directory (from CLI args)."""
    global _runtime_data_dir
    if data_dir:
        _runtime_data_dir = Path(data_dir)


def set_runtime_port(port: int) -> None:
    """Set the runtime port (from CLI args)."""
    global _runtime_port
    _runtime_port = port


def get_runtime_port() -> int:
    """Get the runtime port."""
    return _runtime_port


def load_user_config() -> dict:
    """Load user configuration from JSON file.
    
    Priority:
    1. Desktop app config (APP_CONFIG_FILE)
    2. Legacy project-local config (USER_CONFIG_FILE)
    3. Defaults
    """
    defaults = {
        "db_name": "transactions.db",
        "use_icloud": True,
        "data_dir": None,  # Custom data directory path
        "setup_complete": False  # Whether first-run setup is done
    }
    
    # Try desktop app config first
    config_file = APP_CONFIG_FILE if APP_CONFIG_FILE.exists() else USER_CONFIG_FILE
    
    if config_file.exists():
        try:
            with open(config_file, "r") as f:
                user_config = json.load(f)
                defaults.update(user_config)
        except Exception as e:
            print(f"Error loading config: {e}")
    return defaults


def save_user_config(config_dict: dict, use_app_config: bool = True) -> None:
    """Save user configuration to JSON file.
    
    Args:
        config_dict: Configuration dictionary to save
        use_app_config: If True, save to desktop app config location
    """
    config_file = APP_CONFIG_FILE if use_app_config else USER_CONFIG_FILE
    config_file.parent.mkdir(parents=True, exist_ok=True)
    
    with open(config_file, "w") as f:
        json.dump(config_dict, f, indent=4)


def complete_setup(use_icloud: bool = False, custom_data_dir: str | None = None) -> str:
    """Complete the first-run setup and return the configured data directory.
    
    Args:
        use_icloud: Whether to use iCloud for data storage (macOS only)
        custom_data_dir: Custom directory path for data storage
        
    Returns:
        The configured data directory path
    """
    config = load_user_config()
    
    if use_icloud and IS_MACOS:
        ICLOUD_APP_FOLDER.mkdir(parents=True, exist_ok=True)
        data_dir = str(ICLOUD_APP_FOLDER)
        config["use_icloud"] = True
        config["data_dir"] = None
    elif custom_data_dir:
        Path(custom_data_dir).mkdir(parents=True, exist_ok=True)
        data_dir = custom_data_dir
        config["use_icloud"] = False
        config["data_dir"] = custom_data_dir
    else:
        # Default to app config directory
        APP_CONFIG_DIR.mkdir(parents=True, exist_ok=True)
        data_dir = str(APP_CONFIG_DIR)
        config["use_icloud"] = False
        config["data_dir"] = data_dir
    
    config["setup_complete"] = True
    save_user_config(config)
    
    return data_dir


def get_data_dir() -> Path:
    """Get the current data directory path.
    
    Priority:
    1. Runtime data dir (from CLI args)
    2. Environment variable INVEST_LOG_DATA_DIR
    3. Config file setting
    4. Default based on platform/iCloud
    """
    # 1. Runtime override (CLI args)
    if _runtime_data_dir:
        _runtime_data_dir.mkdir(parents=True, exist_ok=True)
        return _runtime_data_dir
    
    # 2. Environment variable
    env_data_dir = os.environ.get("INVEST_LOG_DATA_DIR")
    if env_data_dir:
        path = Path(env_data_dir)
        path.mkdir(parents=True, exist_ok=True)
        return path
    
    # 3. Load from config
    config = load_user_config()
    
    # Custom data directory set in config
    if config.get("data_dir"):
        path = Path(config["data_dir"])
        path.mkdir(parents=True, exist_ok=True)
        return path
    
    # 4. iCloud if enabled and available
    if config.get("use_icloud") and IS_MACOS:
        ICLOUD_APP_FOLDER.mkdir(parents=True, exist_ok=True)
        return ICLOUD_APP_FOLDER
    
    # 5. Default to app config directory
    APP_CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    return APP_CONFIG_DIR


def get_db_path() -> str:
    """Get the full database file path."""
    # Support legacy environment variable
    if os.environ.get("INVEST_LOG_DB_PATH"):
        return os.environ["INVEST_LOG_DB_PATH"]
    
    config = load_user_config()
    db_name = config.get("db_name", "transactions.db")
    return str(get_data_dir() / db_name)


# Load current settings (for backwards compatibility)
USER_CONFIG = load_user_config()

# Database path (for backwards compatibility with existing imports)
DB_PATH = get_db_path()
