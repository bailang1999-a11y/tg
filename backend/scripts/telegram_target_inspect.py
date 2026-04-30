#!/usr/bin/env python3
import argparse
import asyncio
import json
import os
import shutil
import tempfile
import zipfile

from opentele.api import API, UseCurrentSession
from opentele.td import TDesktop
from telethon import TelegramClient
from telethon.errors import ChannelPrivateError, FloodWaitError, UsernameInvalidError, UsernameNotOccupiedError
from telethon.tl.functions.channels import GetFullChannelRequest
from telethon.tl.functions.messages import GetFullChatRequest
from telethon.tl.types import Channel, Chat


def emit(payload: dict) -> None:
    print(json.dumps(payload, ensure_ascii=False))


def base_result(target: str = "") -> dict:
    return {
        "ok": False,
        "status": "failed",
        "reason": "",
        "identifier": target,
        "name": "",
        "type": "group",
        "size": 0,
        "username": "",
        "source": "",
    }


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


def normalize_target(value: str) -> str:
    target = (value or "").strip()
    if target.startswith("https://t.me/"):
        return target.replace("https://t.me/", "", 1).strip("/")
    if target.startswith("http://t.me/"):
        return target.replace("http://t.me/", "", 1).strip("/")
    return target.lstrip("@")


async def fetch_participants_count(client: TelegramClient, entity) -> int:
    count = int(getattr(entity, "participants_count", 0) or 0)
    if count > 0:
        return count
    try:
        if isinstance(entity, Channel):
            full = await client(GetFullChannelRequest(entity))
            return int(getattr(getattr(full, "full_chat", None), "participants_count", 0) or 0)
        if isinstance(entity, Chat):
            full = await client(GetFullChatRequest(entity.id))
            return int(getattr(getattr(full, "full_chat", None), "participants_count", 0) or 0)
    except Exception:
        return count
    return count


async def inspect_target(client: TelegramClient, target: str) -> dict:
    result = base_result(target)
    try:
        entity = await client.get_entity(normalize_target(target))
        title = getattr(entity, "title", "") or getattr(entity, "first_name", "") or getattr(entity, "username", "") or target
        username = getattr(entity, "username", "") or ""
        megagroup = bool(getattr(entity, "megagroup", False))
        broadcast = bool(getattr(entity, "broadcast", False))
        result["ok"] = True
        result["status"] = "active"
        result["reason"] = "已刷新真实资料"
        result["name"] = title
        result["username"] = username
        result["type"] = "channel" if broadcast and not megagroup else "group"
        result["size"] = await fetch_participants_count(client, entity)
        if username:
            result["identifier"] = "https://t.me/" + username
        return result
    except FloodWaitError as exc:
        result["reason"] = f"触发 Telegram 限流，需等待 {getattr(exc, 'seconds', 0)} 秒"
    except (ChannelPrivateError, UsernameInvalidError, UsernameNotOccupiedError):
        result["reason"] = "监听群不可访问或链接无效"
    except Exception as exc:
        result["reason"] = str(exc)
    return result


async def open_session(file_path: str) -> TelegramClient:
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
    )
    await client.connect()
    return client


async def open_tdata(file_path: str):
    if not zipfile.is_zipfile(file_path):
        raise ValueError("当前 tdata 文件不完整，请重新导入完整 tdata 文件夹或 zip")
    temp_dir = tempfile.mkdtemp(prefix="codex3_tdata_target_")
    with zipfile.ZipFile(file_path) as archive:
        archive.extractall(temp_dir)
    tdata_root = resolve_tdata_root(temp_dir)
    desktop = TDesktop(tdata_root, api=API.TelegramDesktop)
    if not desktop.isLoaded() or desktop.accountsCount < 1:
        shutil.rmtree(temp_dir, ignore_errors=True)
        raise ValueError("未从 tdata 中识别到账户，请重新导入完整目录")
    session_path = os.path.join(temp_dir, "telethon_target")
    client = await desktop.ToTelethon(
        session=session_path,
        flag=UseCurrentSession,
        api=API.TelegramDesktop,
        receive_updates=False,
    )
    await client.connect()
    return client, temp_dir


async def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", required=True)
    parser.add_argument("--access-type", default="")
    parser.add_argument("--target", required=True)
    args = parser.parse_args()
    result = base_result(args.target)
    client = None
    temp_dir = None
    try:
        if not os.path.exists(args.file):
            result["reason"] = "本地会话文件不存在"
            emit(result)
            return 0
        if (args.access_type or "").strip().lower() == "data":
            client, temp_dir = await open_tdata(os.path.abspath(args.file))
            result["source"] = "tdata"
        else:
            client = await open_session(os.path.abspath(args.file))
            result["source"] = "session"
        if not await client.is_user_authorized():
            result["reason"] = "会话未授权，需要重新导入"
            emit(result)
            return 0
        inspected = await inspect_target(client, args.target)
        inspected["source"] = result["source"]
        emit(inspected)
        return 0
    except Exception as exc:
        result["reason"] = str(exc)
        emit(result)
        return 0
    finally:
        if client is not None:
            await client.disconnect()
        if temp_dir:
            shutil.rmtree(temp_dir, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
