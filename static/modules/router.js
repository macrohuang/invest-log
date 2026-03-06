function setActiveRoute(routeKey) {
  navLinks.forEach((link) => {
    const isActive = link.dataset.route === routeKey;
    link.classList.toggle('active', isActive);
  });
}

function getRouteQuery() {
  const hash = window.location.hash || '';
  const queryIndex = hash.indexOf('?');
  if (queryIndex === -1) {
    return new URLSearchParams();
  }
  return new URLSearchParams(hash.slice(queryIndex + 1));
}


function renderRoute() {
  const hash = window.location.hash || '#/overview';
  const route = hash.replace('#/', '').split('?')[0];
  switch (route) {
    case 'holdings':
      setActiveRoute('holdings');
      renderHoldings();
      break;
    case 'ai-analysis':
      setActiveRoute('ai-analysis');
      renderAIAnalysis();
      break;
    case 'transactions':
      setActiveRoute('transactions');
      renderTransactions();
      break;
    case 'charts':
      setActiveRoute('charts');
      renderCharts();
      break;
    case 'add':
      setActiveRoute('transactions');
      renderAddTransaction();
      break;
    case 'transfer':
      setActiveRoute('transactions');
      renderTransfer();
      break;
    case 'settings':
      setActiveRoute('settings');
      renderSettings();
      break;
    case 'symbol-analysis':
      setActiveRoute('holdings');
      renderSymbolAnalysis();
      break;
    default:
      setActiveRoute('overview');
      renderOverview();
  }
}
