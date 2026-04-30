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
    UserAlreadyParticipantError,
    UserBannedInChannelError,
)
from telethon.tl.functions.channels import JoinChannelRequest
from telethon.tl.functions.messages import ImportChatInviteRequest

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


def normalize_invite_hash(identifier: str) -> str:
    value = (identifier or "").strip()
    if value.lower().startswith("joinchat/"):
        return value.split("/", 1)[1].strip()
    if value.startswith("+"):
        return value[1:].strip()
    return value


def normalize_public_target(identifier: str) -> str:
    value = (identifier or "").strip()
    lower = value.lower()
    for prefix in ("https://t.me/", "http://t.me/"):
        if lower.startswith(prefix):
            value = value[len(prefix):]
            break
    value = value.strip().strip("/")
    if value.startswith("@"):
        value = value[1:]
    return value


async def join_target(client: TelegramClient, target: str, target_type: str) -> dict:
    result = base_result(target, target_type)
    target = (target or "").strip()
    target_type = (target_type or "").strip().lower()

    try:
        if target_type == "invite":
            invite_hash = normalize_invite_hash(target)
            if not invite_hash:
                result["reason"] = "邀请链接缺少参数"
                return result
            await client(ImportChatInviteRequest(invite_hash))
        elif target_type in ("channel", "group"):
            public_target = normalize_public_target(target)
            if not public_target:
                result["reason"] = "公开群链接缺少用户名"
                return result
            entity = await client.get_entity(public_target)
            await client(JoinChannelRequest(entity))
        elif target_type == "private_channel":
            result["reason"] = "私有频道 c/... 不能直接加入，请改用邀请链接"
            return result
        else:
            result["reason"] = f"目标类型不支持加群：{target_type or 'unknown'}"
            return result

        result["ok"] = True
        result["status"] = "success"
        result["reason"] = "已加入目标"
        return result
    except UserAlreadyParticipantError:
        result["ok"] = True
        result["status"] = "already_joined"
        result["reason"] = "账号已在目标中"
        return result
    except FloodWaitError as exc:
        result["reason"] = f"触发 Telegram 限流，需等待 {getattr(exc, 'seconds', 0)} 秒"
        return result
    except (InviteHashInvalidError, InviteHashExpiredError):
        result["reason"] = "邀请链接无效或已过期"
        return result
    except ChannelPrivateError:
        result["reason"] = "目标不可访问或需要邀请权限"
        return result
    except UserBannedInChannelError:
        result["reason"] = "账号已被该目标限制"
        return result
    except Exception as exc:
        result["reason"] = str(exc)
        return result


async def join_with_session(file_path: str, target: str, target_type: str, proxy_config=None, use_ipv6: bool = False) -> dict:
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

    result = base_result(target, target_type)
    result["source"] = "session"
    try:
        await client.connect()
        if not await client.is_user_authorized():
            result["reason"] = "会话未授权，需要重新登录"
            return result
        joined = await join_target(client, target, target_type)
        joined["source"] = "session"
        return joined
    finally:
        await client.disconnect()


async def join_with_tdata(file_path: str, target: str, target_type: str, proxy_config=None, use_ipv6: bool = False) -> dict:
    result = base_result(target, target_type)
    result["source"] = "tdata"
    if not zipfile.is_zipfile(file_path):
        result["reason"] = "当前 tdata 文件不完整，请重新导入完整 tdata 文件夹或 zip"
        return result

    temp_dir = tempfile.mkdtemp(prefix="codex3_tdata_join_")
    client = None
    try:
        with zipfile.ZipFile(file_path) as archive:
            archive.extractall(temp_dir)

        tdata_root = resolve_tdata_root(temp_dir)
        desktop = TDesktop(tdata_root, api=API.TelegramDesktop)
        if not desktop.isLoaded() or desktop.accountsCount < 1:
            result["reason"] = "未从 tdata 中识别到账户，请重新导入完整目录"
            return result

        session_path = os.path.join(temp_dir, "telethon_join")
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
        if not await client.is_user_authorized():
            result["reason"] = "tdata 会话未授权，需要重新导入"
            return result

        joined = await join_target(client, target, target_type)
        joined["source"] = "tdata"
        return joined
    finally:
        if client is not None:
            await client.disconnect()
        shutil.rmtree(temp_dir, ignore_errors=True)


async def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", required=True)
    parser.add_argument("--access-type", default="")
    parser.add_argument("--target", required=True)
    parser.add_argument("--target-type", required=True)
    parser.add_argument("--proxy-json", default="")
    args = parser.parse_args()

    file_path = os.path.abspath(args.file)
    access_type = (args.access_type or "").strip().lower()
    result = base_result(args.target, args.target_type)

    try:
        if not os.path.exists(file_path):
            result["reason"] = "本地会话文件不存在"
            emit(result)
            return 0
        proxy_config = telethon_proxy_from_json(args.proxy_json)
        use_ipv6 = telethon_use_ipv6_from_json(args.proxy_json)
        if access_type == "data":
            result = await join_with_tdata(file_path, args.target, args.target_type, proxy_config, use_ipv6)
        else:
            result = await join_with_session(file_path, args.target, args.target_type, proxy_config, use_ipv6)
        emit(result)
        return 0
    except Exception as exc:
        result["reason"] = str(exc)
        emit(result)
        return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
