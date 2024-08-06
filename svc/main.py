from starlette.routing import Route
import uvicorn
from starlette.applications import Starlette
from svc.middleware import LogContextMiddleware
from svc.view import view
from starlette.middleware import Middleware

middlewares = [Middleware(LogContextMiddleware)]
app = Starlette(
    routes=[
        Route("/", view),
    ],
    middleware=middlewares,
)

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
