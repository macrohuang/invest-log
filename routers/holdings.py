from fastapi import APIRouter, Request, Form
from fastapi.responses import HTMLResponse, RedirectResponse
from datetime import date
from typing import Optional
from urllib.parse import quote
import database as db
import price_fetcher
from logger_config import logger
from .utils import templates

router = APIRouter()

@router.get("/holdings", response_class=HTMLResponse)
async def holdings_detail_page(
    request: Request,
    msg: Optional[str] = None,
    msg_type: Optional[str] = None
):
    """Detailed holdings page showing each symbol."""
    holdings_by_symbol = db.get_holdings_by_symbol()
    return templates.TemplateResponse("holdings.html", {
        "request": request,
        "holdings_by_symbol": holdings_by_symbol,
        "currencies": db.CURRENCIES,
        "message": msg,
        "message_type": msg_type or "info"
    })

@router.get("/symbol/{symbol}", response_class=HTMLResponse)
async def symbol_detail_page(
    request: Request,
    symbol: str,
    currency: str = "CNY",
    year: Optional[int] = None,
    account_id: Optional[str] = None
):
    """Symbol detail page with transaction history."""
    current_year = date.today().year
    selected_year = year or current_year
    
    # Get transactions for this symbol and year
    transactions = db.get_transactions(
        symbol=symbol,
        currency=currency,
        account_id=account_id,
        year=selected_year,
        limit=500
    )
    
    # Get current holding info
    holdings = db.get_holdings(account_id=account_id) if account_id else db.get_holdings()
    holding_info = None
    for h in holdings:
        if h['symbol'] == symbol.upper() and h['currency'] == currency:
            holding_info = h
            break
    
    # Get available years for this symbol
    all_transactions = db.get_transactions(
        symbol=symbol,
        currency=currency,
        account_id=account_id,
        limit=1000
    )
    years = sorted(set(
        int(t['transaction_date'][:4]) if isinstance(t['transaction_date'], str) 
        else t['transaction_date'].year 
        for t in all_transactions
    ), reverse=True)

    account_name = None
    if account_id:
        for acc in db.get_accounts():
            if acc.get('account_id') == account_id:
                account_name = acc.get('account_name') or account_id
                break
    
    return templates.TemplateResponse("symbol.html", {
        "request": request,
        "symbol": symbol.upper(),
        "currency": currency,
        "holding": holding_info,
        "transactions": transactions,
        "selected_year": selected_year,
        "years": years or [current_year],
        "asset_type_labels": db.get_asset_type_labels(),
        "account_id": account_id,
        "account_name": account_name
    })

@router.post("/symbol/{symbol}/adjust")
async def adjust_symbol_value(
    symbol: str,
    new_value: float = Form(...),
    currency: str = Form(...),
    account_id: str = Form(...),
    asset_type: str = Form("stock"),
    notes: str = Form("")
):
    logger.info(f"Asset value adjusted: {symbol} {currency} = {new_value}")
    """Adjust the value of an asset."""
    db.adjust_asset_value(
        symbol=symbol,
        new_value=new_value,
        currency=currency,
        account_id=account_id,
        asset_type=asset_type,
        notes=notes if notes else None
    )
    return RedirectResponse(
        url=f"/symbol/{symbol}?currency={currency}&account_id={account_id}",
        status_code=303
    )

@router.post("/holdings/update-price")
async def update_asset_price(
    symbol: str = Form(...),
    currency: str = Form(...)
):
    logger.info(f"Price update requested: {symbol} {currency}")
    """Fetch latest price and store it."""
    price, message = price_fetcher.fetch_price(
        symbol=symbol,
        currency=currency
    )
    
    if price is not None:
        # Store the latest price
        db.update_latest_price(
            symbol=symbol,
            currency=currency,
            price=price
        )
        
        # Log the operation
        db.add_operation_log(
            operation_type="PRICE_UPDATE",
            symbol=symbol.upper(),
            currency=currency,
            details=message,
            price_fetched=price
        )
        logger.info(f"Price updated: {symbol} {currency} = {price}")
        
        # Redirect with success message
        success_msg = quote(f"{symbol} {currency} 价格已更新: {price}")
        return RedirectResponse(url=f"/holdings?msg={success_msg}&msg_type=success", status_code=303)
    else:
        # Log failed attempt
        db.add_operation_log(
            operation_type="PRICE_UPDATE_FAILED",
            symbol=symbol.upper(),
            currency=currency,
            details=message
        )
        logger.warning(f"Price update failed: {symbol} {currency} - {message}")
        
        # Redirect with error message
        error_msg = quote(f"{symbol} {currency} {message}")
        return RedirectResponse(url=f"/holdings?msg={error_msg}&msg_type=error", status_code=303)

