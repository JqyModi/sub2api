#!/usr/bin/env python3
"""Send one idempotent, opt-out-aware reactivation campaign from the OCI host.

Run from /opt/sub2api after deploying the matching application version. The
script reads SMTP settings and users through the local postgres container, so
credentials never need to be copied into a shell command or source file.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import hmac
import json
import smtplib
import subprocess
import sys
import time
import secrets
from datetime import datetime, timedelta, timezone
from email.message import EmailMessage
from email.utils import formataddr
from html import escape
from urllib.parse import quote


CAMPAIGN_ID = "subscription-growth-v020-reactivation"
EVENT = "growth.reactivation"
SOURCE_TYPE = "growth_campaign"
ACTIVITY_URL = "https://jqymodi.github.io/codex-multi-launcher/subscription.html#activity"
PURCHASE_URL = "https://sub2api.minai.eu.org/purchase?tab=subscription"
DEFAULT_CUTOFF = "2026-08-16T00:00:00+08:00"


def psql(query: str) -> str:
    command = [
        "docker", "exec", "sub2api-postgres", "psql", "-U", "sub2api",
        "-d", "sub2api", "-At", "-F", "\t", "-c", query,
    ]
    return subprocess.run(command, check=True, text=True, capture_output=True).stdout


def setting_values(keys: list[str]) -> dict[str, str]:
    quoted = ",".join("'" + key.replace("'", "''") + "'" for key in keys)
    rows = psql(f"SELECT key, value FROM settings WHERE key IN ({quoted})")
    return {
        key: value
        for row in rows.splitlines() if "\t" in row
        for key, value in [row.split("\t", 1)]
    }


def email_hash(value: str) -> str:
    return hashlib.sha256(value.strip().lower().encode()).hexdigest()


def preference_key(email: str) -> str:
    return "notification_email_preference:v2:" + email_hash(EVENT + "\x00" + email)


def delivery_key(email: str) -> str:
    identity = "\x00".join([EVENT, SOURCE_TYPE, CAMPAIGN_ID, email.strip().lower(), ""])
    return "notification_email_delivery:v2:" + email_hash(identity)


def base64url(value: bytes) -> str:
    return base64.urlsafe_b64encode(value).decode().rstrip("=")


def unsubscribe_url(base_url: str, secret: str, email: str) -> str:
    claims = {"email": email.strip(), "event": EVENT, "exp": int((datetime.now(timezone.utc) + timedelta(days=365)).timestamp())}
    payload = base64url(json.dumps(claims, separators=(",", ":")).encode())
    signature = base64url(hmac.new(secret.encode(), payload.encode(), hashlib.sha256).digest())
    return base_url.rstrip("/") + "/api/v1/settings/email-unsubscribe?token=" + quote(payload + "." + signature)


def eligible_users(cutoff: str) -> list[tuple[int, str, str]]:
    normalized = datetime.fromisoformat(cutoff).isoformat()
    query = """
        SELECT id, email, COALESCE(username, '')
        FROM users
        WHERE role <> 'admin'
          AND status = 'active'
          AND deleted_at IS NULL
          AND created_at < TIMESTAMPTZ '%s'
        ORDER BY id
    """ % normalized.replace("'", "''")
    return [
        (int(parts[0]), parts[1], parts[2])
        for row in psql(query).splitlines() if (parts := row.split("\t")) and len(parts) == 3
    ]


def send_message(settings: dict[str, str], recipient: str, recipient_name: str, unsubscribe: str) -> None:
    site_name = settings.get("site_name") or "Codex Multi Launcher"
    subject = f"[{site_name}] 老用户福利已升级：同价加量，邀请再送 $20"
    name = recipient_name.strip() or recipient.split("@", 1)[0]
    plain = f"""{name}，你好：

订阅服务近期更新了福利：
1. 新用户完成邮箱验证后，可领取 3 天 $20 体验订阅。
2. 邀请好友完成首笔订阅后，你可获得 30 天 $20 奖励订阅。
3. 全部订阅套餐已同价加量。

你的现有账号无需重新注册。
活动说明：{ACTIVITY_URL}
订阅服务：{PURCHASE_URL}
不再接收活动更新：{unsubscribe}
"""
    html = f"""<html><body style=\"font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;line-height:1.65;color:#1f2937\">
