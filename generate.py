#!/usr/bin/env python3
import argparse
import io
import json
import os
import re
import time
from datetime import datetime, timedelta

import qrcode
import requests
from reportlab.lib import colors
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.utils import ImageReader
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen import canvas
from reportlab.platypus import Paragraph
from PIL import Image as PILImage

FONT = "DejaVuSans"
FONT_BOLD = "DejaVuSans-Bold"

pdfmetrics.registerFont(TTFont(FONT, "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"))
pdfmetrics.registerFont(TTFont(FONT_BOLD, "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"))

SOURCE_URL = "https://raw.githubusercontent.com/ntacamp/404.notrollsallowed.com/master/404.txt"
CACHE_FILE = "404_cache.txt"
CACHE_TTL = 3600
USERS_CACHE_FILE = "users.json"
GITHUB_API = "https://api.github.com"
GITHUB_GRAPHQL = "https://api.github.com/graphql"
PR_BASE_URL = "https://github.com/ntacamp/404.notrollsallowed.com/pull/"

LABELS_PER_PAGE = 5
PAGE_W, PAGE_H = A4
LABEL_H = PAGE_H / LABELS_PER_PAGE
PAD = 14
QR_SIZE = int(LABEL_H) - PAD * 2
TEXT_W = PAGE_W - PAD * 2 - QR_SIZE - 10


# ---------------------------------------------------------------------------
# Shared PDF helpers
# ---------------------------------------------------------------------------

def _draw_separator(c, y_bottom):
    c.setStrokeColor(colors.lightgrey)
    c.setLineWidth(0.5)
    c.line(0, y_bottom + LABEL_H, PAGE_W, y_bottom + LABEL_H)


def _paginate(c, items, draw_fn):
    pages = (len(items) + LABELS_PER_PAGE - 1) // LABELS_PER_PAGE
    for page_idx in range(pages):
        batch = items[page_idx * LABELS_PER_PAGE:(page_idx + 1) * LABELS_PER_PAGE]
        for slot, item in enumerate(batch):
            y_bottom = PAGE_H - (slot + 1) * LABEL_H
            draw_fn(c, item, y_bottom)
        if page_idx < pages - 1:
            c.showPage()
    return pages


# ---------------------------------------------------------------------------
# Labels subcommand
# ---------------------------------------------------------------------------

def fetch_404_txt():
    if os.path.exists(CACHE_FILE):
        age = time.time() - os.path.getmtime(CACHE_FILE)
        if age < CACHE_TTL:
            with open(CACHE_FILE, encoding="utf-8") as f:
                return f.read()
    resp = requests.get(SOURCE_URL, timeout=15)
    resp.raise_for_status()
    text = resp.text
    with open(CACHE_FILE, "w", encoding="utf-8") as f:
        f.write(text)
    return text


def parse_presentations(text):
    presentations = []
    in_section = False
    entry_re = re.compile(r"\s+PR(\d+)\s+(.+?)\s+\.{3,}\s+(\S+)\s*$")
    for line in text.splitlines():
        if "--[ PRESENTATIONS" in line or "--[ ACTIVITIES" in line:
            in_section = True
            continue
        if in_section and line.startswith("--["):
            in_section = False
        if not in_section:
            continue
        m = entry_re.match(line)
        if m:
            pr_num, title, author = m.group(1), m.group(2).strip(), m.group(3).strip()
            presentations.append({"pr": pr_num, "title": title, "author": author})
    return presentations


def load_users_cache():
    if os.path.exists(USERS_CACHE_FILE):
        with open(USERS_CACHE_FILE, encoding="utf-8") as f:
            return json.load(f)
    return {}


def save_users_cache(cache):
    with open(USERS_CACHE_FILE, "w", encoding="utf-8") as f:
        json.dump(cache, f, ensure_ascii=False, indent=2)


def enrich_with_real_names(presentations):
    cache = load_users_cache()

    github_nicks = [
        p["author"][1:]
        for p in presentations
        if p["author"].startswith("@") and p["author"][1:] not in cache
    ]

    if github_nicks:
        token = os.environ.get("GITHUB_TOKEN")
        if token:
            _fetch_names_graphql(github_nicks, token, cache)
        else:
            _fetch_names_rest(github_nicks, cache)
        save_users_cache(cache)

    for p in presentations:
        if p["author"].startswith("@"):
            nick = p["author"][1:]
            real_name = cache.get(nick)
            if real_name:
                p["display_author"] = f"{p['author']} ({real_name})"
            else:
                p["display_author"] = p["author"]
        else:
            p["display_author"] = p["author"]

    return presentations


def _fetch_names_graphql(nicks, token, cache):
    aliases = " ".join(
        f'u{i}: user(login: "{nick}") {{ name }}' for i, nick in enumerate(nicks)
    )
    query = f"{{ {aliases} }}"
    resp = requests.post(
        GITHUB_GRAPHQL,
        json={"query": query},
        headers={"Authorization": f"bearer {token}"},
        timeout=15,
    )
    resp.raise_for_status()
    data = resp.json().get("data", {})
    for i, nick in enumerate(nicks):
        user_data = data.get(f"u{i}") or {}
        name = (user_data.get("name") or "").strip() or None
        cache[nick] = name
    print(f"Fetched {len(nicks)} GitHub user(s) via GraphQL.")


