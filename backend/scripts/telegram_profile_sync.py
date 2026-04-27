#!/usr/bin/env python3
import argparse
import asyncio
import json
import os
import shutil
import tempfile
import zipfile
from datetime import datetime, timezone
from typing import Optional, Tuple

from opentele.api import API, UseCurrentSession
from opentele.td import TDesktop
from telethon import TelegramClient
from telethon.tl.functions.users import GetFullUserRequest
from telethon.tl.types import (
    UserStatusEmpty,
    UserStatusLastMonth,
    UserStatusLastWeek,
    UserStatusOffline,
    UserStatusOnline,
    UserStatusRecently,
)

from telegram_proxy import telethon_proxy_from_json, telethon_use_ipv6_from_json


TELEGRAM_CONNECT_TIMEOUT_SECONDS = 20
TELEGRAM_CONNECT_RETRIES = 1


def emit(payload: dict) -> None:
    print(json.dumps(payload, ensure_ascii=False))


def base_result() -> dict:
    return {
        "ok": False,
        "authorized": False,
        "phone": "",
        "nickname": "",
        "bio": "",
        "homepage": "",
        "avatar_checked": False,
        "avatar_present": False,
        "avatar_path": "",
        "avatar_error": "",
        "status": "abnormal",
        "last_online_at": None,
        "risk_status": "需重新导入",
        "ban_status": "正常",
        "reason": "",
        "source": "",
    }


def join_name(first_name: Optional[str], last_name: Optional[str]) -> str:
    return " ".join(part.strip() for part in [first_name or "", last_name or ""] if part and part.strip()).strip()


def pick_username(user) -> Optional[str]:
    username = getattr(user, "username", None)
    if username:
        return username
    usernames = getattr(user, "usernames", None) or []
    for item in usernames:
        value = getattr(item, "username", None)
        active = getattr(item, "active", True)
        if value and active:
            return value
    return None


def map_status(user) -> Tuple[str, Optional[str]]:
    status = getattr(user, "status", None)
    if isinstance(status, UserStatusOnline):
        return "online", datetime.now(timezone.utc).replace(microsecond=0).isoformat()
    if isinstance(status, UserStatusOffline):
        was_online = getattr(status, "was_online", None)
        if was_online is not None:
            return "offline", was_online.astimezone(timezone.utc).replace(microsecond=0).isoformat()
        return "offline", None
    if isinstance(status, (UserStatusRecently, UserStatusLastWeek, UserStatusLastMonth, UserStatusEmpty)) or status is None:
        return "offline", None
    return "offline", None


def map_risk_and_ban(user) -> Tuple[str, str]:
    ban_status = "正常"
    if getattr(user, "deleted", False):
        ban_status = "已注销"

    if getattr(user, "restricted", False):
        return "受限", ban_status
    if getattr(user, "scam", False) or getattr(user, "fake", False):
        return "高风险", ban_status
    if getattr(user, "bot", False):
        return "机器人", ban_status
    return "正常", ban_status


def resolve_tdata_root(base_dir: str) -> str:
    for root, _, files in os.walk(base_dir):
        if "key_data" in files:
            return root
        if "key_datas" in files:
            legacy_path = os.path.join(root, "key_datas")
            normalized_path = os.path.join(root, "key_data")
            try:
                with open(legacy_path, "rb") as src, open(normalized_path, "wb") as dst:
                    dst.write(src.read())
            except OSError:
                pass
            return root
    return base_dir


async def sync_avatar(client: TelegramClient, user, avatar_dir: str) -> Tuple[bool, bool, str, str]:
    target_dir = (avatar_dir or "").strip()
    if not target_dir:
        return False, False, "", ""

    os.makedirs(target_dir, exist_ok=True)
    photo = getattr(user, "photo", None)
    if photo is None or photo.__class__.__name__.endswith("Empty"):
        return True, False, "", ""
    try:
        downloaded = await client.download_profile_photo(user, file=bytes, download_big=True)
        if not downloaded:
            return True, False, "", ""
        avatar_path = os.path.join(target_dir, "telegram-avatar.jpg")
        with open(avatar_path, "wb") as handle:
            handle.write(downloaded)
        return True, True, os.path.abspath(avatar_path), ""
    except Exception as exc:
        return True, False, "", str(exc)


async def inspect_session(file_path: str, avatar_dir: str, proxy_config=None, use_ipv6: bool = False) -> dict:
    result = base_result()
    result["source"] = "session"

    client = TelegramClient(
        file_path,
        API.TelegramDesktop.api_id,
        API.TelegramDesktop.api_hash,
        device_model=API.TelegramDesktop.device_model,
        system_version=API.TelegramDesktop.system_version,
        app_version=API.TelegramDesktop.app_version,
        lang_code=API.TelegramDesktop.lang_code,
        system_lang_code=API.TelegramDesktop.system_lang_code,
        receive_updates=False,
        proxy=proxy_config,
        use_ipv6=use_ipv6,
        timeout=TELEGRAM_CONNECT_TIMEOUT_SECONDS,
        connection_retries=TELEGRAM_CONNECT_RETRIES,
        retry_delay=1,
        request_retries=TELEGRAM_CONNECT_RETRIES,
    )

    try:
        await asyncio.wait_for(client.connect(), timeout=TELEGRAM_CONNECT_TIMEOUT_SECONDS + 5)
        if not await client.is_user_authorized():
            result["reason"] = "会话未授权，需要重新登录"
            result["risk_status"] = "需重新登录"
            return result

        me = await client.get_me()
        if me is None:
            result["reason"] = "未读取到账号资料"
            return result

        full = await client(GetFullUserRequest(me))
        user = full.users[0] if getattr(full, "users", None) else me
        nickname = join_name(getattr(user, "first_name", None), getattr(user, "last_name", None))
        username = pick_username(user)
        status, last_online_at = map_status(user)
        risk_status, ban_status = map_risk_and_ban(user)
        avatar_checked, avatar_present, avatar_path, avatar_error = await sync_avatar(client, user, avatar_dir)

        result.update(
            {
                "ok": True,
                "authorized": True,
                "phone": getattr(user, "phone", None) or "",
                "nickname": nickname or (f"@{username}" if username else ""),
                "bio": getattr(full.full_user, "about", None) or "",
                "homepage": f"https://t.me/{username}" if username else "",
                "avatar_checked": avatar_checked,
                "avatar_present": avatar_present,
                "avatar_path": avatar_path,
                "avatar_error": avatar_error,
                "status": status,
                "last_online_at": last_online_at,
                "risk_status": risk_status,
                "ban_status": ban_status,
                "reason": "已同步 Telegram 资料",
            }
        )
        return result
    finally:
        await client.disconnect()


