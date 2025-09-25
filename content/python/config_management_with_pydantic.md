---
title: Pedantic configuration management with Pydantic
date: 2020-07-13
slug: config-management-with-pydantic
aliases:
    - /python/config_management_with_pydantic/
tags:
    - Python
---

Managing configurations in your Python applications isn't something you think about much
often, until complexity starts to seep in and forces you to re-architect your initial
approach. Ideally, your config management flow shouldn't change across different
applications or as your application begins to grow in size and complexity.

Even if you're writing a library, there should be a consistent config management process
that scales up properly. Since I primarily spend my time writing data-analytics,
data-science applications and expose them using Flask[^1] or FastAPI[^2] framework, I'll be
tacking config management from an application development perspective.

## Few ineffective approaches

In the past, while exposing APIs with Flask, I used to use `.env`, `.flaskenv` and `Config`
class approach to manage configs which is pretty much a standard in the Flask realm.
However, it quickly became cumbersome to maintain and juggle between configs depending on
development, staging or production environments.

There were additional application specific global constants to deal with too. So I tried
using `*.json`, `*.yaml` or `*.toml` based config management approaches but those too,
quickly turned into a tangled mess. I was constantly accessing variables buried into 3-4
levels of nested toml data structure and it wasn't pretty.

Then there are config management libraries like Dynaconf[^3] or environ-config[^4] that aim
to ameliorate the issue. While these are all amazing tools but they also introduce their own
custom workflow that can feel over-engineered while dealing with maintenance and extension.

## A pragmatic wishlist

I wanted to take a declarative approach while designing a config management pipleline
that'll be **modular**, **scalable** and easy to **maintain**. To meet my requirements, the
system should be able to:

- Read configs from `.env` files and _shell environment_ at the same time.
- Handle dependency injection for introducing _passwords_ or _secrets_.
- Convert variable types automatically in the appropriate cases, e.g. string to integer
  conversion.
- Keep _development_, _staging_ and _production_ configs separate.
- Switch between the different environments e.g development, staging effortlessly.
- Inspect the _active_ config values
- Create arbitrarily nested config structure if required (Not encouraged though).

## Building the config management pipeline

### Preparation

The code block that appears in this section is self contained. It should run without any
modifications. If you want to play along, then just spin up a Python virtual environment and
install `Pydantic` and `python-dotenv`. The following commands works on any _\*nix_ based
system:

```sh
python3.10 -m venv venv
source venv/bin/activate
pip install pydantic python-dotenv
```

Make sure you have fairly a recent version of `Python 3` installed, preferably
`Python 3.10+`. You might need to install `python3.10 venv`.

### Introduction to Pydantic

To check off all the boxes of the wishlist above, I made a custom config management flow
using Pydantic[^5], python-dotenv[^6] and the `.env` file. Pydantic is a fantastic data
validation library that can be used for validating and implicitly converting data types
using Python's type hints. Type hinting is a formal solution to statically indicate the type
of a value within your Python code. It was specified in PEP-484[^7] and introduced in Python
3.5. Let's define and validate the attributes of a class named `User`:

```py
from Pydantic import BaseModel


class User(BaseModel):
    name: str
    username: str
    password: int


user = User(name="Redowan Delowar", username="rednafi", password="123")

print(user)
```

This will give you:

```txt
>>> User(name='Redowan Delowar', username='rednafi', password=123)
```

In the above example, I defined a simple class named `User` and used Pydantic for data
validation. Pydantic will make sure that the data you assign to the class attributes conform
with the types you've annotated. Notice, how I've assigned a string type data in the
`password` field and Pydantic converted it to integer type without complaining. That's
because the corresponding type annotation suggests that the `password` attribute of the
`User` class should be an integer. When implicit conversion is not possible or the hinted
value of an attribute doesn't conform to its assigned type, Pydantic will throw a
`ValidationError`.

### The orchestration

Now let's see how you can orchestrate your config management flow with the tools mentioned
above. For simplicity, let's say you've 3 sets of configurations.

1. Configs of your app's internal logic
2. Development environment configs
3. Production environment configs

In this case, other than the first set of configs, all should go into the `.env` file.

I'll be using this `.env` file for demonstration. If you're following along, then go ahead,
create an empty `.env` file there and copy the variables mentioned below:

```txt
#.env

ENV_STATE="dev" # or prod

DEV_REDIS_HOST="127.0.0.1"
DEV_REDIS_PORT="4000"

PROD_REDIS_HOST="127.0.0.2"
PROD_REDIS_PORT="5000"
```

Notice how I've used the `DEV_` and `PROD_` prefixes before the environment specific
configs. These help you discern between the variables designated for different environments.

> Configs related to your application's internal logic should either be explicitly mentioned
> in the same `configs.py` or imported from a different `app_configs.py` file. You shouldn't
> pollute your `.env` files with the internal global variables necessitated by your
> application's core logic.

Now let's dump the entire config orchestration and go though the building blocks one by one:

