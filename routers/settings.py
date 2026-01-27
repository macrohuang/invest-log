from fastapi import APIRouter, Request, Form
from fastapi.responses import HTMLResponse, RedirectResponse
from typing import Optional
from urllib.parse import quote
from datetime import date
import database as db
import config
from logger_config import logger
from .utils import templates

router = APIRouter()

@router.get("/settings", response_class=HTMLResponse)
async def settings_page(
    request: Request,
    msg: Optional[str] = None,
    msg_type: Optional[str] = None,
    tab: str = "allocation"
):
    """Settings page for allocation ranges, accounts, and asset type management."""
    settings = db.get_allocation_settings()
    asset_types = db.get_asset_types()
    accounts = db.get_accounts()

    # Build a map for easy template access
    settings_map = {}
    for s in settings:
        if s['currency'] not in settings_map:
            settings_map[s['currency']] = {}
        settings_map[s['currency']][s['asset_type']] = {
            'min': s['min_percent'],
            'max': s['max_percent']
        }
    
    # Check which asset types can be deleted
    asset_types_with_status = []
    for at in asset_types:
        can_delete, status_msg = db.can_delete_asset_type(at['code'])
        asset_types_with_status.append({
            **at,
            'can_delete': can_delete,
            'delete_message': status_msg
        })
    
    # Check which accounts can be deleted
    accounts_with_status = []
    for acc in accounts:
        can_delete = not db.check_account_in_use(acc['account_id'])
        accounts_with_status.append({
            **acc,
            'can_delete': can_delete
        })
    
    return templates.TemplateResponse("settings.html", {
        "request": request,
        "currencies": db.CURRENCIES,
        "asset_types": asset_types_with_status,
        "accounts": accounts_with_status,
        "settings_map": settings_map,
        "user_config": config.USER_CONFIG,
        "current_db_path": config.DB_PATH,
        "message": msg,
        "message_type": msg_type or "info",
        "active_tab": tab
    })

@router.post("/settings")
async def settings_submit(request: Request):
    """Handle settings form submission."""
    form_data = await request.form()
    asset_types = db.get_asset_types()
    
    updated_count = 0
    for currency in db.CURRENCIES:
        for at in asset_types:
            asset_type = at['code']
            min_key = f"{currency}_{asset_type}_min"
            max_key = f"{currency}_{asset_type}_max"
            
            min_val = form_data.get(min_key, "") or "0"
            max_val = form_data.get(max_key, "") or "100"
            db.set_allocation_setting(
                currency=currency,
                asset_type=asset_type,
                min_percent=float(min_val),
                max_percent=float(max_val)
            )
            updated_count += 1
    
    return RedirectResponse(url=f"/settings?msg={quote('配置已保存')}&msg_type=success", status_code=303)

@router.post("/settings/database")
async def database_settings_submit(
    db_name: str = Form(...),
    use_icloud: bool = Form(False)
):
    """Handle database settings form submission."""
    logger.info(f"Updating database settings: db_name={db_name}, use_icloud={use_icloud}")
    
    new_config = {
        "db_name": db_name,
        "use_icloud": use_icloud
    }
    
    try:
        config.save_user_config(new_config)
        # We don't update config.DB_PATH or config.USER_CONFIG in memory here 
        # because it's better to let the app restart (which happens automatically 
        # in dev due to config.json change) or let the user restart.
        return RedirectResponse(
            url=f"/settings?tab=database&msg={quote('数据库配置已保存，重启应用后生效')}&msg_type=success", 
            status_code=303
        )
    except Exception as e:
        logger.error(f"Failed to save database config: {e}")
        return RedirectResponse(
            url=f"/settings?tab=database&msg={quote(f'保存失败: {str(e)}')}&msg_type=error", 
            status_code=303
        )

@router.post("/settings/add-asset-type")
async def add_asset_type(
    code: str = Form(...),
    label: str = Form(...)
):
    logger.info(f"Adding new asset type: {code} - {label}")
    """Add a new asset type."""
    success = db.add_asset_type(code=code, label=label)
    if success:
        return RedirectResponse(url=f"/settings?msg={quote('资产类别已添加')}&msg_type=success", status_code=303)
    else:
        return RedirectResponse(url=f"/settings?msg={quote('添加失败，代码可能已存在')}&msg_type=error", status_code=303)

@router.post("/settings/delete-asset-type/{code}")
async def delete_asset_type(code: str):
    """Delete an asset type."""
    logger.info(f"Deleting asset type: {code}")
    # 检查该资产类别下是否有持仓，如果没有持仓才允许删除，否则提示有开仓资产，不允许删除
    if db.check_asset_type_in_use(code):
        return RedirectResponse(url=f"/settings?tab=asset_types&msg={quote('该资产类别下有开仓资产，不允许删除')}&msg_type=error", status_code=303)
    success, message = db.delete_asset_type(code)
    
    if success:
        return RedirectResponse(url=f"/settings?tab=asset_types&msg={quote('资产类别已删除')}&msg_type=success", status_code=303)
    else:
        # 如果删除失败，通常是因为该类别下已有交易记录（即注释中所说的“有持仓”）
        error_msg = "该资产类别下有开仓资产，不允许删除" if "transactions exist" in message else message
        return RedirectResponse(url=f"/settings?tab=asset_types&msg={quote(error_msg)}&msg_type=error", status_code=303)


@router.post("/settings/add-account")
async def add_account_submit(
    account_id: str = Form(...),
    account_name: str = Form(...),
    broker: Optional[str] = Form(None),
    account_type: Optional[str] = Form(None),
    initial_balance_cny: float = Form(0),
    initial_balance_usd: float = Form(0),
    initial_balance_hkd: float = Form(0)
):
    """Add a new account."""
    logger.info(f"Adding new account: {account_id} - {account_name}")
    success = db.add_account(
        account_id=account_id,
        account_name=account_name,
        broker=broker,
        account_type=account_type
    )
    if success:
        # Create initial balance transactions
        today = date.today()
        balance_map = {
            'CNY': initial_balance_cny,
            'USD': initial_balance_usd,
            'HKD': initial_balance_hkd
        }
        
        for currency, amount in balance_map.items():
            if amount > 0:
                db.add_transaction(
                    transaction_date=today,
                    symbol='CASH',
                    transaction_type='TRANSFER_IN',
                    asset_type='cash',
                    quantity=amount,
                    price=1.0,
                    account_id=account_id,
                    currency=currency,
                    notes='Initial balance'
                )
                logger.info(f"Created initial balance: {amount} {currency} for account {account_id}")
        
        return RedirectResponse(url=f"/settings?tab=accounts&msg={quote('账户已添加')}&msg_type=success", status_code=303)
    else:
        return RedirectResponse(url=f"/settings?tab=accounts&msg={quote('添加失败，账户ID可能已存在')}&msg_type=error", status_code=303)


@router.post("/settings/delete-account/{account_id}")
async def delete_account(account_id: str):
    """Delete an account."""
    logger.info(f"Deleting account: {account_id}")
    success, message = db.delete_account(account_id)
    if success:
        return RedirectResponse(url=f"/settings?tab=accounts&msg={quote('账户已删除')}&msg_type=success", status_code=303)
    else:
        return RedirectResponse(url=f"/settings?tab=accounts&msg={quote(message)}&msg_type=error", status_code=303)
