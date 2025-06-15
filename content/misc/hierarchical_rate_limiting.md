---
title: Hierarchical rate limiting with Redis sorted sets
date: 2025-01-12
tags:
    - Database
    - Python
    - System
---

Recently at work, we ran into this problem:

We needed to send Slack notifications for specific events but had to enforce rate limits to
avoid overwhelming the channel. Here's how the limits worked:

- **Global limit**: Max 100 requests every 30 minutes.
- **Category limit**: Each event type (e.g., errors, warnings) capped at 10 requests per 30
  minutes.

Now, imagine this:

1. There are 20 event types.
2. Each type hits its 10-notification limit in 30 minutes.
3. That's 200 requests total, but the global limit only allows 100. So, 100 requests must be
   dropped—even if some event types still have room under their individual caps.

This created a **hierarchy of limits**:

1. Category limits keep any event type from exceeding 10 requests.
2. The global limit ensures the combined total stays under 100.

Every 30 minutes, the system resets. Here are two issues that could arise:

- If some event types are busier, the global limit could block quieter ones.
- Even with room under the global limit, some event types might still hit their category
  caps.

In our case, the event types are limited, and the category limits are both uniform and
significantly smaller than the global limit, so this isn't a concern.

## Redis sorted sets

The notification sender service runs on multiple instances, each processing events and
sending notifications independently. Without a shared system to enforce rate limits, these
instances would maintain separate counters for global and category-specific limits. This
would create inconsistencies because no instance would have a complete view of the overall
activity, leading to conflicts and potential exceedance of limits.

Redis provides a centralized state that all instances can access, ensuring they share the
same counters for rate limits. This removes inconsistencies and makes rate limiting
reliable, even when the notification sender scales to multiple instances.

Sorted sets in Redis track notifications within a rolling time window by using timestamps as
scores, which keeps entries ordered by time. The implementation:

- Maintains a global sorted set to enforce the overall limit (e.g., 100 notifications per 30
  minutes).
- Uses category-specific sorted sets to enforce category limits for each event type (e.g.,
  10 notifications per 30 minutes for errors, warnings, etc.).

The limits are enforced with two Redis commands:

- `ZREMRANGEBYSCORE` removes entries with timestamps outside the rolling time window,
  keeping only recent notifications.
- `ZCARD` counts the remaining entries in a set to check whether the global or
  category-specific limits have been reached.

## Lua script

Instead of embedding the rate-limiting logic directly into the notification sender, we chose
to implement it as a Lua script in Redis. While we could write the logic in the code and run
it in a Redis pipeline, we opted not to, for the following reasons:

- A dedicated script keeps the rate-limiting logic separate and independently auditable.
- It saves a few TCP calls, as the entire logic runs within Redis itself.
- And most importantly, I wanted to write some Lua.

The script is as follows:

```lua
-- rate_limiter.lua
local function check_rate_limit(
  global_key, category_key, global_limit, category_limit, window
)
  -- Get the current timestamp in seconds (including microseconds)
  local current_time_raw = redis.call('TIME') -- Returns {seconds, microseconds}
  local current_time = current_time_raw[1] + (current_time_raw[2] / 1000000)

  -- Step 1: Remove expired entries
  redis.call('ZREMRANGEBYSCORE', global_key, 0, current_time - window)
  redis.call('ZREMRANGEBYSCORE', category_key, 0, current_time - window)

  -- Step 2: Check the global limit
  local global_count = redis.call('ZCARD', global_key)
  if global_count >= global_limit then
      return 0 -- Reject the request if the global limit is reached
  end

  -- Step 3: Check the category-specific limit
  local category_count = redis.call('ZCARD', category_key)
  if category_count >= category_limit then
      return 0 -- Reject the request if the category limit is reached
  end

  -- Step 4: Add the current notification to the sorted sets
  redis.call('ZADD', global_key, current_time, current_time)
  redis.call('ZADD', category_key, current_time, current_time)

  return 1 -- Allow the request
end

-- Parameters passed to the script:
-- KEYS[1]: The Redis key for the global sorted set
-- KEYS[2]: The Redis key for the category-specific sorted set
-- ARGV[1]: Global limit (e.g., 100)
-- ARGV[2]: Category limit (e.g., 10)
-- ARGV[3]: Time window in seconds (e.g., 1800 for 30 minutes)

local global_key = KEYS[1]
local category_key = KEYS[2]
local global_limit = tonumber(ARGV[1])
local category_limit = tonumber(ARGV[2])
local window = tonumber(ARGV[3])

-- Execute the rate-limiting function and return the result
return check_rate_limit(
    global_key, category_key, global_limit, category_limit, window
)
```

The script performs the following operations in order:

1. **Remove expired entries**:

    - It uses `ZREMRANGEBYSCORE` to remove notifications older than the time window
      (`current_time - window`). This ensures that only active notifications are considered
      for the limits.

    - This eliminates the need for additional bookkeeping to remove expired keys.
      `ZREMRANGEBYSCORE` is fast enough to handle the removal of a small number of keys
      during each invocation.

