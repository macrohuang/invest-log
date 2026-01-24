from fastapi import APIRouter, HTTPException
from typing import Optional
import database as db
from logger_config import logger

router = APIRouter(prefix="/api")

@router.get("/holdings")
async def api_holdings(account_id: Optional[str] = None):
    """Get current holdings."""
    return db.get_holdings(account_id)

@router.get("/holdings-by-currency")
async def api_holdings_by_currency():
    """Get holdings grouped by currency with allocation warnings."""
    return db.get_holdings_by_currency()

@router.get("/transactions")
async def api_transactions(
    symbol: Optional[str] = None,
    account_id: Optional[str] = None,
    transaction_type: Optional[str] = None,
    limit: int = 100
):
    """Get transactions with optional filters."""
    return db.get_transactions(
        symbol=symbol,
        account_id=account_id,
        transaction_type=transaction_type,
        limit=limit
    )

@router.get("/portfolio-history")
async def api_portfolio_history():
    """Get portfolio value over time for charts."""
    transactions = db.get_transactions(limit=1000)
    
    # Group by date and calculate cumulative investment
    history = {}
    for t in sorted(transactions, key=lambda x: x['transaction_date']):
        d = t['transaction_date']
        if d not in history:
            history[d] = 0
        
        if t['transaction_type'] == 'BUY':
            history[d] += t['total_amount']
        elif t['transaction_type'] == 'SELL':
            history[d] -= t['total_amount']
    
    # Convert to cumulative
    cumulative = []
    running_total = 0
    for d in sorted(history.keys()):
        running_total += history[d]
        cumulative.append({"date": d, "value": running_total})
    
    return cumulative

@router.delete("/transactions/{transaction_id}")
async def api_delete_transaction(transaction_id: int):
    """Delete a transaction."""
    if db.delete_transaction(transaction_id):
        logger.info(f"Transaction deleted: id={transaction_id}")
        return {"status": "deleted"}
    logger.warning(f"Transaction not found for deletion: id={transaction_id}")
    raise HTTPException(status_code=404, detail="Transaction not found")