<h2 style=\"color:#6d28d9\">老用户福利更新</h2>
<p>{escape(name)}，你好：</p>
<p>订阅服务近期更新了福利，特意同步给你：</p>
<ul><li>新用户完成邮箱验证后，可领取 3 天 $20 体验订阅。</li><li>邀请好友：好友完成首笔订阅后，你可获得 30 天 $20 奖励订阅。</li><li>全部订阅套餐已同价加量。</li></ul>
<p>你的现有账号无需重新注册；可查看活动说明、邀请好友，或直接继续使用订阅服务。</p>
<p><a href=\"{ACTIVITY_URL}\" style=\"display:inline-block;padding:10px 16px;background:#6d28d9;color:#fff;text-decoration:none;border-radius:6px\">查看福利与使用指引</a></p>
<p style=\"font-size:12px;color:#6b7280\"><a href=\"{PURCHASE_URL}\">前往订阅服务</a> · <a href=\"{unsubscribe}\">不再接收活动更新</a></p>
</body></html>"""
    message = EmailMessage()
    message["Subject"] = subject
    message["From"] = formataddr((settings.get("smtp_from_name") or site_name, settings["smtp_from"]))
    message["To"] = recipient
    message.set_content(plain)
    message.add_alternative(html, subtype="html")

    host = settings["smtp_host"]
    port = int(settings.get("smtp_port") or "587")
    username = settings.get("smtp_username", "")
    password = settings.get("smtp_password", "")
    if settings.get("smtp_use_tls", "false").lower() == "true":
        client = smtplib.SMTP_SSL(host, port, timeout=30)
    else:
        client = smtplib.SMTP(host, port, timeout=30)
        client.ehlo()
        if client.has_extn("starttls"):
            client.starttls()
            client.ehlo()
    try:
        if username:
            client.login(username, password)
        client.send_message(message)
    finally:
        client.quit()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--execute", action="store_true", help="Send instead of only printing the eligible count")
    parser.add_argument("--before", default=DEFAULT_CUTOFF, help="ISO 8601 activity start cutoff")
    parser.add_argument("--delay", type=float, default=1.0, help="Seconds between deliveries")
    args = parser.parse_args()
    try:
        datetime.fromisoformat(args.before)
    except ValueError as exc:
        parser.error(f"--before must be ISO 8601: {exc}")

    required = ["smtp_host", "smtp_from", "smtp_password", "frontend_url"]
    settings = setting_values(required + ["smtp_port", "smtp_username", "smtp_from_name", "smtp_use_tls", "site_name"])
    missing = [key for key in required if not settings.get(key)]
    if missing:
        raise RuntimeError("required production settings are missing: " + ", ".join(missing))
    secret = setting_values(["notification_email_unsubscribe_secret"]).get("notification_email_unsubscribe_secret", "")
    if not secret:
        secret = secrets.token_urlsafe(32)
        escaped = secret.replace("'", "''")
        psql("INSERT INTO settings (key, value, updated_at) VALUES ('notification_email_unsubscribe_secret', '%s', now()) ON CONFLICT (key) DO NOTHING" % escaped)
        secret = setting_values(["notification_email_unsubscribe_secret"]).get("notification_email_unsubscribe_secret", "")
    if not secret:
        raise RuntimeError("could not initialize notification email unsubscribe secret")
    settings["notification_email_unsubscribe_secret"] = secret

    users = eligible_users(args.before)
    preference_keys = [preference_key(email) for _, email, _ in users]
    delivery_keys = [delivery_key(email) for _, email, _ in users]
    excluded = setting_values(preference_keys + delivery_keys)
    pending = [
        (user_id, email, name) for user_id, email, name in users
        if excluded.get(preference_key(email), "").lower() != "unsubscribed" and delivery_key(email) not in excluded
    ]
    print(f"campaign={CAMPAIGN_ID} eligible={len(users)} pending={len(pending)} skipped={len(users) - len(pending)}")
    if not args.execute:
        return 0

    sent = 0
    for _, email, name in pending:
        send_message(settings, email, name, unsubscribe_url(settings["frontend_url"], settings["notification_email_unsubscribe_secret"], email))
        key = delivery_key(email).replace("'", "''")
        psql("INSERT INTO settings (key, value, updated_at) VALUES ('%s', 'sent', now()) ON CONFLICT (key) DO NOTHING" % key)
        sent += 1
        time.sleep(max(args.delay, 0))
    print(f"campaign={CAMPAIGN_ID} sent={sent}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as error:
        sys.stderr.write(error.stderr or str(error) + "\n")
        raise SystemExit(1)
