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

function renderMarkdownLite(value) {
  const source = String(value || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n').trim();
  if (!source) {
    return '<p class="md-paragraph">—</p>';
  }

  const lines = source.split('\n');
  const html = [];
  let paragraph = [];
  let listItems = [];
  let listType = '';
  let blockquote = [];
  let inCodeBlock = false;
  let codeLines = [];
  let tableRows = [];

  const parseTableCells = (line) => {
    return line.replace(/^\||\|$/g, '').split('|').map((c) => c.trim());
  };

  const isTableSeparator = (line) => /^\|?[\s:|-]+\|[\s:|-]*(\|[\s:|-]*)*\|?$/.test(line.trim());

  const flushTable = () => {
    if (!tableRows.length) return;
    let thead = '';
    let tbody = '';
    let headerDone = false;
    for (const row of tableRows) {
      if (row.isSeparator) {
        headerDone = true;
        continue;
      }
      const tag = !headerDone ? 'th' : 'td';
      const cells = row.cells.map((c) => `<${tag} class="md-td">${renderMarkdownInline(c)}</${tag}>`).join('');
      const tr = `<tr class="md-tr">${cells}</tr>`;
      if (!headerDone) {
        thead += tr;
      } else {
        tbody += tr;
      }
    }
    let tableHtml = '<div class="md-table-wrap"><table class="md-table">';
    if (thead) tableHtml += `<thead>${thead}</thead>`;
    if (tbody) tableHtml += `<tbody>${tbody}</tbody>`;
    tableHtml += '</table></div>';
    html.push(tableHtml);
    tableRows = [];
  };

  const flushParagraph = () => {
    if (!paragraph.length) {
      return;
    }
    html.push(`<p class="md-paragraph">${renderMarkdownInline(paragraph.join(' '))}</p>`);
    paragraph = [];
  };

  const flushList = () => {
    if (!listItems.length) {
      return;
    }
    const tag = listType === 'ol' ? 'ol' : 'ul';
    html.push(`<${tag} class="md-list">${listItems.map((item) => `<li>${renderMarkdownInline(item)}</li>`).join('')}</${tag}>`);
    listItems = [];
    listType = '';
  };

  const flushBlockquote = () => {
    if (!blockquote.length) {
      return;
    }
    html.push(`<blockquote class="md-blockquote">${blockquote.map((item) => `<p>${renderMarkdownInline(item)}</p>`).join('')}</blockquote>`);
    blockquote = [];
  };

  const flushCodeBlock = () => {
    if (!codeLines.length) {
      html.push('<pre class="md-code-block"><code></code></pre>');
    } else {
      html.push(`<pre class="md-code-block"><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`);
    }
    codeLines = [];
  };

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();
    const trimmed = line.trim();

    if (trimmed.startsWith('```')) {
      flushParagraph();
      flushList();
      flushBlockquote();
      flushTable();
      if (inCodeBlock) {
        flushCodeBlock();
        inCodeBlock = false;
      } else {
        inCodeBlock = true;
      }
      continue;
    }

    if (inCodeBlock) {
      codeLines.push(rawLine);
      continue;
    }

    if (!trimmed) {
      flushParagraph();
      flushList();
      flushBlockquote();
      flushTable();
      continue;
    }

    // Table row detection
    if (trimmed.startsWith('|') || trimmed.endsWith('|')) {
      flushParagraph();
      flushList();
      flushBlockquote();
      if (isTableSeparator(trimmed)) {
        tableRows.push({ isSeparator: true });
      } else {
        tableRows.push({ cells: parseTableCells(trimmed), isSeparator: false });
      }
      continue;
    }
    flushTable();

    const headingMatch = trimmed.match(/^(#{1,6})\s+(.+)$/);
    if (headingMatch) {
      flushParagraph();
      flushList();
      flushBlockquote();
      const level = Math.min(headingMatch[1].length, 6);
      html.push(`<h${level} class="md-heading md-heading-${level}">${renderMarkdownInline(headingMatch[2])}</h${level}>`);
      continue;
    }

    if (/^(-{3,}|\*{3,}|_{3,})$/.test(trimmed)) {
      flushParagraph();
      flushList();
      flushBlockquote();
      html.push('<hr class="md-divider">');
      continue;
    }

    const blockquoteMatch = trimmed.match(/^>\s?(.*)$/);
    if (blockquoteMatch) {
      flushParagraph();
      flushList();
      blockquote.push(blockquoteMatch[1]);
      continue;
    }
    flushBlockquote();

    const unorderedMatch = trimmed.match(/^[-*+]\s+(.+)$/);
    if (unorderedMatch) {
      flushParagraph();
      if (listType && listType !== 'ul') {
        flushList();
      }
      listType = 'ul';
      listItems.push(unorderedMatch[1]);
      continue;
    }

    const orderedMatch = trimmed.match(/^\d+\.\s+(.+)$/);
    if (orderedMatch) {
      flushParagraph();
      if (listType && listType !== 'ol') {
        flushList();
      }
      listType = 'ol';
      listItems.push(orderedMatch[1]);
      continue;
    }

    flushList();
    paragraph.push(trimmed);
  }

  if (inCodeBlock) {
    flushCodeBlock();
  }
  flushParagraph();
  flushList();
  flushBlockquote();
  flushTable();

  return html.join('');
}

function renderMarkdownInline(value) {
  const placeholders = [];
  let html = escapeHtml(String(value || ''));

  html = html.replace(/`([^`]+)`/g, (_, content) => {
    const token = `__MD_CODE_${placeholders.length}__`;
    placeholders.push(`<code class="md-inline-code">${escapeHtml(content)}</code>`);
    return token;
  });

  html = html.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, (_, label, url) => {
    const safeURL = escapeHtml(url);
    return `<a class="md-link" href="${safeURL}" target="_blank" rel="noreferrer noopener">${escapeHtml(label)}</a>`;
  });
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/(^|[\s(])\*([^*]+)\*(?=$|[\s).,!?:;])/g, '$1<em>$2</em>');

  placeholders.forEach((replacement, index) => {
    html = html.replace(`__MD_CODE_${index}__`, replacement);
  });

  return html;
}
