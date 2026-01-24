"""
Application Configuration

Centralizes configuration settings including database path.
"""

import json
from pathlib import Path
import os

# Project root directory
BASE_DIR = Path(__file__).parent.absolute()

# iCloud Drive path on macOS
ICLOUD_BASE = Path.home() / "Library" / "Mobile Documents" / "com~apple~CloudDocs"
ICLOUD_APP_FOLDER = ICLOUD_BASE / "InvestLog"

# Config file path
USER_CONFIG_FILE = BASE_DIR / "config.json"

def load_user_config():
    """Load user configuration from JSON file."""
    defaults = {
        "db_name": "transactions.db",
        "use_icloud": True
    }
    if USER_CONFIG_FILE.exists():
        try:
            with open(USER_CONFIG_FILE, "r") as f:
                user_config = json.load(f)
                defaults.update(user_config)
        except Exception as e:
            print(f"Error loading config: {e}")
    return defaults

def save_user_config(config_dict):
    """Save user configuration to JSON file."""
    with open(USER_CONFIG_FILE, "w") as f:
        json.dump(config_dict, f, indent=4)

# Load current settings
USER_CONFIG = load_user_config()

# Database configuration
if USER_CONFIG["use_icloud"]:
    # Ensure the iCloud app folder exists
    ICLOUD_APP_FOLDER.mkdir(parents=True, exist_ok=True)
    DB_PATH = str(ICLOUD_APP_FOLDER / USER_CONFIG["db_name"])
else:
    DB_PATH = str(BASE_DIR / USER_CONFIG["db_name"])

# For local development/testing, you can override with environment variable
if os.environ.get("INVEST_LOG_DB_PATH"):
    DB_PATH = os.environ["INVEST_LOG_DB_PATH"]