async def inspect_tdata(file_path: str, avatar_dir: str, proxy_config=None, use_ipv6: bool = False) -> dict:
    result = base_result()
    result["source"] = "tdata"

    if not zipfile.is_zipfile(file_path):
        result["reason"] = "当前 tdata 文件不完整，请重新导入完整 tdata 文件夹或 zip"
        return result

    temp_dir = tempfile.mkdtemp(prefix="codex3_tdata_sync_")
    client = None
    try:
        with zipfile.ZipFile(file_path) as archive:
            archive.extractall(temp_dir)

        tdata_root = resolve_tdata_root(temp_dir)
        desktop = TDesktop(tdata_root, api=API.TelegramDesktop)
        if not desktop.isLoaded() or desktop.accountsCount < 1:
            result["reason"] = "未从 tdata 中识别到账户，请重新导入完整目录"
            return result

        session_path = os.path.join(temp_dir, "telethon_sync")
        client = await desktop.ToTelethon(
            session=session_path,
            flag=UseCurrentSession,
            api=API.TelegramDesktop,
            receive_updates=False,
            use_ipv6=use_ipv6,
            timeout=TELEGRAM_CONNECT_TIMEOUT_SECONDS,
            connection_retries=TELEGRAM_CONNECT_RETRIES,
            retry_delay=1,
            request_retries=TELEGRAM_CONNECT_RETRIES,
        )
        if proxy_config:
            client.set_proxy(proxy_config)
        await asyncio.wait_for(client.connect(), timeout=TELEGRAM_CONNECT_TIMEOUT_SECONDS + 5)
        if not await client.is_user_authorized():
            result["reason"] = "tdata 会话未授权，需要重新导入"
            result["risk_status"] = "需重新导入"
            return result

        me = await client.get_me()
        if me is None:
            result["reason"] = "未读取到 tdata 账号资料"
            return result

        full = await client(GetFullUserRequest(me))
        user = full.users[0] if getattr(full, "users", None) else me
        nickname = join_name(getattr(user, "first_name", None), getattr(user, "last_name", None))
        username = pick_username(user)
        status, last_online_at = map_status(user)
        risk_status, ban_status = map_risk_and_ban(user)
        avatar_checked, avatar_present, avatar_path, avatar_error = await sync_avatar(client, user, avatar_dir)

        result.update(
            {
                "ok": True,
                "authorized": True,
                "phone": getattr(user, "phone", None) or "",
                "nickname": nickname or (f"@{username}" if username else ""),
                "bio": getattr(full.full_user, "about", None) or "",
                "homepage": f"https://t.me/{username}" if username else "",
                "avatar_checked": avatar_checked,
                "avatar_present": avatar_present,
                "avatar_path": avatar_path,
                "avatar_error": avatar_error,
                "status": status,
                "last_online_at": last_online_at,
                "risk_status": risk_status,
                "ban_status": ban_status,
                "reason": "已同步 Telegram 资料",
            }
        )
        return result
    finally:
        if client is not None:
            await client.disconnect()
        shutil.rmtree(temp_dir, ignore_errors=True)


async def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", required=True)
    parser.add_argument("--access-type", default="")
    parser.add_argument("--avatar-dir", default="")
    parser.add_argument("--proxy-json", default="")
    args = parser.parse_args()

    file_path = os.path.abspath(args.file)
    access_type = (args.access_type or "").strip().lower()
    avatar_dir = os.path.abspath(args.avatar_dir) if (args.avatar_dir or "").strip() else ""

    result = base_result()
    try:
        if not os.path.exists(file_path):
            result["reason"] = "本地会话文件不存在"
            emit(result)
            return 0

        proxy_config = telethon_proxy_from_json(args.proxy_json)
        use_ipv6 = telethon_use_ipv6_from_json(args.proxy_json)
        if access_type == "data":
            result = await inspect_tdata(file_path, avatar_dir, proxy_config, use_ipv6)
        else:
            result = await inspect_session(file_path, avatar_dir, proxy_config, use_ipv6)
        emit(result)
        return 0
    except Exception as exc:
        reason = str(exc)
        lower_reason = reason.lower()
        if proxy_config and isinstance(exc, asyncio.TimeoutError):
            result["risk_status"] = "代理连接超时"
            result["reason"] = "通过绑定代理连接 Telegram 超时"
        elif proxy_config and ("authorization key" in lower_reason or "authkeynotfound" in lower_reason or "auth key" in lower_reason):
            result["risk_status"] = "代理链路异常"
            result["reason"] = "代理出口与当前 tdata 会话不匹配，Telegram 拒绝该授权密钥"
        else:
            result["reason"] = reason
        emit(result)
        return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
