import asyncio
import logging
from starlette.requests import Request
from starlette.responses import JSONResponse


async def work() -> None:
    await asyncio.sleep(1)
    logging.info("Work done after 1 second")


async def view(request: Request) -> JSONResponse:
    await work()

    return JSONResponse({"message": "Did some work!"})