@router.post("/holdings/manual-update-price")
async def manual_update_asset_price(
    symbol: str = Form(...),
    currency: str = Form(...),
    price: float = Form(...)
):
    """Manually update the latest price for a symbol."""
    logger.info(f"Manual price update requested: {symbol} {currency} = {price}")
    
    # Store the manual price
    db.update_latest_price(
        symbol=symbol,
        currency=currency,
        price=price
    )
    
    # Log the operation
    db.add_operation_log(
        operation_type="MANUAL_PRICE_UPDATE",
        symbol=symbol.upper(),
        currency=currency,
        details="Manual price update",
        price_fetched=price
    )
    
    # Redirect with success message
    success_msg = quote(f"{symbol} {currency} 价格已手动更新: {price}")
    return RedirectResponse(url=f"/holdings?msg={success_msg}&msg_type=success", status_code=303)

@router.post("/holdings/toggle-auto-update")
async def toggle_auto_update(
    symbol: str = Form(...),
    auto_update: int = Form(...)
):
    """Toggle auto update status for a symbol."""
    db.update_symbol_auto_update(symbol, auto_update)
    return {"status": "success", "symbol": symbol, "auto_update": auto_update}

@router.post("/holdings/update-asset-type")
async def update_asset_type(
    symbol: str = Form(...),
    asset_type: str = Form(...)
):
    """Update asset type for a symbol."""
    try:
        success, old_type, new_type = db.update_symbol_asset_type(symbol, asset_type)
    except ValueError:
        error_msg = quote(f"Invalid asset type: {asset_type}")
        return RedirectResponse(url=f"/holdings?msg={error_msg}&msg_type=error", status_code=303)

    if not success:
        error_msg = quote(f"Symbol not found: {symbol}")
        return RedirectResponse(url=f"/holdings?msg={error_msg}&msg_type=error", status_code=303)

    if old_type == new_type:
        info_msg = quote(f"Asset type unchanged for {symbol}")
        return RedirectResponse(url=f"/holdings?msg={info_msg}&msg_type=info", status_code=303)

    db.add_operation_log(
        operation_type="ASSET_TYPE_UPDATE",
        symbol=symbol.upper(),
        details=f"{old_type} -> {new_type}"
    )
    success_msg = quote(f"{symbol} asset type updated to {new_type}")
    return RedirectResponse(url=f"/holdings?msg={success_msg}&msg_type=success", status_code=303)

@router.post("/holdings/update-all")
async def update_all_prices(
    currency: str = Form(...)
):
    """Update all symbols for a currency that have auto_update enabled."""
    holdings_by_symbol = db.get_holdings_by_symbol()
    if currency not in holdings_by_symbol:
        error_msg = quote(f"未找到币种: {currency}")
        return RedirectResponse(url=f"/holdings?msg={error_msg}&msg_type=error", status_code=303)
    
    symbols_data = holdings_by_symbol[currency]['symbols']
    updated_count = 0
    errors = []
    
    for s in symbols_data:
        # Check if auto_update is enabled for this symbol
        if s.get('auto_update', 1):
            price, message = price_fetcher.fetch_price(s['symbol'], currency)
            if price is not None:
                db.update_latest_price(s['symbol'], currency, price)
                db.add_operation_log(
                    operation_type="PRICE_UPDATE",
                    symbol=s['symbol'],
                    currency=currency,
                    details=message,
                    price_fetched=price
                )
                updated_count += 1
            else:
                errors.append(f"{s['symbol']}: {message}")
    
    msg = f"成功更新 {updated_count} 个标的价格。"
    msg_type = "success"
    if errors:
        msg += " 部分失败: " + "; ".join(errors[:2]) + ("..." if len(errors) > 2 else "")
        msg_type = "info"
        
    return RedirectResponse(url=f"/holdings?msg={quote(msg)}&msg_type={msg_type}", status_code=303)

@router.post("/holdings/quick-trade")
async def quick_trade(
    symbol: str = Form(...),
    currency: str = Form(...),
    account_id: str = Form(...),
    asset_type: str = Form("stock"),
    transaction_type: str = Form(...),
    quantity: float = Form(...),
    price: float = Form(...),
    commission: float = Form(0),
    notes: str = Form(""),
    link_cash: bool = Form(False)
):
    logger.info(f"Quick trade: {transaction_type} {quantity} {symbol} @ {price} {currency}")
    """Quick trade from holdings page."""
    db.add_transaction(
        transaction_date=date.today(),
        symbol=symbol,
        transaction_type=transaction_type,
        asset_type=asset_type,
        currency=currency,
        quantity=quantity,
        price=price,
        account_id=account_id,
        commission=commission,
        notes=notes if notes else None,
        link_cash=link_cash
    )
    return RedirectResponse(url="/holdings", status_code=303)