2. **Check the global limit**:

    - `ZCARD` counts the number of active notifications in the global sorted set.
    - If this count equals or exceeds the global limit (e.g., 100), the request is rejected
      (`return 0`).

3. **Check the category-specific limit**:

    - `ZCARD` is used again to count the active notifications for the specific category.
    - If this count equals or exceeds the category limit (e.g., 10), the request is rejected
      (`return 0`).

4. **Add the notification**:

    - If both limits are within bounds, the script uses `ZADD` to insert the current
      notification into both the global and category-specific sorted sets, using a timestamp
      as the score for accurate tracking.

## Using the script

You can load the Lua script from disk, register it with Redis, and call it before invoking
the notification service. If the script returns 0, drop the notification request. If it
returns 1, send the notification. Here's how to do it in Python:

```py
from redis import Redis
from redis.commands.core import Script


def load_lua_script(redis_client: Redis, script_path: str) -> Script:
    with open(script_path, "r") as file:
        lua_script = file.read()
    return redis_client.register_script(lua_script)


def send_notification(
    script: Script,
    global_key: str,
    category_key: str,
    global_limit: int,
    category_limit: int,
    window: int,
    message: str,
) -> None:
    # Check the rate limiter
    result: int = script(
        keys=[global_key, category_key],
        args=[global_limit, category_limit, window],
    )

    if result == 1:
        # Allowed: send the notification
        print(f"Notification sent: {message}")
        # Add actual notification-sending logic here
    else:
        # Blocked: drop the notification
        print(f"Notification dropped (rate limit exceeded): {message}")


if __name__ == "__main__":
    # Connect to Redis
    redis_client = Redis(host="localhost", port=6379, decode_responses=True)

    # Load and register the Lua script
    script_path = "rate_limiter.lua"
    script = load_lua_script(redis_client, script_path)

    # Define rate limiting parameters
    global_key = "rate_limit:global"
    category_key = "rate_limit:category:errors"
    global_limit = 100  # Max 100 requests globally
    category_limit = 10  # Max 10 requests per category
    window = 1800  # 30-minute window

    # Send a single notification
    send_notification(
        script,
        global_key,
        category_key,
        global_limit,
        category_limit,
        window,
        "This is a single notification message",
    )
```

Registering the Lua script loads it from disk once and reuses it, which is faster than
repeatedly loading and evaluating it for each invocation.

To test this, you'll need a running Redis instance. You can run one with Docker:

```sh
docker run --name redis-server -d -p 6379:6379 redis
```

Now, running the script will print:

```txt
Notification sent: This is a single notification message
```

Since this sends a notification only once, the rate limiting isn't apparent yet, but it's
working under the hood and will kick in if any limit is exceeded. To see it in action, you
can attempt to send multiple notifications in a tight loop.

## Testing the rate limiter

You can call the `send_notification` function multiple times to test the rate limiter. Below
is an example that simulates several notification requests in a short loop, giving you a
sense of how many will be allowed versus blocked:

```py
from redis import Redis
import time


def main() -> None:
    # Connect to Redis
    redis_client = Redis(host="localhost", port=6379, decode_responses=True)

    # Load the Lua script
    with open("rate_limiter.lua", "r") as file:
        lua_script = file.read()

    # Register the Lua script
    script = redis_client.register_script(lua_script)

    # Example keys and arguments
    global_key = "rate_limit:global"
    category_key = "rate_limit:category:errors"
    global_limit = 10
    category_limit = 3
    window = 60  # 1 minute in seconds for this test

    # Run the script in a loop
    for i in range(10):
        time.sleep(0.1)
        result = script(
            keys=[global_key, category_key],
            args=[global_limit, category_limit, window],
        )
        message = "Some notification message"
        if result == 1:
            # Allowed: send the notification
            print(f"{i}. Notification sent: {message}")
            # Add actual notification-sending logic here
        else:
            # Blocked: drop the notification
            print(
                f"{i}. Notification dropped (rate limit exceeded): {message}"
            )


if __name__ == "__main__":
    main()
```

This code demonstrates how to test the rate limiter by simulating multiple notification
requests. The Lua script is loaded, registered with Redis, and executed in a loop to
evaluate whether each request is allowed or blocked based on the defined rate limits.

Running this will produce output similar to:

```txt
0. Notification sent: Some notification message
1. Notification sent: Some notification message
2. Notification sent: Some notification message
3. Notification dropped (rate limit exceeded): Some notification message
4. Notification dropped (rate limit exceeded): Some notification message
5. Notification dropped (rate limit exceeded): Some notification message
6. Notification dropped (rate limit exceeded): Some notification message
7. Notification dropped (rate limit exceeded): Some notification message
8. Notification dropped (rate limit exceeded): Some notification message
9. Notification dropped (rate limit exceeded): Some notification message
```

Here, for demonstration, we set the global rate limit to 10 and the category limit to 3 with
a 60-second rolling window. After three successful category notifications (and a total of
three global notifications), the rate limiter rejects additional requests in the same
window, illustrating how both the global and category limits work together.
