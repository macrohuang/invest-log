from fastapi import APIRouter, Request
from fastapi.responses import HTMLResponse
import database as db
from .utils import templates

router = APIRouter()

@router.get("/", response_class=HTMLResponse)
async def index(request: Request):
    """Dashboard with holdings overview by currency."""
    holdings_by_currency = db.get_holdings_by_currency()
    return templates.TemplateResponse("index.html", {
        "request": request,
        "holdings_by_currency": holdings_by_currency,
        "currencies": db.CURRENCIES,
        "asset_type_labels": db.ASSET_TYPE_LABELS
    })

@router.get("/charts", response_class=HTMLResponse)
async def charts_page(request: Request):
    """Charts and analytics page - per symbol within each currency."""
    holdings_by_symbol = db.get_holdings_by_symbol()
    return templates.TemplateResponse("charts.html", {
        "request": request,
        "holdings_by_symbol": holdings_by_symbol,
        "asset_type_labels": db.ASSET_TYPE_LABELS
    })
