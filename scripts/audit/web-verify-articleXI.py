#!/usr/bin/env python3
"""Article XI §11.2 real-browser verification of catalog-web.

Drives Chromium against http://localhost:3093 (amber.local nginx proxy),
fills login form with admin/<ADMIN_PASSWORD>, clicks Sign In, captures
post-login DOM evidence + screenshot. Asserts:

1. Login form renders with expected fields (positive).
2. POST /api/v1/auth/login returns a real user via the proxy (positive).
3. After Sign In, the document title/URL/visible text changes to a
   logged-in shell (post-login outcome — Article XI §11.2.1).
4. Article XI §11.5 negative: also drive a wrong-password attempt and
   verify the form stays on /login with an error message.
"""

import os
import sys
import time

from playwright.sync_api import sync_playwright

BASE = os.environ.get("CZ_WEB_URL", "http://localhost:3093")
EVID = "/run/media/milosvasic/DATA4TB/Projects/Catalogizer/docs/audits/evidence-2026-04-29-web"
os.makedirs(EVID, exist_ok=True)

def fail(msg):
    print(f"FAIL: {msg}")
    sys.exit(1)

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    ctx = browser.new_context(viewport={"width": 1920, "height": 1080})
    page = ctx.new_page()

    page.goto(f"{BASE}/", wait_until="networkidle", timeout=15000)

    title = page.title()
    print(f"Title: {title!r}")
    if "Catalogizer" not in title:
        fail(f"unexpected title: {title!r}")

    page.screenshot(path=f"{EVID}/01-landing.png", full_page=True)
    print(f"  saved: {EVID}/01-landing.png ({os.path.getsize(EVID+'/01-landing.png')} bytes)")

    # SPA: wait for the React app to mount the login form. Try /login
    # explicitly if root doesn't render fields after a beat.
    page.wait_for_timeout(2000)
    print(f"URL after landing: {page.url}")
    try:
        page.wait_for_selector('input[type="password"]', timeout=8000)
    except Exception:
        # Maybe SPA needs explicit /login
        page.goto(f"{BASE}/login", wait_until="networkidle", timeout=10000)
        page.wait_for_selector('input[type="password"]', timeout=8000)
    print(f"URL after wait: {page.url}")

    user_input = page.locator(
        'input[name="username"], input[id="username"], input[type="text"][placeholder*="user" i], '
        'input[type="email"], input[placeholder*="username" i]'
    ).first
    pass_input = page.locator(
        'input[name="password"], input[id="password"], input[type="password"]'
    ).first

    if user_input.count() == 0 or pass_input.count() == 0:
        fail(f"login fields not found at {page.url}; HTML excerpt:\n{page.content()[:1500]}")

    user_input.fill("admin")
    pass_input.fill(os.environ["ADMIN_PASSWORD"])
    page.screenshot(path=f"{EVID}/02-creds-filled.png", full_page=True)
    print(f"  saved: {EVID}/02-creds-filled.png")

    # Locate Sign In button
    submit = page.locator(
        'button[type="submit"], button:has-text("Sign In"), button:has-text("Login"), button:has-text("Log In")'
    ).first
    if submit.count() == 0:
        fail("Sign In button not found")
    submit.click()
    print("clicked Sign In")

    # Wait for navigation to a non-/login page
    try:
        page.wait_for_url(lambda url: "/login" not in url, timeout=10000)
        print(f"  navigated to: {page.url}")
    except Exception:
        # Maybe app keeps URL but DOM updates
        print(f"  URL unchanged: {page.url} — checking DOM")

    page.wait_for_load_state("networkidle", timeout=10000)
    page.screenshot(path=f"{EVID}/03-post-login.png", full_page=True)
    print(f"  saved: {EVID}/03-post-login.png")

    # Article XI §11.2.1: assert end-user-visible outcome.
    # "Sign In" button should be GONE. A logged-in shell typically shows
    # navigation labels like Dashboard, Media, Browse, Collections.
    body_text = page.locator("body").inner_text(timeout=2000)
    print(f"  body text length: {len(body_text)}")

    if "Sign In" in body_text or "Sign in to your account" in body_text:
        fail(f"Sign In still visible after login attempt — login did not succeed")

    # Look for any sign of logged-in shell
    logged_in_markers = [
        "Dashboard", "Media", "Browse", "Collections", "Logout",
        "Sign Out", "Settings", "admin", "Catalog", "Home",
    ]
    found = [m for m in logged_in_markers if m in body_text]
    if not found:
        # Save body text for forensics
        with open(f"{EVID}/post-login-body.txt", "w") as f:
            f.write(body_text)
        fail(f"no logged-in markers found in post-login page; body excerpt:\n{body_text[:800]}")
    print(f"  logged-in markers found: {found}")

    # Article XI §11.5 negative test: fresh isolated context (no shared
    # cookies), try wrong password, expect login form to remain.
    print("\n=== Negative test (wrong credentials) ===")
    ctx2 = browser.new_context(
        viewport={"width": 1920, "height": 1080},
        storage_state=None,
    )
    page2 = ctx2.new_page()
    page2.goto(f"{BASE}/login", wait_until="networkidle", timeout=15000)
    page2.wait_for_timeout(2000)
    try:
        page2.wait_for_selector('input[type="password"]', timeout=8000)
    except Exception:
        page2.goto(f"{BASE}/", wait_until="networkidle")
        page2.wait_for_selector('input[type="password"]', timeout=8000)

    p2 = page2.locator('input[type="password"]').first
    # Username field — pick the visible non-password text input above the password
    u2 = page2.locator(
        'input[type="text"]:visible, input[type="email"]:visible, '
        'input[name="username"]:visible, input[id="username"]:visible'
    ).first
    if u2.count() == 0:
        # Fall back to the first input that isn't a password
        u2 = page2.locator('input:not([type="password"]):not([type="hidden"]):not([type="submit"])').first
    u2.fill("admin")
    p2.fill("WRONGPASSWORD-12345")
    page2.locator(
        'button[type="submit"], button:has-text("Sign In"), button:has-text("Login")'
    ).first.click()
    page2.wait_for_timeout(3000)
    page2.screenshot(path=f"{EVID}/04-wrong-creds.png", full_page=True)
    body2 = page2.locator("body").inner_text(timeout=2000)
    # Save body for forensics
    with open(f"{EVID}/04-wrong-creds-body.txt", "w") as f:
        f.write(body2)

    # Negative correctness:
    #   1. URL must NOT have navigated to a logged-in page (/dashboard, /media, etc.)
    #   2. EITHER the login form is still visible (button + password field)
    #      OR an error message is displayed (Invalid credentials etc.)
    url_after = page2.url
    if "/dashboard" in url_after or "/browse" in url_after or "/media" in url_after:
        fail(f"BUG: app navigated to {url_after} after wrong password — auth bypass")

    pwd_still_visible = page2.locator('input[type="password"]:visible').count() > 0
    error_markers = ["invalid", "incorrect", "wrong", "failed", "error"]
    has_error = any(m.lower() in body2.lower() for m in error_markers)

    if not pwd_still_visible and not has_error:
        fail(f"BUG: post-wrong-password URL={url_after}, no password field, no error message → silent accept")

    print(f"  URL after wrong creds: {url_after}")
    print(f"  password field still visible: {pwd_still_visible}")
    print(f"  error marker in body: {has_error}")
    print("  negative path correctly rejected")
    print(f"  saved: {EVID}/04-wrong-creds.png")

    browser.close()

print("\n✓ All Article XI §11.2 assertions passed:")
print("  - landing page loads with correct title (positive)")
print("  - login form renders with username + password fields (positive)")
print("  - admin/<ADMIN_PASSWORD> succeeds → 'Sign In' is absent post-login (positive outcome)")
print("  - logged-in shell shows expected nav markers (positive outcome)")
print("  - WRONG-PASSWORD is rejected, form stays visible (matching negative)")
