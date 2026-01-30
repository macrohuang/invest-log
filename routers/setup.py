"""
First-Run Setup Router

Handles the initial setup flow for configuring data storage location.
"""

from fastapi import APIRouter, Request, Form
from fastapi.responses import HTMLResponse, JSONResponse, RedirectResponse
from typing import Optional
import config
from .utils import templates

router = APIRouter(prefix="/setup", tags=["setup"])


@router.get("", response_class=HTMLResponse)
async def setup_page(request: Request):
    """Display the first-run setup page."""
    return templates.TemplateResponse("setup.html", {
        "request": request,
        "is_macos": config.IS_MACOS,
        "is_windows": config.IS_WINDOWS,
        "icloud_available": config.is_icloud_available(),
        "icloud_path": str(config.get_icloud_app_folder()),
        "default_path": str(config.APP_CONFIG_DIR),
    })


@router.get("/status", response_class=JSONResponse)
async def setup_status():
    """Check if setup is complete and return current configuration status."""
    user_config = config.load_user_config()
    return {
        "setup_complete": user_config.get("setup_complete", False),
        "is_first_run": config.is_first_run(),
        "is_macos": config.IS_MACOS,
        "is_windows": config.IS_WINDOWS,
        "icloud_available": config.is_icloud_available(),
        "current_data_dir": str(config.get_data_dir()),
        "use_icloud": user_config.get("use_icloud", False),
    }


@router.post("/complete", response_class=JSONResponse)
async def complete_setup_api(
    use_icloud: bool = Form(False),
    custom_path: Optional[str] = Form(None),
    existing_db_path: Optional[str] = Form(None)
):
    """Complete the setup process via API (for desktop app integration)."""
    try:
        cleaned_db_path = existing_db_path.strip() if existing_db_path else None
        data_dir = config.complete_setup(
            use_icloud=use_icloud,
            custom_data_dir=custom_path if not use_icloud else None,
            existing_db_path=cleaned_db_path
        )
        return {
            "success": True,
            "data_dir": data_dir,
            "message": "Setup completed successfully"
        }
    except Exception as e:
        return {
            "success": False,
            "error": str(e)
        }


@router.post("", response_class=RedirectResponse)
async def setup_submit(
    request: Request,
    storage_option: str = Form(...),
    custom_path: Optional[str] = Form(None),
    existing_db_path: Optional[str] = Form(None)
):
    """Handle setup form submission (for web browser mode)."""
    use_icloud = storage_option == "icloud"
    selected_custom_path = custom_path if storage_option == "custom" else None

    cleaned_db_path = existing_db_path.strip() if existing_db_path else None

    config.complete_setup(
        use_icloud=use_icloud,
        custom_data_dir=selected_custom_path,
        existing_db_path=cleaned_db_path
    )

    return RedirectResponse(url="/", status_code=303)