```py
# configs.py

from typing import Optional

from pydantic import BaseSettings, Field, BaseModel


class AppConfig(BaseModel):
    """Application configurations."""

    VAR_A: int = 33
    VAR_B: float = 22.0


class GlobalConfig(BaseSettings):
    """Global configurations."""

    # These variables will be loaded from the .env file. However, if
    # there is a shell environment variable having the same name,
    # that will take precedence.

    APP_CONFIG: AppConfig = AppConfig()

    # define global variables with the Field class
    ENV_STATE: Optional[str] = Field(None, env="ENV_STATE")

    # environment specific variables do not need the Field class
    REDIS_HOST: Optional[str] = None
    REDIS_PORT: Optional[int] = None
    REDIS_PASS: Optional[str] = None

    class Config:
        """Loads the dotenv file."""

        env_file: str = ".env"


class DevConfig(GlobalConfig):
    """Development configurations."""

    class Config:
        env_prefix: str = "DEV_"


class ProdConfig(GlobalConfig):
    """Production configurations."""

    class Config:
        env_prefix: str = "PROD_"


class FactoryConfig:
    """Returns a config instance dependending on the ENV_STATE variable."""

    def __init__(self, env_state: Optional[str]):
        self.env_state = env_state

    def __call__(self):
        if self.env_state == "dev":
            return DevConfig()

        elif self.env_state == "prod":
            return ProdConfig()


cnf = FactoryConfig(GlobalConfig().ENV_STATE)()
print(cnf.__repr__())
```

The print statement of the last line in the above code block is to inspect the _active
configuration_ class. You'll soon learn what I meant by the term _active configuration_. You
can comment out the last line while using the code in production. Let's explain what's going
on with each of the classes defined above.

#### AppConfig

The `AppConfig` class defines the config variables required for you API's internal logic. In
this case I'm not loading the variables from the `.env` file, rather defining them directly
in the class. You can also define and import them from another `app_configs.py` file if
necessary but they shouldn't be placed in the `.env` file. For data validation to work, you
have to inherit from Pydantic's `BaseModel` and annotate the attributes using type hints
while constructing the `AppConfig` class. Later, this class is called from the
`GlobalConfig` class to build a nested data structure.

#### GlobalConfig

`GlobalConfig` defines the variables that propagates through other environment classes and
the attributes of this class are globally accessible from all other environments. In this
class, the variables are loaded from the `.env` file. In the `.env` file, global variables
don't have any environment specific prefixes like `DEV_` or `PROD_` before them. The class
`GlobalConfig` inherits from Pydantic's `BaseSettings` which helps to load and read the
variables from the `.env` file. The `.env` file itself is loaded in the nested `Config`
class. Although the environment variables are loaded from the `.env` file, Pydantic also
loads your actual shell environment variables at the same time. From Pydantic's
[documentation]:

> Even when using a dotenv file, Pydantic will still read environment variables as well as
> the dotenv file, **environment variables will always take priority over values loaded from
> a dotenv file**.

This means you can keep the sensitive variables in your `.bashrc` or `zshrc` and Pydantic
will inject them during runtime. It's a powerful feature, as it implies that you can easily
keep the insensitive variables in your `.env` file and include that to the version control
system. Meanwhile the sensitive information should be injected as a shell environment
variable. For example, although I've defined an attribute called `REDIS_PASS` in the
`GlobalConfig` class, there is no mention of any `REDIS_PASS` variable in the `.env` file.
So normally, it returns `None` but you can easily inject a _password_ into the `REDIS_PASS`
variable from the shell. Assuming that you've set up your `venv` and installed the
dependencies, you can test it by copying the contents of the above code snippet in file
called `configs.py` and running the commands below:

```sh
export DEV_REDIS_PASS=ubuntu
python configs.py
```

This should printout:

```txt
>>> DevConfig(
...     ENV_STATE='dev',
...     APP_CONFIG=AppConfig(VAR_A=33, VAR_B=22.0),
...     REDIS_PASS='ubuntu',
...     REDIS_HOST='127.0.0.1', REDIS_PORT=4000)
```

