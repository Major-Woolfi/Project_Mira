# --- Импорты ---
import asyncio
import aiofiles
import json
import logging
import os
import secrets
import signal
import sys
import time
import uuid

from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional

from dotenv import load_dotenv

from admin import panel
import VirtualBox
import core

load_dotenv(".env")


# --- Конфиг ---
class Config:
    LOG_DIR: str = os.getenv("LOG_DIR")
    DATA_DIR: str = os.getenv("DATA_DIR")


# --- Логирование ---
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(name)s - %(message)s",
    handlers=[
        logging.StreamHandler(),
        logging.FileHandler(
            os.path.join(Config.LOG_DIR, f"{datetime.now().strftime('%Y-%m-%d')}.log"),
            encoding="utf-8",
        ),
    ],
)
logger = logging.getLogger(__name__)


# --- Утилиты ---
async def run_service(name: str, service_main):
    try:
        logger.info(f"Запуск {name}")
        await service_main()
    except asyncio.CancelledError:
        logger.info(f"{name} остановлен")
        raise
    except Exception as e:
        logger.error(f"Ошибка в {name}: {e}", exc_info=True)


# --- Иницилизация ---
async def main():
    tasks = []

    try:
        logger.info("Инициализация...")

        tasks.append(asyncio.create_task(run_service("admin-panel", panel.main)))
        tasks.append(asyncio.create_task(run_service("VirtualBox", VirtualBox.main)))
        tasks.append(asyncio.create_task(run_service("core", core.main)))

        done, pending = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)

        logger.info("Остановка программы...")
        for task in pending:
            task.cancel()

        await asyncio.gather(*pending, return_exceptions=True)

    except KeyboardInterrupt:
        logger.info("Получен сигнал прерывания (Ctrl+C)")
    except Exception as e:
        logger.critical(f"Критическая ошибка при инициализации: {e}", exc_info=True)
    finally:
        for task in tasks:
            if not task.done():
                task.cancel()
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)
        logger.info("Программа остановлена.")


if __name__ == "__main__":
    asyncio.run(main())
