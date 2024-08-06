from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response
import logging
from svc.log import ContextFilter


class LogContextMiddleware(BaseHTTPMiddleware):
    def __init__(self, app):
        super().__init__(app)
        self.logger = logging.getLogger()
        self.context_filter = next(
            f for f in self.logger.filters if isinstance(f, ContextFilter)
        )

    async def dispatch(self, request: Request, call_next) -> Response:
        # Extract user information from the request (headers or parameters)
        user_id = request.headers.get("Svc-User-ID", "unknown")
        platform = request.headers.get("Svc-Platform", "unknown")

        # Set context in the logger
        self.context_filter.set_context(user_id=user_id, platform=platform)

        # Log the incoming request
        self.logger.info("Handling request")

        response = await call_next(request)

        # Log the outgoing response
        self.logger.info("Finished handling request")

        # Clear context after request is handled
        self.context_filter.set_context(**{})

        return response
