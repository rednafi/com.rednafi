---
title: Python logging quirks in AWS Lambda environment
date: 2022-10-20
slug: logging-quirks-in-lambda-environment
aliases:
    - /python/logging_quirks_in_lambda_environment/
tags:
    - Python
    - AWS
---

At my workplace, while working on a Lambda[^1] function, I noticed that my Python logs
weren't appearing on the corresponding Cloudwatch[^2] log dashboard. At first, I thought
that the function wasn't picking up the correct log level from the environment variables. We
were using serverless[^3] framework and GitLab CI to deploy the function, so my first line
of investigation involved checking for missing environment variables in those config files.

However, I quickly realized that the environment variables were being propagated to the
Lambda function as expected. So, the issue had to be coming from somewhere else. After
perusing through some docs, I discovered from the source code of Lambda Python Runtime
Interface Client[^4] that AWS Lambda Python runtime pre-configures[^5] a logging handler
that modifies the format of the log message, and also adds some metadata to the record if
available. What's not pre-configured though is the log level. This means that no matter the
type of log message you try to send, it won't print anything.

According to the docs[^6], to make your logging work in the Lambda environment, you'll only
need to set the log level for the root logger like this:

```py
# src.py
import logging

# Get the root logger.
logger = logging.getLogger()

# Set the log level to the logger object.
logger.setLevel(logging.INFO)

# Use the logger. This will print the message only in the Lambda runtime.
logger.info("Hello from Lambda!")
```

While this does make the log messages appear on the Cloudwatch dashboard, it doesn't work
whenever you'll need to introspect the logs in your local Python interpreter. If you execute
the above snippet locally, you won't see any log message on your console. That's because
here we're only setting the log level for the root logger and we haven't defined any
handler. To fix the local logging, you'll need to add a handler to the logger as and set the
log level on it as follows:

```py
# src.py
import logging

# Get the root logger.
logger = logging.getLogger()

# Create a handler.
s_handler = logging.StreamHandler()

# Link the handler to the logger.
logger.addHandler(s_handler)

# Set the log level to the logger.
logger.setLevel(logging.INFO)

# Use the logger. This will print the message in the local environment.
logger.info("This is an info message.")
```

In the Lambda Python runtime, the root logger is already pre-configured to have modified
handlers. The snippet above first adds another handler to the logger and sets the log level.
So technically, the root logger will contain two handlers in the Lambda environment and
print every log message twice with different handlers. However, you won't see the duplicate
messages in your local environment since the local logger will have only the one handler
that we've defined. So, the logger will behave differently in the two environments; not
good.

> Having multiple stream handlers on the root logger that send the message to the stdout
> will print every log message twice.

So, this still doesn't do what we want. Besides, sometimes in the local environment, I just
want to use `logging.basicConfig` and start logging with minimal configuration. The goal
here is to configure the root logger in a way that doesn't conflict with Lambda's
pre-configured handlers and also works locally without any side effects. Here's what I've
found that works:

```py
# src.py
import logging

# If the logger has pre-configured handlers, set the log level to the
# root logger only. This branch will get executed in the Lambda runtime.
if logging.getLogger().hasHandlers():
    logging.getLogger().setLevel(logging.INFO)
else:
    # Just configure with basicConfig for local usage. This branch will
    # get executed in the local environment.
    logging.basicConfig(level=logging.INFO)

# Use the logger.
logging.info("This is an info message.")
```

The above snippet first inspects whether the root logger contains any handlers and if it
does then sets the log level for the root logger. Otherwise, it just configures the logger
with `basicConfig` for local development. This will print out the log messages both in the
local and Lambda environment and won't suffer from any side effects like message
duplication. It'll also make sure that the pre-configured formatting of the log message is
kept intact.

[^1]: [AWS Lambda](https://aws.amazon.com/lambda/)

[^2]: [Cloudwatch](https://aws.amazon.com/cloudwatch/)

[^3]: [Serverless framework](https://www.serverless.com/)

[^4]:
    [Lambda Python Runtime Interface Client](https://github.com/aws/aws-lambda-python-runtime-interface-client)

[^5]:
    [Pre-configured root logger in the Lambda environment](https://github.com/aws/aws-lambda-python-runtime-interface-client/blob/970e9c1d2613e0ce9c388547c76ac30992ad0e96/awslambdaric/bootstrap.py#L376-L385)

[^6]:
    [AWS Lambda function logging in Python](https://docs.aws.amazon.com/lambda/latest/dg/python-logging.html)

[^7]:
    [Using Python logging with AWS Lambda](https://stackoverflow.com/questions/37703609/using-python-logging-with-aws-lambda)
    [^7]
