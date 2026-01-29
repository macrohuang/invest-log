from fastapi.templating import Jinja2Templates

import config

templates = Jinja2Templates(directory=str(config.get_resource_path("templates")))

def format_currency(value):
    if value is None:
        return "-"
    try:
        return "{:,.2f}".format(float(value))
    except (ValueError, TypeError):
        return value

templates.env.filters["format_currency"] = format_currency
