from fastapi import APIRouter, Request, Form
from fastapi.responses import HTMLResponse, RedirectResponse
from datetime import date
import database as db
from logger_config import logger
from .utils import templates

router = APIRouter()

@router.get("/transactions", response_class=HTMLResponse)
async def transactions_page(request: Request, page: int = 1):
    """Transaction list page with pagination."""
    per_page = 100
    offset = (page - 1) * per_page
    
    total_count = db.get_transaction_count()
    transactions = db.get_transactions(limit=per_page, offset=offset)
    
    total_pages = (total_count + per_page - 1) // per_page
    
    return templates.TemplateResponse("transactions.html", {
        "request": request,
        "transactions": transactions,
        "current_page": page,
        "total_pages": total_pages,
        "total_count": total_count,
        "per_page": per_page
    })

@router.get("/add", response_class=HTMLResponse)
async def add_transaction_page(request: Request):
    """Add transaction form."""
    accounts = db.get_accounts()
    asset_types = db.get_asset_types()
    holdings = db.get_holdings()
    return templates.TemplateResponse("add.html", {
        "request": request,
        "accounts": accounts,
        "today": date.today().isoformat(),
        "currencies": db.CURRENCIES,
        "asset_types": asset_types,
        "holdings": holdings
    })

@router.post("/add")
async def add_transaction_submit(
    transaction_date: str = Form(...),
    symbol: str = Form(...),
    transaction_type: str = Form(...),
    asset_type: str = Form("stock"),
    currency: str = Form("CNY"),
    quantity: float = Form(...),
    price: float = Form(...),
    account_id: str = Form(...),
    commission: float = Form(0),
    notes: str = Form(""),
    link_cash: bool = Form(False)
):
    """Handle add transaction form submission."""
    db.add_transaction(
        transaction_date=date.fromisoformat(transaction_date),
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
    logger.info(f"Transaction added: {transaction_type} {quantity} {symbol} @ {price} {currency}")
    return RedirectResponse(url="/transactions", status_code=303)
