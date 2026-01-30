"""
Transaction Log Web Interface

FastAPI application with Jinja2 templates.
Supports desktop app mode with CLI arguments.
"""

import argparse
import os
import signal
import sys
import threading
import time
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles

import config
import database as db
from logger_config import logger


def parse_args():
    """Parse command line arguments for desktop app mode."""
    parser = argparse.ArgumentParser(description="Invest Log - Investment Tracking Application")
    parser.add_argument(
        "--data-dir",
        type=str,
        help="Directory for storing database and application data"
    )
    parser.add_argument(
        "--port",
        type=int,
        default=8000,
        help="Port to run the server on (default: 8000)"
    )
    parser.add_argument(
        "--host",
        type=str,
        default="127.0.0.1",
        help="Host to bind the server to (default: 127.0.0.1)"
    )
    return parser.parse_args()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler for startup and shutdown."""
    # Startup
    db.init_database()
    logger.info(f"Application started, database initialized at: {config.get_db_path()}")
    
    yield
    
    # Shutdown
    logger.info("Application shutting down gracefully")


# Create FastAPI app with lifespan handler
app = FastAPI(title="Invest Log", lifespan=lifespan)

# Setup static files
app.mount(
    "/static",
    StaticFiles(directory=str(config.get_resource_path("static"))),
    name="static",
)

# Import routers after app creation to avoid circular imports
from routers import overview, transactions, holdings, settings, api, setup

# Include Routers
app.include_router(overview.router)
app.include_router(transactions.router)
app.include_router(holdings.router)
app.include_router(settings.router)
app.include_router(api.router)
app.include_router(setup.router)


def signal_handler(signum, frame):
    """Handle shutdown signals gracefully."""
    logger.info(f"Received signal {signum}, initiating graceful shutdown...")
    sys.exit(0)


def start_parent_watch():
    """Exit backend when parent process is gone (desktop sidecar mode)."""
    if os.environ.get("INVEST_LOG_PARENT_WATCH") != "1":
        return

    def _watch():
        time.sleep(2)
        while True:
            if os.getppid() == 1:
                logger.info("Parent process exited; shutting down backend.")
                os._exit(0)
            time.sleep(1)

    threading.Thread(target=_watch, daemon=True).start()


if __name__ == "__main__":
    import uvicorn
    
    # Parse CLI arguments
    args = parse_args()
    
    # Configure runtime settings from CLI args
    if args.data_dir:
        config.set_runtime_data_dir(args.data_dir)
        logger.info(f"Using data directory: {args.data_dir}")
    
    config.set_runtime_port(args.port)
    logger.info(f"Starting server on {args.host}:{args.port}")
    
    # Register signal handlers for graceful shutdown
    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)

    # Watch parent process in desktop sidecar mode
    start_parent_watch()
    
    # Run the server
    uvicorn.run(
        app,
        host=args.host,
        port=args.port,
        log_level="info"
    )
