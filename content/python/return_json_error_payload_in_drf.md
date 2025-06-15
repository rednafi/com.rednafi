---
title: Return JSON error payload instead of HTML text in DRF
date: 2022-04-13
tags:
    - Python
    - API
    - Django
---

At my workplace, we have a large Django monolith that powers the main website and works as
the primary REST API server at the same time. We use Django Rest Framework (DRF) to build
and serve the API endpoints. This means, whenever there's an error, based on the incoming
request header—we've to return different formats of error responses to the website and API
users.

The default DRF configuration returns a JSON response when the system experiences an HTTP
400 (bad request) error. However, the server returns an HTML error page to the API users
whenever HTTP 403 (forbidden), HTTP 404 (not found), or HTTP 500 (internal server error)
occurs. This is suboptimal; JSON APIs should never return HTML text whenever something goes
wrong. On the other hand, the website needs those error text to appear accordingly.

This happens because 403, 404, and 500 are handled by Django's default handlers for those
errors and not by DRF's exception handlers. As the DRF doc suggests[^1], overriding the
error handlers is one way of solving it. But this will only work if the application is an
API-only backend or if you haven't already overridden the error handlers for custom error
pages.

In our case, we already had to override the default error handlers to display custom error
pages on the website. These custom pages would bleed into the API endpoints occasionally
when errors occur. So, I thought, if I could handle this in the middleware layer, that'd be
cleaner than most of the solutions that I'd seen at that point.

## Solution

To fix the dilemma, I wrote a middleware called `JSONErrorMiddleware` that returns the
expected response based on the content type in the request header. If the header has
`Content-Type: html/text` and it experiences an error, the server returns an appropriate
HTML page. On the contrary, if the incoming request header has
`Content-Type: application/json` and the server sees an error, it responds with a JSON error
payload instead. Here's how the middleware looks:

```py
# <app>/middleware.py

from http import HTTPStatus


class JSONErrorMiddleware:
    """Without this middleware, APIs would respond with
    html/text whenever there's an error."""

    def __init__(self, get_response):
        self.get_response = get_response
        self.status_code_description = {
            v.value: v.description for v in HTTPStatus
        }

    def __call__(self, request):
        response = self.get_response(request)

        # If the content_type isn't 'application/json', do nothing.
        if not request.content_type == "application/json":
            return response

        # If there's no error, let Django and DRF's default views deal
        # with it.
        status_code = response.status_code
        if (
            not HTTPStatus.BAD_REQUEST
            < status_code
            <= HTTPStatus.INTERNAL_SERVER_ERROR
        ):
            return response

        # Return a JSON error response if any of 403, 404, or 500 occurs.
        r = JsonResponse(
            {
                "error": {
                    "status_code": status_code,
                    "message": self.status_code_description[status_code],
                    "detail": {"url": request.get_full_path()},
                }
            },
        )
        r.status_code = response.status_code
        return r
```

You'll have to add this middleware to the list of middlewares in the `settings.py` file:

```py
MIDDLEWARE = [..., "<app>.middleware.JSONErrorMiddleware"]
```

And voila, now the API and non-API errors will be handled differently as expected!

## Test

Here's how you can unit test the behavior of the middleware:

```py
import json
from unittest.mock import MagicMock

from django.http import JsonResponse
from django.test import RequestFactory, TestCase, override_settings

from main.middleware import JSONErrorMiddleware


@override_settings(
    MIDDLEWARE_CLASSES=("main.middleware.JSONErrorMiddleware",),
)
class TestJSONErrorMiddleware(TestCase):
    def setUp(self):
        super().setUp()
        self.factory = RequestFactory()

        def get_response(request):
            response = MagicMock()
            response.status_code = HTTPStatus.FORBIDDEN
            return response

        self.middleware = JSONErrorMiddleware(get_response)

    def test_json_error_middleware(self):
        # Arrange
        corrupted_url = "/account"

        # Act
        request = self.factory.get(
            path=corrupted_url,
        )
        request.content_type = "application/json"

        response = self.middleware.__call__(request)

        # Assert
        # Assert 404 no longer returns html/text.
        self.assertTrue(isinstance(response, JsonResponse))

        # Assert json format.
        json_data = json.loads(response.content)
        expected_json_data = {
            "error": {
                "status_code": HTTPStatus.FORBIDDEN,
                "message": HTTPStatus.FORBIDDEN.description,
                "detail": {"url": "/account"},
            }
        }
        for k, v in json_data["error"].items():
            self.assertEqual(v, expected_json_data["error"][k])
```

## Breadcrumbs

This workflow has been tested on Django 3.2, 4.0, and DRF 3.13.

## References

[^1]:
    [Generic error views](https://www.django-rest-framework.org/api-guide/exceptions/#generic-error-views)

[^2]:
    [HTML sometimes returned when Accept: application/json is provided #3362](https://github.com/encode/django-rest-framework/issues/3362)
    [^2]

[^3]:
    [Added generic 500 and 400 JSON error handlers #5904](https://github.com/encode/django-rest-framework/pull/5904)
    [^3]
