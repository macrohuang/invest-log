async function renderOverview() {
  view.innerHTML = `
    <div class="section-title">Overview</div>
    <div class="overview-summary card">Loading overview...</div>
    <div class="section-sub">Market value, allocations, and warning bands.</div>
    <div class="grid three">
      <div class="card">Loading CNY allocation...</div>
      <div class="card">Loading USD allocation...</div>
      <div class="card">Loading HKD allocation...</div>
    </div>
  `;

  try {
    const [byCurrency, bySymbol, exchangeRates] = await Promise.all([
      fetchJSON('/api/holdings-by-currency'),
      fetchJSON('/api/holdings-by-symbol'),
      fetchJSON('/api/exchange-rates'),
    ]);

    const currencyOrder = ['CNY', 'USD', 'HKD'];
    const currencyList = currencyOrder.filter((curr) => (byCurrency || {})[curr]);
    const extraCurrencies = Object.keys(byCurrency || {})
      .filter((curr) => !currencyOrder.includes(curr))
      .sort();
    currencyList.push(...extraCurrencies);
    const rateMap = { CNY: 1 };
    (exchangeRates || []).forEach((item) => {
      if (item && item.to_currency === 'CNY' && Number(item.rate) > 0) {
        rateMap[item.from_currency] = Number(item.rate);
      }
    });

    let totalMarket = 0;
    let totalCost = 0;

    if (bySymbol) {
      Object.entries(bySymbol).forEach(([currency, entry]) => {
        const rate = Number(rateMap[currency] || 0);
        if (rate <= 0) {
          return;
        }
        totalMarket += Number(entry.total_market_value || 0) * rate;
        totalCost += Number(entry.total_cost || 0) * rate;
      });
    }

    const totalPnL = totalMarket - totalCost;
    const pnlClass = totalPnL >= 0 ? 'pill positive' : 'pill negative';
    const summaryCard = `
      <div class="overview-summary card">
        <div class="summary-title">Total Market Value (CNY)</div>
        <div class="summary-value" data-sensitive>${formatMoney(totalMarket, 'CNY')}</div>
        <div class="summary-sub">Converted by Settings exchange rates.</div>
        <div class="summary-pill ${pnlClass}" data-sensitive>${formatMoney(totalPnL, 'CNY')} total PnL</div>
      </div>
    `;

    const allocationCards = currencyList.map((currency) => {
      const data = byCurrency[currency] || { total: 0, allocations: [] };
      const allocationItems = (data.allocations || []).map((alloc) => ({
        label: alloc.label,
        value: alloc.amount || 0,
        amount: alloc.amount,
      }));
      const pieChart = renderPieChart({
        items: allocationItems,
        totalLabel: 'Total',
        totalValue: data.total || 0,
        currency,
      });
      const allocations = (data.allocations || []).map((alloc) => {
        const warning = alloc.warning ? `<div class="alert">${escapeHtml(alloc.warning)}</div>` : '';
        return `
          <div class="list-item">
            <div>
              <strong>${escapeHtml(alloc.label)}</strong>
              <div class="bar"><span style="width:${alloc.percent}%;"></span></div>
            </div>
            <div style="text-align:right;">
              <div>${formatPercent(alloc.percent)}</div>
              <div data-sensitive>${formatMoney(alloc.amount, currency)}</div>
              ${warning}
            </div>
          </div>
        `;
      }).join('');
      const listMarkup = allocations ? `<div class="list">${allocations}</div>` : '';

      return `
        <div class="card">
          <h3>${currency} Allocation</h3>
          ${pieChart}
          ${listMarkup}
        </div>
      `;
    }).join('');

    view.innerHTML = `
      <div class="section-title">Overview</div>
      ${summaryCard}
      <div class="section-sub">Market value, allocations, and warning bands.</div>
      <div class="grid three">${allocationCards}</div>
    `;
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load overview data. Check API connection.');
  }
}

