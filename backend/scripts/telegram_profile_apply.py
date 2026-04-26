#!/usr/bin/env python3
import argparse
import asyncio
import json
import os
import re
import shutil
import tempfile
import zipfile
from typing import Optional, Tuple

from opentele.api import API, UseCurrentSession
from opentele.td import TDesktop
from telethon import TelegramClient
from telethon.errors import (
    AboutTooLongError,
    FirstNameInvalidError,
    ImageProcessFailedError,
    PhotoCropSizeSmallError,
    UsernameInvalidError,
    UsernameNotModifiedError,
    UsernameOccupiedError,
)
from telethon.tl.functions.account import UpdateProfileRequest, UpdateUsernameRequest
from telethon.tl.functions.photos import UploadProfilePhotoRequest

USERNAME_PATTERN = re.compile(r"^[A-Za-z0-9_]{5,32}$")
PROFILE_FIELDS = ("nickname", "bio", "homepage", "avatar")


def emit(payload: dict) -> None:
    print(json.dumps(payload, ensure_ascii=False))


def base_result() -> dict:
    return {
        "ok": False,
        "status": "failed",
        "reason": "",
        "source": "",
        "terminal": "",
        "requested_count": 0,
        "applied_count": 0,
        "failed_count": 0,
        "fields": {
            key: {"requested": False, "ok": False, "reason": ""}
            for key in PROFILE_FIELDS
        },
    }


def mark_requested(result: dict, key: str) -> None:
    result["fields"][key]["requested"] = True


def mark_success(result: dict, key: str) -> None:
    field = result["fields"][key]
    field["requested"] = True
    field["ok"] = True
    field["reason"] = ""


def mark_failure(result: dict, key: str, reason: str) -> None:
    field = result["fields"][key]
    field["requested"] = True
    field["ok"] = False
    field["reason"] = reason


def fail_remaining_fields(result: dict, reason: str) -> None:
    for field in result["fields"].values():
        if field["requested"] and not field["ok"] and not field["reason"]:
            field["reason"] = reason


def finalize_result(result: dict, fallback_reason: str = "") -> dict:
    requested_count = 0
    applied_count = 0
    failed_count = 0
    failed_reasons = []
    for key in PROFILE_FIELDS:
        field = result["fields"][key]
        if not field["requested"]:
            continue
        requested_count += 1
        if field["ok"]:
            applied_count += 1
            continue
        failed_count += 1
        reason = (field.get("reason") or "").strip()
        if reason:
            failed_reasons.append(f"{key}:{reason}")

    result["requested_count"] = requested_count
    result["applied_count"] = applied_count
    result["failed_count"] = failed_count

    if requested_count == 0:
        result["status"] = "failed"
        result["reason"] = fallback_reason or "未指定需要修改的资料项"
        result["ok"] = False
        return result

    if failed_count == 0:
        result["status"] = "success"
        result["reason"] = "资料修改已提交到 Telegram"
        result["ok"] = True
        return result

    if applied_count > 0:
        result["status"] = "partial_success"
        result["reason"] = "部分资料修改已提交到 Telegram"
        result["ok"] = True
        return result

    result["status"] = "failed"
    result["reason"] = fallback_reason or "；".join(failed_reasons) or "资料修改失败"
    result["ok"] = False
    return result


def resolve_tdata_root(base_dir: str) -> str:
    for root, _, files in os.walk(base_dir):
        if "key_data" in files:
            return root
    return base_dir


def parse_username(homepage: str) -> str:
    value = (homepage or "").strip()
    if not value:
        return ""
    if value.startswith("@"):
        value = value[1:]
    value = value.replace("https://", "").replace("http://", "")
    if value.lower().startswith("www."):
        value = value[4:]
    if value.lower().startswith("t.me/"):
        value = value[5:]
    elif value.lower().startswith("telegram.me/"):
        value = value[12:]
    value = value.strip().strip("/")
    if "/" in value:
        value = value.split("/", 1)[0]
    if not USERNAME_PATTERN.match(value):
        raise ValueError("个人频道格式无效，请填写 @username 或 https://t.me/username")
    return value


async def open_client(file_path: str, access_type: str) -> Tuple[TelegramClient, Optional[str], str]:
    if access_type == "data":
        if not zipfile.is_zipfile(file_path):
            raise ValueError("当前 tdata 文件不完整，请重新导入完整 tdata 文件夹或 zip")
        temp_dir = tempfile.mkdtemp(prefix="codex3_tdata_apply_")
        with zipfile.ZipFile(file_path) as archive:
            archive.extractall(temp_dir)

        tdata_root = resolve_tdata_root(temp_dir)
        desktop = TDesktop(tdata_root, api=API.TelegramDesktop)
        if not desktop.isLoaded() or desktop.accountsCount < 1:
            shutil.rmtree(temp_dir, ignore_errors=True)
            raise ValueError("未从 tdata 中识别到账户，请重新导入完整目录")

        session_path = os.path.join(temp_dir, "telethon_apply")
        client = await desktop.ToTelethon(
            session=session_path,
            flag=UseCurrentSession,
            api=API.TelegramDesktop,
            receive_updates=False,
        )
        await client.connect()
        return client, temp_dir, "tdata"

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
    return client, None, "session"