Notice how your injected `REDIS_PASS` has appeared in the printed config class instance.
Although I injected `DEV_REDIS_PASS` into the environment variable, it appeared as
`REDIS_PASS` inside the `DevConfig` instance. This is convenient because you won't need to
change the name of the variables in your codebase when you change the environment. To
understand why it printed an instance of the `DevConfig` class, refer to the
[FactoryConfig](#factoryconfig) section.

#### DevConfig

`DevConfig` class inherits from the `GlobalConfig` class and it can define additional
variables specific to the development environment. It inherits all the variables defined in
the `GlobalConfig` class. In this case, the `DevConfig` class doesn't define any new
variable.

The nested `Config` class inside `DevConfig` defines an attribute `env_prefix` and assigns
`DEV_` prefix to it. This helps Pydantic to read your prefixed variables like
`DEV_REDIS_HOST`, `DEV_REDIS_PORT` etc without you having to explicitly mention them.

#### ProdConfig

`ProdConfig` class also inherits from the `GlobalConfig` class and it can define additional
variables specific to the production environment. It inherits all the variables defined in
the `GlobalConfig` class. In this case, like `DevConfig` this class doesn't define any new
variable.

The nested `Config` class inside `ProdConfig` defines an attribute `env_prefix` and assigns
`PROD_` prefix to it. This helps Pydantic to read your prefixed variables like
`PROD_REDIS_HOST`, `PROD_REDIS_PORT` etc without you having to explicitly mention them.

#### FactoryConfig

`FactoryConfig` is the controller class that dictates which config class should be activated
based on the environment state defined as `ENV_STATE` in the `.env` file. If it finds
`ENV_STATE="dev"` then the control flow statements in the `FactoryConfig` class will
activate the development configs _(DevConfig)_. Similarly, if `ENV_STATE="prod"` is found
then the control flow will activate the production configs _(ProdConfig)_. Since the current
environment state is `ENV_STATE="dev"`, when you run the code, it prints an instance of the
activated `DevConfig` class. This way, you can assign different values to the same variable
based on different _environment contexts_.

You can also dynamically change the environment by changing the value of `ENV_STATE` on your
shell. Run:

```sh
EXPORT ENV_STATE="prod"
python configs.py
```

This time the config instance should change and print the following:

```txt
>>> ProdConfig(
...     ENV_STATE='prod',
...     APP_CONFIG=AppConfig(VAR_A=33, VAR_B=22.0),
...     REDIS_PASS='ubuntu', REDIS_HOST='127.0.0.2',
...     REDIS_PORT=5000)
```

## Accessing the configs

Using the config variables is easy. Suppose you want use the variables in file called
`app.py`. You can easily do so as shown in the following code block:

```py
# app.py

from configs import cnf


APP_CONFIG = cnf.APP_CONFIG
VAR_A = APP_CONFIG.VAR_A  # this is a nested config
VAR_B = APP_CONFIG.VAR_B
REDIS_HOST = cnf.REDIS_HOST  # this is a top-level config
REDIS_PORT = cnf.REDIS_PORT


print(APP_CONFIG)
print(VAR_A)
print(VAR_B)
print(REDIS_HOST)
print(REDIS_PORT)
```

This should print out:

```txt
>>> ProdConfig(
...     ENV_STATE='prod',
...     APP_CONFIG=AppConfig(VAR_A=33, VAR_B=22.0),
...     REDIS_PASS='ubuntu',
...     REDIS_HOST='127.0.0.2',
...     REDIS_PORT=5000)

VAR_A=33 VAR_B=22.0
33
22.0
127.0.0.2
5000
```

## Extending the pipeline

The modular design demonstrated above is easy to maintain and extend in my opinion.
Previously, for simplicity, I've defined only two environment scopes; development and
production. Let's say you want to add the configs for your _staging environment_.

- First you'll need to add those _staging_ variables to the `.env` file.

```txt
...

STAGE_REDIS_HOST="127.0.0.3"
STAGE_REDIS_PORT="6000"

...

```

- Then you've to create a class named `StageConfig` that inherits from the `GlobalConfig`
  class. The architecture of the class is similar to that of the `DevConfig` or `ProdConfig`
  class.

```py
# configs.py
...


class StageConfig(GlobalConfig):
    """Staging configurations."""

    class Config:
        env_prefix: str = "STAGE_"


...
```

- Finally, you'll need to insert an `ENV_STATE` logic into the control flow of the
  `FactoryConfig` class. See how I've appended another if-else block to the previous (prod)
  block.

```py
# configs.py
...

class FactoryConfig:
    """Returns a config instance depending on the ENV_STATE variable."""

    def __init__(self, env_state: Optional[str]):
        self.env_state = env_state

    def __call__(self):
        if self.env_state == "dev":
            return DevConfig()

        elif self.env_state == "prod":
            return ProdConfig()

        elif self.env_state == "stage"
            return StageConfig()
...
```

To see your new addition in action just change the `ENV_STATE` to "stage" in the `.env` file
or export it to your shell environment.

```txt
export ENV_STATE="stage"
python configs.py
```

This will print out an instance of the class `StageConfig`.

## Remarks

The above workflow works perfectly for my usage scenario. So subjectively, I feel like it's
an elegant solution to a very icky problem. Your mileage will definitely vary.

[^1]: [Flask](https://github.com/pallets/flask)

[^2]: [FastAPI](https://github.com/tiangolo/fastapi)

[^3]: [Dynaconf](https://github.com/rochacbruno/dynaconf)

[^4]: [environ-config](https://github.com/hynek/environ-config)

[^5]: [Pydantic](https://github.com/samuelcolvin/pydantic)

[^6]: [python-dotenv](https://github.com/theskumar/python-dotenv)

[^7]: [PEP-484](https://www.python.org/dev/peps/pep-0484/)

[^8]:
    [Settings management with pydantic](https://pydantic-docs.helpmanual.io/usage/settings/)
    [^8]

[^9]: [Flask config management](https://flask.palletsprojects.com/en/1.1.x/config/) [^9]
