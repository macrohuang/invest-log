"""
Transaction Log Web Interface

FastAPI application with Jinja2 templates.
"""

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
import database as db
from logger_config import logger
from routers import overview, transactions, holdings, settings, api

app = FastAPI(title="Invest Log")

# Setup static files
app.mount("/static", StaticFiles(directory="static"), name="static")

# Initialize database on startup
@app.on_event("startup")
def startup():
    db.init_database()
    logger.info("Application started, database initialized")

# Include Routers
app.include_router(overview.router)
app.include_router(transactions.router)
app.include_router(holdings.router)
app.include_router(settings.router)
app.include_router(api.router)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8000)
