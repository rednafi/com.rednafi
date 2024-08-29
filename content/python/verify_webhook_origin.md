---
title: Verifying webhook origin via payload hash signing
date: 2022-09-18
tags:
    - Python
    - API
---

While working with GitHub webhooks, I discovered a common pattern[^1] a webhook receiver can
adopt to verify that the incoming webhooks are indeed arriving from GitHub; not from some
miscreant trying to carry out a man-in-the-middle attack. After some amount of digging, I
found that it's quite a common practice that many other webhook services employ as well.
Also, check out how Sentry does it here[^2].

Moreover, GitHub's documentation demonstrates the pattern in Ruby. So I thought it'd be a
good idea to translate that into Python in a more platform-agnostic manner. The core idea of
the pattern goes as follows:

-   The webhook sender will hash the JSONified webhook payload with a well-known hashing
    algorithm like MD5, SHA-1, or SHA-256. A secret token known to the receiver will be used
    to sign the calculated hash of the payload.

-   The sender will include the payload hash digest prefixed by the name of the hash
    algorithm to the header of the webhook request. For example, the GitHub webhook's
    request header has a key like the following. Notice how the digest is prefixed with the
    name of the algorithm `sha256`:

```txt
X-Hub-Signature-256=\
    sha-256=e863e1f6370b60981bbbcbc2da3313321e65eaaac36f9d1262af415965df9320
```

-   The webhook receiver is then expected to hash the received JSON payload with the same
    algorithm found in the prefix of the header and sign with the common secret token known
    to both the sender and the receiver. Afterward, the receiver compares the calculated
    hash with the incoming hash in the request header. If the two digests match, that
    ensures that the payload hasn't been tampered with. Otherwise, the receiver should
    reject the incoming payload. This provides a second layer of protection over the usual
    authentication that the receiver might have in place.

To demonstrate the workflow, here's an example of how the webhook sender might be
implemented:

```python
# sender.py

from __future__ import annotations

import hashlib
import json
from http import HTTPStatus

import httpx
from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import JSONResponse
from starlette.routing import Route


async def send_webhook(request: Request) -> JSONResponse:
    # Get the request body as bytes.
    raw_body = await request.body()

    # Disallow empty body.
    if not raw_body:
        return JSONResponse(
            {"error": "Empty body"},
            status_code=HTTPStatus.BAD_REQUEST,
        )

    # Check that the request body is a valid JSON payload.
    try:
        body = json.loads(raw_body)
    except json.JSONDecodeError:
        return JSONResponse(
            {"error": "Invalid JSON body"},
            status_code=HTTPStatus.BAD_REQUEST,
        )

    # Hash the body and sign it with a secret.
    x_payload_signature = hashlib.sha256(raw_body)
    x_payload_signature.update(b"some-secret")
    x_payload_signature = x_payload_signature.hexdigest()

    # Send the webhook.
    async with httpx.AsyncClient() as client:
        response = await client.post(
            "http://localhost:6000/receive-webhook",
            json=body,
            headers={
                "X-Payload-Signature-256": f"sha256={x_payload_signature}",
                "Content-Type": "application/json",
            },
        )

        if response.status_code != HTTPStatus.ACCEPTED:
            return JSONResponse(
                {"error": "Could not sent webhook"},
                status_code=HTTPStatus.BAD_REQUEST,
            )

    return JSONResponse(
        {
            "message": "Webhook sent",
            "response_payload": response.json(),
        },
        status_code=HTTPStatus.OK,
    )


app = Starlette(
    debug=True,
    routes=[
        Route("/send-webhook", send_webhook, methods=["POST"]),
    ],
)
```

Here, I've implemented a simple POST API that:

-   Accepts a payload from the user.
-   Hashes the payload with `sha-256` algorithm and signs it with a `some-secret` token.
-   Adds the digest to the request header to the receiver. The header has a key called
    `X-Payload-Signature-256` that contains the prefixed payload digest:

```txt
X-Payload-Signature-256: \
    sha-256=e863e1f6370b60981bbbcbc2da3313321e65eaaac36f9d1262af415965df9320
```

-   After hashing, the sender sends the payload to the receiver via HTTP POST request. Here,
    I'm using HTTPx to send the request to the receiver. For demonstration purposes, I'm
    assuming that the receiver endpoint is `localhost:6000/receive-webhook`.

The receiver will:

-   Accept the incoming request from the sender.
-   Parse the header and store the value of `X-Payload-Signature-256`.
-   Calculate the hash value of the incoming payload in the same manner as the sender.
-   Sign the payload with the common secret that's known to both parties.
-   Compare the newly calculated signed-hash with the digest value of the
    `X-Payload-Signature-256` attribute.
-   Only accept and process the payload if the incoming and the computed hashes match.

Here's how you can implement the receiver:

```python
# receiver.py

from __future__ import annotations

import hashlib
import json
import secrets
from http import HTTPStatus

from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import JSONResponse
from starlette.routing import Route


async def receive_webhook(request: Request) -> JSONResponse:
    # Get the payload signature from the request headers.
    x_payload_signature_256 = request.headers.get("X-Payload-Signature-256")

    # Disallow empty signature.
    if x_payload_signature_256 is None:
        return JSONResponse(
            {"error": "Missing X-Payload-Signature header"},
            status_code=HTTPStatus.BAD_REQUEST,
        )

    # Check that the signature is valid.
    if not x_payload_signature_256.startswith("sha256="):
        return JSONResponse(
            {"error": "Invalid X-Payload-Signature header"},
            status_code=HTTPStatus.BAD_REQUEST,
        )

    # Get x_payload_signature_256 without the "sha256=" prefix.
    x_payload_signature = x_payload_signature_256.removeprefix("sha256=")

    raw_body = await request.body()

    # Disallow empty body.
    if not raw_body:
        return JSONResponse(
            {"error": "Empty body"},
            status_code=HTTPStatus.BAD_REQUEST,
        )

    # Check that the request body is a valid JSON payload.
    try:
        body = json.loads(raw_body)
    except json.JSONDecodeError:
        return JSONResponse(
            {"error": "Invalid JSON body"},
            status_code=HTTPStatus.BAD_REQUEST,
        )

    # Hash the incoming body with the secret.
    expected_signature = hashlib.sha256(raw_body)
    expected_signature.update(b"some-secret")
    expected_signature = expected_signature.hexdigest()

    # Compare the expected signature with the incoming signature.
    if (
        secrets.compare_digest(x_payload_signature, expected_signature)
        is False
    ):
        return JSONResponse(
            {"error": "Invalid signature"},
            status_code=HTTPStatus.UNAUTHORIZED,
        )

    return JSONResponse(
        {"message": "Webhook accepted"},
        status_code=HTTPStatus.ACCEPTED,
    )


app = Starlette(
    debug=True,
    routes=[
        Route("/receive-webhook", receive_webhook, methods=["POST"]),
    ],
)
```

> In the receiver, instead of using plain string comparison to compare the payload hashes,
> leverage `secrets.compare_digest` to mitigate the possibility of timing attacks[^3].

To test the end-to-end workflow, you'll need to pip install `httpx`[^4] and `uvicorn`[^5].
Then on your console, you can run the two scripts in the background with the following
command:

```sh
nohup uvicorn sender:app --reload --port 5000 > /dev/null \
    & nohup uvicorn receiver:app --reload --port 6000 > /dev/null &
```

This will spin up two uvicorn servers in the background where the sender and the receiver
can be accessed via ports 5000 and 6000 respectively. Now if you make a request to the
sender service, you'll see that the sender sends the webhook payload to the receiver service
and returns an HTTP 200 code only if the receiver has been able to verify the signed-hash of
the payload:

```sh
curl -si POST http://localhost:5000/send-webhook -d '{"hello": "world"}'
```

This will return:

```txt
HTTP/1.1 200 OK
date: Tue, 20 Sep 2022 06:31:07 GMT
server: uvicorn
content-length: 76
content-type: application/json

{"message":"Webhook sent","response_payload":{"message":"Webhook accepted"}}
```

The reciver will return a HTTP 400 error code if it can't verify the payload. Once you're
done, kill the running servers with `sudo pkill uvicorn` command.

[^1]:
    [Securing your webhooks](https://docs.github.com/en/developers/webhooks-and-events/webhooks/securing-your-webhooks)

[^2]:
    [Sentry hook resources](https://docs.sentry.io/product/integrations/integration-platform/webhooks/#sentry-hook-resource)

[^3]: [Timing attack](https://en.wikipedia.org/wiki/Timing_attack)
[^4]: [HTTPx](https://www.python-httpx.org/)
[^5]: [Uvicorn](https://www.uvicorn.org/)
