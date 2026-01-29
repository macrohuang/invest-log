#!/usr/bin/env python3
"""
One-time helper to repair symbols.asset_type after migration.

Usage:
  python3 scripts/repair_asset_types.py --export /tmp/asset_types.csv
  # Edit the CSV's asset_type column
  python3 scripts/repair_asset_types.py --apply /tmp/asset_types.csv
"""

from __future__ import annotations

import argparse
import csv
import sqlite3
import sys
from pathlib import Path
from typing import Iterable

ROOT_DIR = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT_DIR))

import config  # noqa: E402


CONF_SCORE = {"low": 1, "medium": 2, "high": 3}


def _contains_any(text: str, keywords: Iterable[str]) -> tuple[bool, str]:
    for kw in keywords:
        if kw and kw in text:
            return True, kw
    return False, ""


def _load_asset_types(cursor: sqlite3.Cursor) -> set[str]:
    rows = cursor.execute("SELECT code FROM asset_types").fetchall()
    return {r[0].lower() for r in rows}


def _collect_text(cursor: sqlite3.Cursor, symbol_id: int) -> str:
    notes = [
        r[0]
        for r in cursor.execute(
            "SELECT notes FROM transactions WHERE symbol_id = ? AND notes IS NOT NULL AND TRIM(notes) != ''",
            (symbol_id,),
        ).fetchall()
    ]
    tags = [
        r[0]
        for r in cursor.execute(
            "SELECT tags FROM transactions WHERE symbol_id = ? AND tags IS NOT NULL AND TRIM(tags) != ''",
            (symbol_id,),
        ).fetchall()
    ]
    return " ".join(notes + tags)


def _sample_note(cursor: sqlite3.Cursor, symbol_id: int) -> str:
    row = cursor.execute(
        """
        SELECT notes
        FROM transactions
        WHERE symbol_id = ? AND notes IS NOT NULL AND TRIM(notes) != ''
        ORDER BY transaction_date DESC, id DESC
        LIMIT 1
        """,
        (symbol_id,),
    ).fetchone()
    return row[0] if row else ""


def _sample_tags(cursor: sqlite3.Cursor, symbol_id: int) -> str:
    row = cursor.execute(
        """
        SELECT tags
        FROM transactions
        WHERE symbol_id = ? AND tags IS NOT NULL AND TRIM(tags) != ''
        ORDER BY transaction_date DESC, id DESC
        LIMIT 1
        """,
        (symbol_id,),
    ).fetchone()
    return row[0] if row else ""


def _suggest_asset_type(text: str, symbol: str, available: set[str]) -> tuple[str, str, str]:
    text_upper = text.upper()
    symbol_upper = symbol.strip().upper()

    candidates: list[tuple[str, str, str]] = []

    def add(code: str, confidence: str, reason: str) -> None:
        if code in available:
            candidates.append((code, confidence, reason))

    # ASCII-only keyword lists (Unicode escapes for Chinese)
    cash_strong = [
        "\u73b0\u91d1",  # 现金
        "\u4f59\u989d",  # 余额
        "\u8d27\u5e01",  # 货币
        "\u5b58\u6b3e",  # 存款
        "\u6d3b\u671f",  # 活期
    ]
    cash_weak = [
        "\u95f2\u94b1",  # 闲钱
        "\u7406\u8d22",  # 理财
    ]
    bond_keywords = [
        "\u503a\u5238",  # 债券
        "\u56fd\u503a",  # 国债
    ]
    insurance_keywords = [
        "\u4fdd\u9669",  # 保险
    ]
    metal_keywords = [
        "\u9ec4\u91d1",  # 黄金
        "\u767d\u94f6",  # 白银
        "\u8d35\u91d1\u5c5e",  # 贵金属
    ]
    fund_keywords = [
        "\u57fa\u91d1",  # 基金
        "\u8054\u63a5",  # 联接
        "QDII",
    ]

    if symbol_upper == "CASH":
        add("cash", "high", "symbol=CASH")

    hit, kw = _contains_any(text, cash_strong)
    if hit:
        add("cash", "high", f"notes contain cash keyword: {kw}")

    hit, kw = _contains_any(text, cash_weak)
    if hit:
        add("cash", "medium", f"notes contain cash keyword: {kw}")

    if "ETF" in text_upper:
        add("etf", "high", "notes contain ETF")

    hit, kw = _contains_any(text, metal_keywords)
    if hit:
        add("metal", "medium", f"notes contain metal keyword: {kw}")

    hit, kw = _contains_any(text, bond_keywords)
    if hit:
        add("bond", "medium", f"notes contain bond keyword: {kw}")

    hit, kw = _contains_any(text, insurance_keywords)
    if hit:
        add("insurance", "high", f"notes contain insurance keyword: {kw}")

    hit, kw = _contains_any(text, fund_keywords)
    if hit:
        if "fund" in available:
            add("fund", "low", f"notes contain fund keyword: {kw}")
        elif "etf" in available:
            add("etf", "low", f"notes contain fund keyword: {kw} (mapped to etf)")

    if not candidates:
        return "", "", ""

    # Prefer certain categories when multiple matches exist.
    precedence = ["cash", "insurance", "bond", "metal", "fund", "etf", "stock"]
    prec_index = {code: idx for idx, code in enumerate(precedence)}

    best_code = None
    best_conf = -1
    best_prec = 999
    reasons: dict[str, list[str]] = {}
    conf_by_code: dict[str, int] = {}

    for code, conf, reason in candidates:
        score = CONF_SCORE.get(conf, 1)
        if code not in conf_by_code or score > conf_by_code[code]:
            conf_by_code[code] = score
            reasons[code] = [reason]
        elif score == conf_by_code[code]:
            reasons[code].append(reason)

    for code, score in conf_by_code.items():
        prec = prec_index.get(code, 999)
        if prec < best_prec or (prec == best_prec and score > best_conf):
            best_code = code
            best_conf = score
            best_prec = prec

    if not best_code:
        return "", "", ""

    confidence = "low"
    for label, score in CONF_SCORE.items():
        if score == best_conf:
            confidence = label
            break

    reason = "; ".join(reasons.get(best_code, []))
    return best_code, confidence, reason


