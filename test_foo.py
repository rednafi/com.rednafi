# test_script.py

import pytest
import respx
import httpx
from script import make_request


@pytest.mark.asyncio
async def test_make_request_ok():
    url = "https://httpbin.org/post"
    expected_json = {"key_1": "value_1", "key_2": "value_2"}

    # Mocking the HTTP POST request using respx
    with respx.mock:
        respx.post(url).mock(return_value=httpx.Response(200, json=expected_json))

        # Calling the function
        response = await make_request(url)

        # Assertions
        assert response == expected_json
