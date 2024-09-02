import pytest
from starlette.applications import Starlette
from starlette.responses import JSONResponse
from starlette.routing import Route
from starlette.requests import Request
from httpx import AsyncClient
from script import make_request


async def test_endpoint(request: Request) -> JSONResponse:
    return JSONResponse({"key_1": "value_1", "key_2": "value_2"})


app = Starlette(routes=[Route("/post", test_endpoint, methods=["POST"])])


@pytest.mark.asyncio
async def test_make_request() -> None:
    # Manually create the AsyncClient
    async with AsyncClient(app=app, base_url="http://testserver") as client:
        url = "http://testserver/post"
        response = await make_request(url, client=client)
        assert response == {"key_1": "value_1", "key_2": "value_2"}