def _fetch_names_rest(nicks, cache):
    print(f"No GITHUB_TOKEN set — fetching {len(nicks)} user(s) one by one (REST).")
    for nick in nicks:
        resp = requests.get(f"{GITHUB_API}/users/{nick}", timeout=15)
        if resp.status_code == 200:
            name = (resp.json().get("name") or "").strip() or None
            cache[nick] = name
        elif resp.status_code == 403:
            print("GitHub API rate limit reached; skipping remaining users.")
            break


def make_qr_image(url, size):
    qr = qrcode.QRCode(border=1, error_correction=qrcode.constants.ERROR_CORRECT_M)
    qr.add_data(url)
    qr.make(fit=True)
    img = qr.make_image(fill_color="black", back_color="white")
    img = img.resize((size, size), PILImage.LANCZOS)
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    buf.seek(0)
    return buf


def draw_label(c, presentation, y_bottom):
    pr_num = presentation["pr"]
    title = presentation["title"]
    url = PR_BASE_URL + pr_num

    qr_buf = make_qr_image(url, QR_SIZE)
    qr_x = PAGE_W - PAD - QR_SIZE
    qr_y = y_bottom + (LABEL_H - QR_SIZE) / 2
    c.drawImage(
        ImageReader(qr_buf),
        qr_x, qr_y,
        width=QR_SIZE, height=QR_SIZE,
        preserveAspectRatio=True,
    )

    title_style = ParagraphStyle(
        "title",
        fontName=FONT_BOLD,
        fontSize=16,
        leading=20,
        wordWrap="LTR",
    )
    title_para = Paragraph(title, title_style)
    tw, th = title_para.wrap(TEXT_W, LABEL_H)

    line_h = 14
    title_gap = 12
    block_h = th + title_gap + line_h + line_h
    text_y_top = y_bottom + (LABEL_H + block_h) / 2

    title_para.drawOn(c, PAD, text_y_top - th)

    meta_y = text_y_top - th - title_gap
    c.setFont(FONT, 12)
    c.setFillColor(colors.black)
    c.drawString(PAD, meta_y, presentation["display_author"])
    c.drawString(PAD, meta_y - line_h, f"PR{pr_num}")

    _draw_separator(c, y_bottom)


def cmd_labels(args):
    text = fetch_404_txt()
    presentations = parse_presentations(text)
    if not presentations:
        print("No presentations found.")
        return
    presentations = enrich_with_real_names(presentations)
    c = canvas.Canvas(args.output, pagesize=A4)
    pages = _paginate(c, presentations, draw_label)
    c.save()
    print(f"Parsed {len(presentations)} presentations. Written {args.output} ({pages} page(s)).")


# ---------------------------------------------------------------------------
# Times subcommand
# ---------------------------------------------------------------------------

def generate_time_slots(start_str, end_str):
    fmt = "%H:%M"
    current = datetime.strptime(start_str, fmt)
    end = datetime.strptime(end_str, fmt)
    if end <= current:
        end += timedelta(hours=24)
    slots = []
    while current <= end:
        slots.append(current.strftime(fmt))
        current += timedelta(minutes=30)
    return slots


def draw_time_label(c, time_str, y_bottom):
    max_w = PAGE_W - 2 * PAD
    max_h = LABEL_H - 2 * PAD

    size = 300
    while size > 10:
        w = pdfmetrics.stringWidth(time_str, FONT, size)
        if w <= max_w and size <= max_h:
            break
        size -= 1

    ascent = pdfmetrics.getFont(FONT).face.ascent * size / 1000

    w = pdfmetrics.stringWidth(time_str, FONT, size)
    x = (PAGE_W - w) / 2
    # digits/colon have no descenders, so visual extent is 0..ascent above baseline
    y = y_bottom + (LABEL_H - ascent) / 2

    c.setFont(FONT, size)
    c.setFillColor(colors.black)
    c.drawString(x, y, time_str)

    _draw_separator(c, y_bottom)


def cmd_times(args):
    slots = generate_time_slots(args.start, args.end)
    if not slots:
        print("No time slots generated (check --start and --end).")
        return
    c = canvas.Canvas(args.output, pagesize=A4)
    pages = _paginate(c, slots, draw_time_label)
    c.save()
    print(f"Generated {len(slots)} time slot(s). Written {args.output} ({pages} page(s)).")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(prog="generate.py")
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_labels = sub.add_parser("labels", help="Generate presentation labels")
    p_labels.add_argument("--output", default="labels.pdf")

    p_times = sub.add_parser("times", help="Generate time-slot labels")
    p_times.add_argument("--start", default="10:00", metavar="HH:MM")
    p_times.add_argument("--end", default="00:00", metavar="HH:MM")
    p_times.add_argument("--output", default="times.pdf")

    args = parser.parse_args()
    if args.cmd == "labels":
        cmd_labels(args)
    else:
        cmd_times(args)


if __name__ == "__main__":
    main()