def export_csv(csv_path: str) -> int:
    con = sqlite3.connect(config.DB_PATH)
    con.row_factory = sqlite3.Row
    cur = con.cursor()

    available_types = _load_asset_types(cur)
    if not available_types:
        print("No asset_types found in DB. Aborting.")
        return 1

    rows = cur.execute("SELECT id, symbol, asset_type FROM symbols ORDER BY symbol").fetchall()
    with open(csv_path, "w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(
            [
                "symbol_id",
                "symbol",
                "current_asset_type",
                "asset_type",
                "suggested_asset_type",
                "confidence",
                "reason",
                "sample_note",
                "sample_tags",
            ]
        )
        for r in rows:
            text = _collect_text(cur, r["id"])
            suggested, confidence, reason = _suggest_asset_type(text, r["symbol"], available_types)
            target = suggested or (r["asset_type"] or "")
            writer.writerow(
                [
                    r["id"],
                    r["symbol"],
                    r["asset_type"] or "",
                    target,
                    suggested,
                    confidence,
                    reason,
                    _sample_note(cur, r["id"]),
                    _sample_tags(cur, r["id"]),
                ]
            )

    print(f"Exported {len(rows)} symbols to {csv_path}")
    return 0


def apply_csv(csv_path: str, dry_run: bool) -> int:
    con = sqlite3.connect(config.DB_PATH)
    con.row_factory = sqlite3.Row
    cur = con.cursor()

    available_types = _load_asset_types(cur)
    if not available_types:
        print("No asset_types found in DB. Aborting.")
        return 1

    updates = []
    with open(csv_path, newline="") as f:
        reader = csv.DictReader(f)
        required_cols = {"symbol_id", "asset_type"}
        if not required_cols.issubset(reader.fieldnames or []):
            print(f"CSV missing required columns: {sorted(required_cols)}")
            return 1

        for row in reader:
            symbol_id = row.get("symbol_id", "").strip()
            asset_type = row.get("asset_type", "").strip().lower()
            if not symbol_id or not asset_type:
                continue
            if asset_type not in available_types:
                print(f"Skip id={symbol_id}: invalid asset_type '{asset_type}'")
                continue
            updates.append((asset_type, int(symbol_id)))

    if not updates:
        print("No updates to apply.")
        return 0

    # Filter out no-op updates
    filtered = []
    for asset_type, symbol_id in updates:
        row = cur.execute("SELECT asset_type FROM symbols WHERE id = ?", (symbol_id,)).fetchone()
        if not row:
            print(f"Skip id={symbol_id}: not found")
            continue
        current = (row[0] or "").lower()
        if current == asset_type:
            continue
        filtered.append((asset_type, symbol_id, current))

    if not filtered:
        print("No changes after filtering.")
        return 0

    if dry_run:
        print("Dry run: would apply these updates:")
        for asset_type, symbol_id, current in filtered:
            print(f"- id={symbol_id}: {current} -> {asset_type}")
        return 0

    cur.executemany("UPDATE symbols SET asset_type = ? WHERE id = ?", [(a, i) for a, i, _ in filtered])
    con.commit()
    print(f"Applied {len(filtered)} updates.")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Repair symbols.asset_type via CSV workflow.")
    parser.add_argument("--export", dest="export_path", help="Export CSV to this path")
    parser.add_argument("--apply", dest="apply_path", help="Apply CSV from this path")
    parser.add_argument("--dry-run", action="store_true", help="Preview updates when using --apply")
    args = parser.parse_args()

    if not args.export_path and not args.apply_path:
        parser.print_help()
        return 1

    if args.export_path:
        return export_csv(args.export_path)

    return apply_csv(args.apply_path, args.dry_run)


if __name__ == "__main__":
    raise SystemExit(main())
