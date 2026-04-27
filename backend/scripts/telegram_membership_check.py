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
from telethon.errors import (
    ChannelPrivateError,
    FloodWaitError,
    InviteHashExpiredError,
    InviteHashInvalidError,
    UserBannedInChannelError,
    UserNotParticipantError,
    UsernameInvalidError,
    UsernameNotOccupiedError,
)
from telethon.tl.functions.channels import GetParticipantRequest
from telethon.tl.functions.messages import CheckChatInviteRequest
from telethon.tl.types import ChannelParticipantBanned, ChatInviteAlready

from telegram_proxy import telethon_proxy_from_json, telethon_use_ipv6_from_json


def emit(payload: dict) -> None:
    print(json.dumps(payload, ensure_ascii=False))


def base_result(target: str = "", target_type: str = "") -> dict:
    return {
        "ok": False,
        "status": "failed",
        "reason": "",
        "source": "",
        "target": target,
        "target_type": target_type,
        "ref": "",
    }


def batch_result(results: list, source: str = "", status: str = "success", reason: str = "") -> dict:
    return {
        "ok": status == "success",
        "status": status,
        "reason": reason,
        "source": source,
        "results": results,
    }


def failed_results(targets: list, status: str, reason: str, source: str = "") -> list:
    results = []
    for item in targets:
        result = base_result(str(item.get("target", "")), str(item.get("target_type", "")))
        result["ref"] = str(item.get("ref", ""))
        result["source"] = source
        result["status"] = status
        result["reason"] = reason
        results.append(result)
    return results


def resolve_tdata_root(base_dir: str) -> str:
    for root, _, files in os.walk(base_dir):
        if "key_data" in files:
            return root
    return base_dir


def normalize_invite_hash(identifier: str) -> str:
    value = (identifier or "").strip()
    if value.startswith("https://t.me/"):
        value = value.replace("https://t.me/", "", 1).strip("/")
    if value.startswith("http://t.me/"):
        value = value.replace("http://t.me/", "", 1).strip("/")
    if value.lower().startswith("joinchat/"):
        return value.split("/", 1)[1].strip()
    if value.startswith("+"):
        return value[1:].strip()
    return value


def normalize_public_target(identifier: str) -> str:
    value = (identifier or "").strip()
    if value.startswith("https://t.me/"):
        value = value.replace("https://t.me/", "", 1).strip("/")
    if value.startswith("http://t.me/"):
        value = value.replace("http://t.me/", "", 1).strip("/")
    return value.lstrip("@")


def mark_active(result: dict, reason: str = "账号仍在目标群内") -> dict:
    result["ok"] = True
    result["status"] = "active"
    result["reason"] = reason
    return result


async def check_invite_membership(client: TelegramClient, target: str, result: dict) -> dict:
    invite_hash = normalize_invite_hash(target)
    if not invite_hash:
        result["status"] = "target_invalid"
        result["reason"] = "邀请链接缺少参数"
        return result
    try:
        invite = await client(CheckChatInviteRequest(invite_hash))
        if isinstance(invite, ChatInviteAlready):
            return mark_active(result)
        result["status"] = "not_member"
        result["reason"] = "账号不在该邀请目标内"
        return result
    except (InviteHashInvalidError, InviteHashExpiredError):
        result["status"] = "target_invalid"
        result["reason"] = "邀请链接无效或已过期"
    except UserBannedInChannelError:
        result["status"] = "banned"
        result["reason"] = "账号已被该目标限制"
    except FloodWaitError as exc:
        result["status"] = "flood_wait"
        result["reason"] = f"触发 Telegram 限流，需等待 {getattr(exc, 'seconds', 0)} 秒"
    except Exception as exc:
        result["reason"] = str(exc)
    return result


async def check_channel_membership(client: TelegramClient, target: str, result: dict, me=None) -> dict:
    try:
        entity = await client.get_entity(normalize_public_target(target))
        if me is None:
            me = await client.get_me()
        participant = await client(GetParticipantRequest(entity, me))
        participant_value = getattr(participant, "participant", None)
        if isinstance(participant_value, ChannelParticipantBanned):
            result["status"] = "kicked" if getattr(participant_value, "left", False) else "banned"
            result["reason"] = "账号已被该目标移除或限制"
            return result
        return mark_active(result)
    except UserNotParticipantError:
        result["status"] = "kicked"
        result["reason"] = "账号已不在该目标群内"
    except UserBannedInChannelError:
        result["status"] = "banned"
        result["reason"] = "账号已被该目标限制"
    except (ChannelPrivateError, UsernameInvalidError, UsernameNotOccupiedError):
        result["status"] = "inaccessible"
        result["reason"] = "目标不可访问，账号可能已被踢出或目标已私有"
    except FloodWaitError as exc:
        result["status"] = "flood_wait"
        result["reason"] = f"触发 Telegram 限流，需等待 {getattr(exc, 'seconds', 0)} 秒"
    except Exception as exc:
        result["reason"] = str(exc)
    return result


