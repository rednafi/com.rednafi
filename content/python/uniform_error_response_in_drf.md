---
title: Uniform error response in Django Rest Framework
date: 2022-01-20
tags:
    - Python
    - Django
    - TIL
---

Django Rest Framework exposes a neat hook to customize the response payload of your API when
errors occur. I was going through Microsoft's REST API guideline[^1] and wanted to make the
error response of my APIs more uniform and somewhat similar to this[^2].

I'll use a modified version of the quickstart example[^3] in the DRF docs to show how to
achieve that. Also, we'll need a POST API to demonstrate the changes better. Here's the same
example with the added POST API. Place this code in the project's `urls.py` file.

```python
# urls.py

from django.urls import path, include
from django.contrib.auth.models import User
from rest_framework import routers, serializers, viewsets


# Serializers define the API representation.
class UserSerializer(serializers.HyperlinkedModelSerializer):
    class Meta:
        model = User
        fields = ["url", "username", "email", "is_staff"]

    def validate_username(self, username: str) -> str:
        if len(username) < 10:
            raise serializers.ValidationError(
                "Username must be at least 10 characters long.",
            )
        return username

    def validate_email(self, email: str) -> str:
        try:
            validate_email(email)
        except ValidationError:
            raise serializers.ValidationError("Invalid email format.")
        return email

    def create(self, validated_data: str) -> User:
        return User.objects.create(**validated_data)


# ViewSets define the view behavior.
class UserViewSet(viewsets.ModelViewSet):
    queryset = User.objects.all()
    serializer_class = UserSerializer


# Routers provide an easy way of automatically determining the URL conf.
router = routers.DefaultRouter()
router.register(r"users", UserViewSet)

# Wire up our API using automatic URL routing.
# Additionally, we include login URLs for the browsable API.
urlpatterns = [
    path("", include(router.urls)),
    path(
        "api-auth/", include("rest_framework.urls", namespace="rest_framework")
    ),
]
```

If you make a POST request to `/users` endpoint with the following payload where it'll
intentionally fail email and username validation—

```json
{
  "username": "hello",
  "email": "email..",
  "is_staff": false
}

```

you'll see the following response:

```json
{
  "username":[
    "Username must be at least 10 characters long."
  ],
  "email":[
    "Enter a valid email address."
  ]
}
```

While this is okay, there's one gotcha here. The error payload isn't consistent. Depending
on the type of error, the shape of the response payload will change. This can be a problem
if your system has custom error handling logic that expects a consistent response.

I wanted the error payload to have a predictable shape while carrying more information
like—HTTP error code, error message, etc. You can do it by wrapping the default
`rest_framework.views.exception_handler` function in a custom exception handler function.
Let's write the `api_exception_handler`:

```python
# urls.py
from rest_framework.views import exception_handler
from http import HTTPStatus
from typing import Any

from rest_framework.views import Response

...


def api_exception_handler(exc: Exception, context: dict[str, Any]) -> Response:
    """Custom API exception handler."""

    # Call REST framework's default exception handler first,
    # to get the standard error response.
    response = exception_handler(exc, context)

    if response is not None:
        # Using the description's of the HTTPStatus class as error message.
        http_code_to_message = {v.value: v.description for v in HTTPStatus}

        error_payload = {
            "error": {
                "status_code": 0,
                "message": "",
                "details": [],
            }
        }
        error = error_payload["error"]
        status_code = response.status_code

        error["status_code"] = status_code
        error["message"] = http_code_to_message[status_code]
        error["details"] = response.data
        response.data = error_payload
    return response


...
```

Now, you'll have to register this custom exception handler in the `settings.py` file. Head
over to the `REST_FRAMEWORK` section and add the following key:

```python
REST_FRAMEWORK = {
    ...
    "EXCEPTION_HANDLER": "<project>.urls.api_exception_handler",
}
```

If you make a POST request to `/users` endpoint with an invalid payload as before, you'll
see this:

```json
{
  "error": {
    "status_code":400,
    "message":"Bad request syntax or unsupported method",
    "details":{
      "username":[
        "Username must be at least 10 character long."
      ],
      "email":[
        "Enter a valid email address."
      ]
    }
  }
}
```

Much nicer!

[^1]: [API guidelines - Microsoft](https://github.com/microsoft/api-guidelines)
[^2]:
    [Error payload](https://github.com/microsoft/api-guidelines/blob/vNext/Guidelines.md#examples)

[^3]: [DRF example](https://www.django-rest-framework.org/#example)
[^4]:
    [Custom Exception Handling - DRF docs](https://www.django-rest-framework.org/api-guide/exceptions/#custom-exception-handling)
    [^4]
