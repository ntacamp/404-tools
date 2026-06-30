#!/usr/bin/env python3
import io
import json
import os
import re
import time

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
CACHE_TTL = 3600  # seconds
USERS_CACHE_FILE = "users.json"
GITHUB_API = "https://api.github.com"
GITHUB_GRAPHQL = "https://api.github.com/graphql"
PR_BASE_URL = "https://github.com/ntacamp/404.notrollsallowed.com/pull/"
OUTPUT_FILE = "labels.pdf"

LABELS_PER_PAGE = 5
PAGE_W, PAGE_H = A4  # 595.28 x 841.89 pt
LABEL_H = PAGE_H / LABELS_PER_PAGE
PAD = 14  # horizontal padding
QR_SIZE = int(LABEL_H) - PAD * 2  # square, fits within label height
TEXT_W = PAGE_W - PAD * 2 - QR_SIZE - 10  # 10 pt gap before QR


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
    """Fetch GitHub real names for all @username presenters not yet cached."""
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
        cache[nick] = name  # None stored as sentinel to avoid re-fetching
    print(f"Fetched {len(nicks)} GitHub user(s) via GraphQL.")


def _fetch_names_rest(nicks, cache):
    print(f"No GITHUB_TOKEN set — fetching {len(nicks)} user(s) one by one (REST).")
    for nick in nicks:
        resp = requests.get(f"{GITHUB_API}/users/{nick}", timeout=15)
        if resp.status_code == 200:
            name = (resp.json().get("name") or "").strip() or None
            cache[nick] = name  # None stored as sentinel to avoid re-fetching
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

    # QR code
    qr_buf = make_qr_image(url, QR_SIZE)
    qr_x = PAGE_W - PAD - QR_SIZE
    qr_y = y_bottom + (LABEL_H - QR_SIZE) / 2
    c.drawImage(
        ImageReader(qr_buf),
        qr_x, qr_y,
        width=QR_SIZE, height=QR_SIZE,
        preserveAspectRatio=True,
    )

    # Text block: title (bold), author, PR number
    title_style = ParagraphStyle(
        "title",
        fontName=FONT_BOLD,
        fontSize=16,
        leading=20,
        wordWrap="LTR",
    )
    title_para = Paragraph(title, title_style)
    tw, th = title_para.wrap(TEXT_W, LABEL_H)

    # Vertical centre: stack title + gap + two lines of meta
    line_h = 14  # height per meta line
    title_gap = 12  # spacing between title and author
    block_h = th + title_gap + line_h + line_h
    text_y_top = y_bottom + (LABEL_H + block_h) / 2  # top of title paragraph

    title_para.drawOn(c, PAD, text_y_top - th)

    meta_y = text_y_top - th - title_gap
    c.setFont(FONT, 12)
    c.setFillColor(colors.black)
    c.drawString(PAD, meta_y, presentation["display_author"])
    c.drawString(PAD, meta_y - line_h, f"PR{pr_num}")

    # Separator line at the top of this label
    c.setStrokeColor(colors.lightgrey)
    c.setLineWidth(0.5)
    c.line(0, y_bottom + LABEL_H, PAGE_W, y_bottom + LABEL_H)


def generate_pdf(presentations, output_path):
    c = canvas.Canvas(output_path, pagesize=A4)
    total = len(presentations)
    pages = (total + LABELS_PER_PAGE - 1) // LABELS_PER_PAGE

    for page_idx in range(pages):
        batch = presentations[page_idx * LABELS_PER_PAGE:(page_idx + 1) * LABELS_PER_PAGE]
        for slot, pres in enumerate(batch):
            # slot 0 = top of page
            y_bottom = PAGE_H - (slot + 1) * LABEL_H
            draw_label(c, pres, y_bottom)
        if page_idx < pages - 1:
            c.showPage()

    c.save()
    print(f"Parsed {total} presentations. Written {output_path} ({pages} page(s)).")


def main():
    text = fetch_404_txt()
    presentations = parse_presentations(text)
    if not presentations:
        print("No presentations found.")
        return
    presentations = enrich_with_real_names(presentations)
    generate_pdf(presentations, OUTPUT_FILE)


if __name__ == "__main__":
    main()