async def check_membership(client: TelegramClient, target: str, target_type: str, me=None) -> dict:
    result = base_result(target, target_type)
    target_type = (target_type or "").strip().lower()
    target = (target or "").strip()
    if not target:
        result["status"] = "target_invalid"
        result["reason"] = "缺少目标标识"
        return result
    if target_type == "invite":
        return await check_invite_membership(client, target, result)
    if target_type in {"channel", "group"}:
        return await check_channel_membership(client, target, result, me)
    result["status"] = "target_invalid"
    result["reason"] = f"目标类型不支持成员校验：{target_type or 'unknown'}"
    return result


async def check_many_memberships(client: TelegramClient, targets: list, source: str) -> list:
    results = []
    me = None
    if any(str(item.get("target_type", "") or "").strip().lower() in {"channel", "group"} for item in targets):
        me = await client.get_me()
    for index, item in enumerate(targets):
        target = str(item.get("target", "") or "").strip()
        target_type = str(item.get("target_type", "") or "").strip()
        checked = await check_membership(client, target, target_type, me)
        checked["source"] = source
        checked["ref"] = str(item.get("ref", "") or "")
        results.append(checked)
        if checked.get("status") == "flood_wait":
            remaining = failed_results(targets[index + 1 :], "flood_wait", checked.get("reason") or "触发 Telegram 限流", source)
            results.extend(remaining)
            break
    return results


async def open_session(file_path: str, proxy_config=None, use_ipv6: bool = False) -> TelegramClient:
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
    )
    await client.connect()
    return client


async def open_tdata(file_path: str, proxy_config=None, use_ipv6: bool = False):
    if not zipfile.is_zipfile(file_path):
        raise ValueError("当前 tdata 文件不完整，请重新导入完整 tdata 文件夹或 zip")
    temp_dir = tempfile.mkdtemp(prefix="codex3_tdata_member_")
    with zipfile.ZipFile(file_path) as archive:
        archive.extractall(temp_dir)
    tdata_root = resolve_tdata_root(temp_dir)
    desktop = TDesktop(tdata_root, api=API.TelegramDesktop)
    if not desktop.isLoaded() or desktop.accountsCount < 1:
        shutil.rmtree(temp_dir, ignore_errors=True)
        raise ValueError("未从 tdata 中识别到账户，请重新导入完整目录")
    session_path = os.path.join(temp_dir, "telethon_member")
    client = await desktop.ToTelethon(
        session=session_path,
        flag=UseCurrentSession,
        api=API.TelegramDesktop,
        receive_updates=False,
        use_ipv6=use_ipv6,
    )
    if proxy_config:
        client.set_proxy(proxy_config)
    await client.connect()
    return client, temp_dir


async def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", required=True)
    parser.add_argument("--access-type", default="")
    parser.add_argument("--target", default="")
    parser.add_argument("--target-type", default="")
    parser.add_argument("--targets-json", default="")
    parser.add_argument("--proxy-json", default="")
    args = parser.parse_args()
    batch_mode = bool((args.targets_json or "").strip())
    targets = []
    if batch_mode:
        try:
            decoded = json.loads(args.targets_json)
            if isinstance(decoded, list):
                targets = decoded
        except Exception:
            targets = []
        if not targets:
            emit(batch_result([], status="failed", reason="批量目标参数无效"))
            return 0
    result = base_result(args.target, args.target_type)
    client = None
    temp_dir = None
    try:
        if not os.path.exists(args.file):
            reason = "本地会话文件不存在"
            if batch_mode:
                emit(batch_result(failed_results(targets, "account_invalid", reason), status="account_invalid", reason=reason))
            else:
                result["status"] = "account_invalid"
                result["reason"] = reason
                emit(result)
            return 0
        proxy_config = telethon_proxy_from_json(args.proxy_json)
        use_ipv6 = telethon_use_ipv6_from_json(args.proxy_json)
        if (args.access_type or "").strip().lower() == "data":
            client, temp_dir = await open_tdata(os.path.abspath(args.file), proxy_config, use_ipv6)
            result["source"] = "tdata"
        else:
            client = await open_session(os.path.abspath(args.file), proxy_config, use_ipv6)
            result["source"] = "session"
        if not await client.is_user_authorized():
            reason = "会话未授权，需要重新导入"
            if batch_mode:
                emit(batch_result(failed_results(targets, "account_invalid", reason, result["source"]), source=result["source"], status="account_invalid", reason=reason))
            else:
                result["status"] = "account_invalid"
                result["reason"] = reason
                emit(result)
            return 0
        if batch_mode:
            checked_results = await check_many_memberships(client, targets, result["source"])
            emit(batch_result(checked_results, source=result["source"]))
            return 0
        checked = await check_membership(client, args.target, args.target_type)
        checked["source"] = result["source"]
        emit(checked)
        return 0
    except Exception as exc:
        reason = str(exc)
        if batch_mode:
            emit(batch_result(failed_results(targets, "failed", reason, result["source"]), source=result["source"], status="failed", reason=reason))
        else:
            result["reason"] = reason
            emit(result)
        return 0
    finally:
        if client is not None:
            await client.disconnect()
        if temp_dir:
            shutil.rmtree(temp_dir, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
