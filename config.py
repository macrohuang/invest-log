"""
Application Configuration

Centralizes configuration settings including database path.
"""

from pathlib import Path
import os

# iCloud Drive path on macOS
ICLOUD_BASE = Path.home() / "Library" / "Mobile Documents" / "com~apple~CloudDocs"
ICLOUD_APP_FOLDER = ICLOUD_BASE / "InvestLog"

# Ensure the iCloud app folder exists
ICLOUD_APP_FOLDER.mkdir(parents=True, exist_ok=True)

# Database configuration
# Store database in iCloud for automatic sync and backup
DB_PATH = str(ICLOUD_APP_FOLDER / "transactions.db")

# For local development/testing, you can override with environment variable
if os.environ.get("INVEST_LOG_DB_PATH"):
    DB_PATH = os.environ["INVEST_LOG_DB_PATH"]
