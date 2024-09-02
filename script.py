# script.py

import httpx
import asyncio
from typing import Any


async def make_request(url: str, client: httpx.AsyncClient) -> dict[str, Any]:
    response = await client.post(
        url,
        json={"key_1": "value_1", "key_2": "value_2"},
    )
    return response.json()


async def main() -> None:
    headers = {"Content-Type": "application/json"}
    url = "https://httpbin.org/post"

    async with httpx.AsyncClient(headers=headers) as client:
        response = await make_request(url, client)
        import json

        print(json.dumps(response, indent=2))


asyncio.run(main())
