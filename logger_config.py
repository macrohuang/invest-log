"""
Logging Configuration Module

Sets up application logging with daily rotation, keeping 7 days of logs.
"""

import logging
import os
from logging.handlers import TimedRotatingFileHandler
from pathlib import Path

def _resolve_log_dir() -> Path:
    env_data_dir = os.environ.get("INVEST_LOG_DATA_DIR")
    if env_data_dir:
        return Path(env_data_dir) / "logs"
    try:
        import config
        return config.get_data_dir() / "logs"
    except Exception:
        return Path.home() / ".investlog" / "logs"

LOG_DIR = _resolve_log_dir()
LOG_DIR.mkdir(parents=True, exist_ok=True)

def setup_logging():
    """Configure logging with daily rotation, keeping 7 days of logs."""
    logger = logging.getLogger("invest_log")
    
    # Avoid adding handlers multiple times
    if logger.handlers:
        return logger
    
    logger.setLevel(logging.INFO)
    
    # File handler with daily rotation
    file_handler = TimedRotatingFileHandler(
        LOG_DIR / "app.log",
        when="midnight",
        interval=1,
        backupCount=7,
        encoding="utf-8"
    )
    file_handler.suffix = "%Y-%m-%d"
    file_handler.setLevel(logging.INFO)
    
    # Console handler
    console_handler = logging.StreamHandler()
    console_handler.setLevel(logging.INFO)
    
    # Formatter
    formatter = logging.Formatter(
        "%(asctime)s - %(name)s - %(levelname)s - %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S"
    )
    file_handler.setFormatter(formatter)
    console_handler.setFormatter(formatter)
    
    logger.addHandler(file_handler)
    logger.addHandler(console_handler)
    
    return logger

# Create logger instance
logger = setup_logging()