async def apply_profile(file_path: str, access_type: str, nickname: str, bio: str, homepage: str, avatar_path: str) -> dict:
    result = base_result()
    client = None
    temp_dir = None
    source = "session" if access_type != "data" else "tdata"
    if nickname:
        mark_requested(result, "nickname")
    if bio:
        mark_requested(result, "bio")
    if homepage:
        mark_requested(result, "homepage")
    if avatar_path:
        mark_requested(result, "avatar")
    try:
        client, temp_dir, source = await open_client(file_path, access_type)
        result["source"] = source

        if not await client.is_user_authorized():
            fail_remaining_fields(result, "账号未授权，需要重新登录")
            return finalize_result(result, "账号未授权，需要重新登录")

        me = await client.get_me()
        result["terminal"] = getattr(me, "phone", None) or getattr(me, "id", None) or ""

        if nickname:
            try:
                await client(UpdateProfileRequest(first_name=nickname, last_name=""))
                mark_success(result, "nickname")
            except FirstNameInvalidError:
                mark_failure(result, "nickname", "昵称格式无效")
            except Exception as exc:
                mark_failure(result, "nickname", str(exc))

        if bio:
            try:
                await client(UpdateProfileRequest(about=bio))
                mark_success(result, "bio")
            except AboutTooLongError:
                mark_failure(result, "bio", "个性签名过长")
            except Exception as exc:
                mark_failure(result, "bio", str(exc))

        if homepage:
            try:
                username = parse_username(homepage)
                await client(UpdateUsernameRequest(username))
                mark_success(result, "homepage")
            except UsernameOccupiedError:
                mark_failure(result, "homepage", "个人频道已被占用")
            except UsernameInvalidError:
                mark_failure(result, "homepage", "个人频道格式无效")
            except UsernameNotModifiedError:
                mark_success(result, "homepage")
            except ValueError as exc:
                mark_failure(result, "homepage", str(exc))
            except Exception as exc:
                mark_failure(result, "homepage", str(exc))

        if avatar_path:
            try:
                if not os.path.exists(avatar_path):
                    raise ValueError("头像文件不存在")
                uploaded = await client.upload_file(avatar_path)
                await client(UploadProfilePhotoRequest(file=uploaded))
                mark_success(result, "avatar")
            except (PhotoCropSizeSmallError, ImageProcessFailedError):
                mark_failure(result, "avatar", "头像文件无法处理，请更换图片")
            except ValueError as exc:
                mark_failure(result, "avatar", str(exc))
            except Exception as exc:
                mark_failure(result, "avatar", str(exc))

        return finalize_result(result)
    except ValueError as exc:
        fail_remaining_fields(result, str(exc))
        return finalize_result(result, str(exc))
    except Exception as exc:
        fail_remaining_fields(result, str(exc))
        return finalize_result(result, str(exc))
    finally:
        if client is not None:
            await client.disconnect()
        if temp_dir:
            shutil.rmtree(temp_dir, ignore_errors=True)


async def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", required=True)
    parser.add_argument("--access-type", default="")
    parser.add_argument("--nickname", default="")
    parser.add_argument("--bio", default="")
    parser.add_argument("--homepage", default="")
    parser.add_argument("--avatar-path", default="")
    args = parser.parse_args()

    try:
        file_path = os.path.abspath(args.file)
        if not os.path.exists(file_path):
            result = base_result()
            result["reason"] = "本地会话文件不存在"
            if (args.nickname or "").strip():
                mark_requested(result, "nickname")
            if (args.bio or "").strip():
                mark_requested(result, "bio")
            if (args.homepage or "").strip():
                mark_requested(result, "homepage")
            if (args.avatar_path or "").strip():
                mark_requested(result, "avatar")
            fail_remaining_fields(result, result["reason"])
            emit(finalize_result(result, result["reason"]))
            return 0

        result = await apply_profile(
            file_path=file_path,
            access_type=(args.access_type or "").strip().lower(),
            nickname=(args.nickname or "").strip(),
            bio=(args.bio or "").strip(),
            homepage=(args.homepage or "").strip(),
            avatar_path=os.path.abspath(args.avatar_path) if (args.avatar_path or "").strip() else "",
        )
        emit(result)
        return 0
    except ValueError as exc:
        result = base_result()
        result["reason"] = str(exc)
        emit(finalize_result(result, result["reason"]))
        return 0
    except Exception as exc:
        result = base_result()
        result["reason"] = str(exc)
        emit(finalize_result(result, result["reason"]))
        return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
