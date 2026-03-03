const displayTimeZone = 'Asia/Shanghai';
const displayDateFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: displayTimeZone,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
});
const displayDateTimeFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: displayTimeZone,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
});

function extractDateParts(formatter, value) {
  const parts = {};
  formatter.formatToParts(value).forEach((part) => {
    if (part.type !== 'literal') {
      parts[part.type] = part.value;
    }
  });
  return parts;
}

function parseTimestampAsDate(value) {
  if (value === null || value === undefined) {
    return null;
  }

  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value;
  }

  const text = String(value).trim();
  if (!text) {
    return null;
  }

  if (/^\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}$/.test(text)) {
    const parsedUTC = new Date(text.replace(' ', 'T') + 'Z');
    return Number.isNaN(parsedUTC.getTime()) ? null : parsedUTC;
  }

  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) {
    const parsedDate = new Date(`${text}T00:00:00Z`);
    return Number.isNaN(parsedDate.getTime()) ? null : parsedDate;
  }

  const parsed = new Date(text);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function formatDateInDisplayTimezone(value = new Date()) {
  const date = parseTimestampAsDate(value);
  if (!date) {
    return '';
  }
  const parts = extractDateParts(displayDateFormatter, date);
  return `${parts.year}-${parts.month}-${parts.day}`;
}

function formatDateTimeInDisplayTimezone(value) {
  const date = parseTimestampAsDate(value);
  if (!date) {
    return value ? String(value) : '—';
  }
  const parts = extractDateParts(displayDateTimeFormatter, date);
  return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`;
}

function formatDateTimeISOInDisplayTimezone(value = new Date()) {
  const date = parseTimestampAsDate(value);
  if (!date) {
    return '';
  }
  const parts = extractDateParts(displayDateTimeFormatter, date);
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}:${parts.second}+08:00`;
}

function formatMoney(value, currency) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  const symbol = currencySymbols[currency] || '';
  try {
    return new Intl.NumberFormat('en-US', {
      style: symbol ? 'currency' : 'decimal',
      currency: currency,
      maximumFractionDigits: 2,
    }).format(value);
  } catch (err) {
    return `${symbol}${value.toFixed(2)}`;
  }
}

function formatMoneyPlain(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'decimal',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(value);
  } catch (err) {
    return Number(value).toFixed(2);
  }
}

function formatNumber(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'decimal',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(value);
  } catch (err) {
    return Number(value).toFixed(2);
  }
}

function formatPercent(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  return `${Number(value).toFixed(2)}%`;
}

function escapeHtml(value) {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function escapeRegExp(value) {
  return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function highlightMatchText(text, keyword) {
  const source = String(text || '');
  const needle = String(keyword || '').trim();
  if (!needle) {
    return escapeHtml(source);
  }
  const regex = new RegExp(escapeRegExp(needle), 'ig');
  let lastIndex = 0;
  let output = '';
  let matched = false;
  let hit = regex.exec(source);
  while (hit) {
    matched = true;
    const start = hit.index;
    const end = start + hit[0].length;
    output += escapeHtml(source.slice(lastIndex, start));
    output += `<mark>${escapeHtml(source.slice(start, end))}</mark>`;
    lastIndex = end;
    hit = regex.exec(source);
  }
  if (!matched) {
    return escapeHtml(source);
  }
  output += escapeHtml(source.slice(lastIndex));
  return output;
}

function formatValue(value, currency) {
  if (currency) {
    return formatMoney(value, currency);
  }
  return formatNumber(value);
}

