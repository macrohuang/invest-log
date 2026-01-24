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
    asset_types = db.get_asset_types()
    asset_type_labels = {t['code']: t['label'] for t in asset_types}
    return templates.TemplateResponse("holdings.html", {
        "request": request,
        "holdings_by_symbol": holdings_by_symbol,
        "asset_type_labels": asset_type_labels,
        "currencies": db.CURRENCIES,
        "message": msg,
        "message_type": msg_type or "info"
    })

@router.get("/symbol/{symbol}", response_class=HTMLResponse)
async def symbol_detail_page(
    request: Request,
    symbol: str,
    currency: str = "CNY",
    year: Optional[int] = None
):
    """Symbol detail page with transaction history."""
    current_year = date.today().year
    selected_year = year or current_year
    
    # Get transactions for this symbol and year
    transactions = db.get_transactions(
        symbol=symbol,
        currency=currency,
        year=selected_year,
        limit=500
    )
    
    # Get current holding info
    holdings = db.get_holdings()
    holding_info = None
    for h in holdings:
        if h['symbol'] == symbol.upper() and h['currency'] == currency:
            holding_info = h
            break
    
    # Get available years for this symbol
    all_transactions = db.get_transactions(symbol=symbol, currency=currency, limit=1000)
    years = sorted(set(
        int(t['transaction_date'][:4]) if isinstance(t['transaction_date'], str) 
        else t['transaction_date'].year 
        for t in all_transactions
    ), reverse=True)
    
    return templates.TemplateResponse("symbol.html", {
        "request": request,
        "symbol": symbol.upper(),
        "currency": currency,
        "holding": holding_info,
        "transactions": transactions,
        "selected_year": selected_year,
        "years": years or [current_year],
        "asset_type_labels": db.ASSET_TYPE_LABELS
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
        url=f"/symbol/{symbol}?currency={currency}",
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
