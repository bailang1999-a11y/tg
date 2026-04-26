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
    ChatWriteForbiddenError,
    ChannelPrivateError,
    FloodWaitError,
    MessageIdInvalidError,
    UserBannedInChannelError,
)


def emit(payload: dict) -> None:
    print(json.dumps(payload, ensure_ascii=False))


def base_result(target: str = "", target_type: str = "", step_type: str = "") -> dict:
    return {
        "ok": False,
        "status": "failed",
        "reason": "",
        "source": "",
        "target": target,
        "target_type": target_type,
        "step_type": step_type,
        "message_id": "",
    }


def resolve_tdata_root(base_dir: str) -> str:
    for root, _, files in os.walk(base_dir):
        if "key_data" in files:
            return root
    return base_dir


def normalize_access_type(access_type: str) -> str:
    value = (access_type or "").strip().lower()
    if value in ("data", "tdata", "tdesktop"):
        return "data"
    return "session"


def normalize_target(target: str, target_type: str) -> str:
    value = (target or "").strip()
    kind = (target_type or "").strip().lower()
    if kind == "channel" and value and not value.startswith("@") and not value.startswith("https://"):
        return value
    return value


def parse_message_id(value: str) -> int:
    try:
        message_id = int((value or "").strip())
    except ValueError:
        raise ValueError("转发消息 ID 必须是数字")
    if message_id <= 0:
        raise ValueError("转发消息 ID 必须大于 0")
    return message_id


async def send_step(client: TelegramClient, target: str, target_type: str, args) -> dict:
    result = base_result(target, target_type, args.step_type)
    step_type = (args.step_type or "").strip().lower()
    entity = await client.get_entity(normalize_target(target, target_type))

    if step_type == "text":
        content = (args.content or "").strip()
        if not content:
            result["reason"] = "文本消息内容为空"
            return result
        sent = await client.send_message(entity, content)
    elif step_type in ("image", "gif", "voice"):
        media_path = os.path.abspath(args.media_path or "")
        if not media_path or not os.path.exists(media_path):
            result["reason"] = "媒体文件不存在"
            return result
        caption = (args.content or "").strip() or None
        sent = await client.send_file(
            entity,
            media_path,
            caption=caption,
            voice_note=step_type == "voice",
            force_document=False,
        )
    elif step_type == "forward":
        source_chat_id = (args.source_chat_id or "").strip()
        if not source_chat_id:
            result["reason"] = "转发来源为空"
            return result
        message_id = parse_message_id(args.message_id)
        sent = await client.forward_messages(entity, message_id, from_peer=source_chat_id)
    else:
        result["reason"] = f"不支持的消息阶段：{step_type or 'unknown'}"
        return result

    result["ok"] = True
    result["status"] = "success"
    result["reason"] = "消息已提交到 Telegram"
    result["message_id"] = str(getattr(sent, "id", "") or "")
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

    temp_dir = tempfile.mkdtemp(prefix="codex3_tdata_message_")
    with zipfile.ZipFile(file_path) as archive:
        archive.extractall(temp_dir)

    tdata_root = resolve_tdata_root(temp_dir)
    desktop = TDesktop(tdata_root, api=API.TelegramDesktop)
    if not desktop.isLoaded() or desktop.accountsCount < 1:
        shutil.rmtree(temp_dir, ignore_errors=True)
        raise ValueError("未从 tdata 中识别到账户，请重新导入完整目录")

    session_path = os.path.join(temp_dir, "telethon_message")
    client = await desktop.ToTelethon(
        session=session_path,
        flag=UseCurrentSession,
        api=API.TelegramDesktop,
        receive_updates=False,
    )
    await client.connect()
    return client, temp_dir


async def send_with_account(args) -> dict:
    file_path = os.path.abspath(args.file)
    result = base_result(args.target, args.target_type, args.step_type)
    client = None
    temp_dir = None
    source = normalize_access_type(args.access_type)

    try:
        if not os.path.exists(file_path):
            result["reason"] = "本地会话文件不存在"
            return result

        if source == "data":
            client, temp_dir = await open_tdata(file_path)
            result["source"] = "tdata"
        else:
            client = await open_session(file_path)
            result["source"] = "session"

        if not await client.is_user_authorized():
            result["reason"] = "账号未授权，需要重新登录"
            return result

        sent = await send_step(client, args.target, args.target_type, args)
        sent["source"] = result["source"]
        return sent
    except FloodWaitError as exc:
        result["source"] = "tdata" if source == "data" else "session"
        result["reason"] = f"触发 Telegram 限流，需等待 {getattr(exc, 'seconds', 0)} 秒"
        return result
    except (ChatWriteForbiddenError, ChannelPrivateError):
        result["source"] = "tdata" if source == "data" else "session"
        result["reason"] = "目标不可写入或需要加入/授权"
        return result
    except UserBannedInChannelError:
        result["source"] = "tdata" if source == "data" else "session"
        result["reason"] = "账号已被该目标限制"
        return result
    except MessageIdInvalidError:
        result["source"] = "tdata" if source == "data" else "session"
        result["reason"] = "转发消息不存在或不可访问"
        return result
    except Exception as exc:
        result["source"] = "tdata" if source == "data" else "session"
        result["reason"] = str(exc)
        return result
    finally:
        if client is not None:
            await client.disconnect()
        if temp_dir:
            shutil.rmtree(temp_dir, ignore_errors=True)


async def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", required=True)
    parser.add_argument("--access-type", default="")
    parser.add_argument("--target", required=True)
    parser.add_argument("--target-type", required=True)
    parser.add_argument("--step-type", required=True)
    parser.add_argument("--content", default="")
    parser.add_argument("--media-path", default="")
    parser.add_argument("--source-chat-id", default="")
    parser.add_argument("--message-id", default="")
    args = parser.parse_args()

    result = await send_with_account(args)
    emit(result)
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
