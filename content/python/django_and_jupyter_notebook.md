---
title: Debugging a containerized Django application in Jupyter Notebook
date: 2023-01-14
tags:
  - Python
  - Django
---

Back in the days when I was working as a data analyst, I used to spend hours inside Jupyter
notebooks exploring, wrangling, and plotting data to gain insights. However, as I shifted my
career gear towards backend software development, my usage of interactive exploratory tools
dwindled.

Nowadays, I spend the majority of my time working on a fairly large Django monolith
accompanied by a fleet of microservices. Although I love my text editor and terminal
emulators, I miss the ability to just start a Jupyter Notebook server and run code snippets
interactively. While Django allows you to open up a shell environment and run code snippets
interactively, it still isn't as flexible as a notebook.

So, I wanted to see if I could connect a Jupyter notebook server to a containerized Django
application running on my local machine and interactively start making queries from there.
Turns out, you can do that by integrating three tools into your Dockerized environment:
ipykernel[^1], jupyter[^2], and django-extensions[^3]. Before I start explaining how
everything is tied together, here's a fully working example[^4] of a containerized Django
application where you can log into the Jupyter server and start debugging the app.

The app is just a Dockerized version of the famous `polls-app` from the Django tutorial. The
directory structure looks as follows:

```txt
../django-jupyter/
├── Dockerfile
├── docker-compose.yml
├── mysite
│   ├── db.sqlite3
│   ├── manage.py
│   ├── mysite
│   │   ├── __init__.py
│   │   ├── _debug_settings.py
│   │   ├── asgi.py
│   │   ├── settings.py
│   │   ├── urls.py
│   │   └── wsgi.py
│   ├── polls
│   │   ├── __init__.py
│   │   ├── admin.py
│   │   ├── apps.py
│   │   ├── migrations
│   │   │   ├── 0001_initial.py
│   │   │   └── __init__.py
│   │   ├── models.py
│   │   ├── tests.py
│   │   ├── urls.py
│   │   └── views.py
│   └── script.ipynb
├── requirements.txt
└── requirements-dev.txt
```

We define and pin the dependencies required for the Jupyter integration in the
`requirements-dev.txt` file:

```txt
# These pinned deps will probably get outdated by the time you're reading it.
# Use the latest version but always pin them in applications.

ipykernel==6.20.1
jupyter==1.0.0
django-extensions==3.2.1
```

The application dependencies are defined in the `requirements.txt` file:

```txt
django==4.1.5
```

In the `mysite/mysite/_debug_settings.py` file, we import the configs from the primary
settings file and add the Jupyter configuration attributes there. Here's the full content of
the extended `_debug_settings.py` file:

```py
from .settings import *  # noqa

INSTALLED_APPS.append("django_extensions")  # noqa

SHELL_PLUS = "ipython"
SHELL_PLUS_PRINT_SQL = True
IPYTHON_ARGUMENTS = [
    "--ext",
    "django_extensions.management.notebook_extension",
    "--debug",
]

IPYTHON_KERNEL_DISPLAY_NAME = "Django Shell-Plus"
NOTEBOOK_ARGUMENTS = [
    "--ip",
    "0.0.0.0",
    "--port",
    "8895",
    "--allow-root",
    "--no-browser",
    "--NotebookApp.iopub_data_rate_limit=1e5",
    "--NotebookApp.token=''",
]

DJANGO_ALLOW_ASYNC_UNSAFE = True
```

Notice how we're appending the `django_extensions` app to the `INSTALLED_APPS` list defined
in the main settings file. Then we're setting the shell to `ipython` with the `SHELL_PLUS`
attribute. The `NOTEBOOK_ARGUMENTS` defines the port of the Jupyter server and some
auth-specific settings.

Next, in the `Dockerfile`, we're defining the application like this:

```dockerfile
# Dockerfile

FROM python:3.11-bullseye

# Set the working directory inside the container.
WORKDIR /code

# Don't write .pyc files and make the output unbuffered.
ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

# Install dependencies.
RUN pip install --upgrade pip
COPY requirements.txt requirements-dev.txt  ./
RUN pip install -r requirements.txt -r requirements-dev.txt

# Copy the project code.
COPY . /code
```

Finally, we're orchestrating the application and the Jupyter server in the
`docker-compose.yml` file. Here's how it looks:

```yml
version: "3.9"

services:
  web:
    build: .
    working_dir: /code/mysite
    volumes:
      - .:/code

  webserver:
    extends:
      service: web
    command: python manage.py runserver 0.0.0.0:8000
    ports:
      - "8000:8000"

  jupyter:
    extends:
      service: web
    environment:
      - DJANGO_SETTINGS_MODULE=mysite._debug_settings
      - DJANGO_ALLOW_ASYNC_UNSAFE=true
    command:
      python manage.py shell_plus --notebook
    ports:
      - "8895:8895"

  debug:
    extends:
      service: web
    working_dir: /code
    command: sleep infinity
```

We're orchestrating three services here: `webserver`, `jupyter`, and `debug`. All of them
extend the base `web` service that builds the `Dockerfile`. The `webserver` service is where
the Django app is run and exposed via the `8000` port. The `jupyter` service runs the
Jupyter server and makes it accessible through your browser via the `8895` port.
Additionally, note how we are using our extended version of the main settings by overriding
the `DJANGO_SETTINGS_MODULE` environment variable and setting it to
`mysite._debug_settings`. The `debug` container is spun up to run the migration commands and
perform other maintenance tasks within the container network. All the maintenance commands
are defined in the `Makefile` for your convenience. You can run any of these by running
`make <target>` from the root directory.

And that's it!

If you have Docker[^5] and docker-compose[^6] installed on your local system, you can give
it a try. Clone the example-app[^4] repo, navigate to the root directory and run:

```sh
docker compose up -d
```

Then run the migration command:

```sh
docker compose exec debug python mysite/manage.py makemigrations \
    && docker compose exec debug python mysite/manage.py migrate
```

Now head over to your browser and go to `http://localhost:8000`. You should see an empty
page with a simple header like this:

![empty page with a simple header][image_1]

If you go to `http://localhost:8895`, you'll be able to open a new notebook that
automatically connects to your database and allows you to write interactive code
immediately.

![interactive code in jupyter notebook][image_2]

You can run the following snippet and it'll create two questions and two choices in the
database.

```py
from polls import models as polls_models
from datetime import datetime, timezone

for question_text in ("Are you okay?", "Do you wanna go there?"):
    question = polls_models.Question.objects.create(
        question_text=question_text,
        pub_date=datetime.now(tz=timezone.utc),
    )
    question.choice_set.set(
        polls_models.Choice.objects.create(choice_text=ctext)
        for ctext in ("yes", "no")
    )
```

If you run this and refresh your application server, you'll that the objects have been
created and they appear in the view:

![display created objects in jupyter notebook][image_3]

[^1]: [ipykernel](https://ipython.readthedocs.io/en/stable/install/kernel_install.html)

[^2]: [jupyter](https://jupyter.org/)

[^3]: [django-extensions](https://django-extensions.readthedocs.io/en/latest/)

[^4]: [example-app](https://github.com/rednafi/django-jupyter)

[^5]: [Docker](https://www.docker.com/)

[^6]: [docker-compose](https://docs.docker.com/compose/)

[^7]:
    [How to access Jupyter notebook in a Docker Container](https://stackoverflow.com/questions/62193187/django-shell-plus-how-to-access-jupyter-notebook-in-docker-container)
    [^7]

[image_1]: https://blob.rednafi.com/static/images/django_and_jupyter_notebook/img_1.png
[image_2]: https://blob.rednafi.com/static/images/django_and_jupyter_notebook/img_2.png
[image_3]: https://blob.rednafi.com/static/images/django_and_jupyter_notebook/img_3.png
