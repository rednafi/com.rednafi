---
title: Building a CORS proxy with Cloudflare Workers
date: 2023-05-21
slug: cors-proxy-with-cloudflare-workers
aliases:
    - /javascript/cors_proxy_with_cloudflare_workers/
tags:
    - JavaScript
    - Networking
    - GitHub
---

Cloudflare absolutely nailed the serverless function DX with Cloudflare Workers[^1].
However, I feel like it's yet to receive widespread popularity like AWS Lambda since as of
now, the service only offers a single runtime—JavaScript. But if you can look past that big
folly, it's a delightful piece of tech to work with. I've been building small tools with it
for a couple of years but never got around to writing about the immense productivity boost
it usually gives me whenever I need to quickly build and deploy a self-contained service.

Recently, I was doing some lightweight frontend work and needed to make some AJAX calls from
one domain to another. Usually, browser's CORS (Cross-Origin Resource Sharing)[^2] policy
will get in your way if you try this. While you're reading this piece, open the dev console
and paste the following `fetch` snippet:

```js
fetch("https://mozilla.org")
  .then((response) => response.text())
  .then((data) => {
    // Do something with the received data
    console.log(data);
  })
  .catch((error) => {
    // Handle any errors that occurred during the request
    console.error("Error:", error);
  });
```

This snippet will attempt to make a `GET` request from <https://rednafi.com> to
<https://mozilla.org>. However, the client's CORS policy won't allow you to make an AJAX
request like this and load external resources into the current site. On your console, you'll
see an error message like this:

```txt
Access to fetch at 'https://mozilla.org/' from origin 'https://rednafi.com'
has been blocked by CORS policy: No 'Access-Control-Allow-Origin' header is
present on the requested resource.

If an opaque response serves your needs, set the request's mode to
'no-cors' to fetch the resource with CORS disabled.
```

This is a good security measure. Without CORS, a malicious script could make a request to a
server in another domain and access the resources that the user of the page is not intended
to have access to. So much has been said and written about CORS that I won't even attempt to
explain it here. Here's another high-level introduction to the concept[^3].

## CORS proxy

While CORS is generally a good thing, it can be quite annoying when you're trying to build
something that needs access to external resources. In those cases, you'll have to mess
around with the origin server and add a few headers that the browser can understand before
it allows you to load those external resources. But sometimes you don't have access to the
origin server or simply don't want to deal with modifying the server's response headers
every time you need to access external resources. That's where CORS proxies can come in
handy.

> A CORS proxy server acts as a bridge between your client and the target server. It
> receives your request and forwards it to the target server with a modified origin header
> so that the target server thinks the request is coming from the same origin as itself.

This way, you can bypass the same-origin policy of browsers and access resources from
different domains. I usually use free proxies like cors.sh[^4] to bypass CORS restrictions.
You can drop this snippet to your browser's console and this time it'll allow you to load
the contents of <https://mozilla.org> from <https://rednafi.com>:

```js
// Notice how we're prepending CORS URL before the target URL
fetch("https://proxy.cors.sh/https://mozilla.org")
  .then((response) => response.text())
  .then((data) => {
    // Do something with the received data
    console.log(data);
  })
  .catch((error) => {
    // Handle any errors that occurred during the request
    console.error("Error:", error);
  });
```

The target server's response looks somewhat like this:

```html
<!doctype html>
<html class="windows no-js" lang="en" dir="ltr" data-country-code="US"
data-latest-firefox="113.0.1" data-esr-versions="102.11.0"
data-gtm-container-id="GTM-MW3R8V" data-gtm-page-id="Homepage"
data-stub-attribution-rate="1.0" data-convert-project-id="10039-1003343">
<head>
...
```

If you want to learn more about how CORS proxies work, here's a fantastic resource[^5] that
explains the inner machinery in more detail.

## Free proxy servers can be pernicious

Using a free CORS proxy server can be dangerous as it might spill the beans on your requests
and data to some random service you don't know or trust. Since it plays as a middleman
between your app and the resource you're after, it could potentially snoop on, mess with, or
keep tabs on your requests and data. Plus, some of those free CORS proxy servers might have
restrictions on the size, type, or number of requests they can handle, or they might not
even support HTTPS or other security bells and whistles.

## Build your own CORS proxy with Cloudflare Workers

With all the intros out of the way, here's how CloudFlare Workers afforded me to prop up a
CORS proxy in less than half an hour. If you're impatient and just want to take a look at
the service in its full glory then head over here[^6]. GitHub Actions deploys the service
automatically to CloudFlare Workers every time a change is pushed to the `main` branch.

### Installing the prerequisites

Assuming you have `node` installed on your system, you can fetch the wrangler[^7] CLI with
the following command:

```sh
npm install -g wrangler
```

This will allow us to develop and test the service locally.

### Bootstrapping the service

Create a new directory where you want to develop your service and bring it under source
control. Now, run:

```sh
npm create cloudflare@latest
```

The CLI will guide you through the entire bootstrapping process interactively. You'll have
to create a Cloudflare account (if you don't have one already) and log into the dashboard.
Then it'll prompt you to deploy your first `hello-world` API endpoint that you can
immediately start to play with without doing anything else. Being able to see the serverless
function in action within like 5 minutes gave me a huge dopamine boost that AWS Lambda never
could. You can see the interactive bootstrapping section here:

<details>
<summary><strong>Complete CLI output...</strong></strong></summary>

```txt
using create-cloudflare version 2.0.7

╭ Create an application with Cloudflare Step 1 of 3
│
├ Where do you want to create your application?
│ dir cors-proxy
│
├ What type of application do you want to create?
│ type "Hello World" script
│
├ Do you want to use TypeScript?
│ typescript no
│
├ Copying files from "simple" template
│
╰ Application created

╭ Installing dependencies Step 2 of 3
│
├ Installing dependencies
│ installed via `npm install`
│
╰ Dependencies Installed

╭ Deploy with Cloudflare Step 3 of 3
│
├ Do you want to deploy your application?
│ yes deploying via `npm run deploy`
│
├ Logging into Cloudflare This will open a browser window
│ allowed via `wrangler login`
│
├ Deploying your application
│ deployed via `npm run deploy`
│
├  SUCCESS  View your deployed application at
│  https://cors-proxy.rednafi.workers.dev (this may take a few mins)
│
│ Run the development server npm run dev
│ Deploy your application npm run deploy
│ Read the documentation https://developers.cloudflare.com/workers
│ Stuck? Join us at https://discord.gg/cloudflaredev
│
╰ See you again soon!
```

</details>

Running the interactive session will create the following directory structure:

```txt
├── src
│   └── worker.js
├── package-lock.json
├── package.json
└── wrangler.toml
```

### Developing the CORS proxy

We'll write our proxy server in `src/worker.js` file. Copy the following JS snippet and
paste it to the file:

```js
export default {
  async fetch(request, env, ctx) {
    // Extract method, url and headers from the incoming request object.
    const { method, url, headers } = request;

    // Extract destination url from the query string.
    const destUrl = new URL(url).searchParams.get("url");

    // If the destination url is not present, return 400.
    if (!destUrl) {
      return new Response("Missing destination URL.", { status: 400 });
    }

    // If the request method is OPTIONS, return CORS headers.
    if (
      method === "OPTIONS" &&
      headers.has("Origin") &&
      headers.has("Access-Control-Request-Method")
    ) {
      const responseHeaders = {
        "Access-Control-Allow-Origin": headers.get("Origin"),
        "Access-Control-Allow-Methods": "*", // Allow all methods
        "Access-Control-Allow-Headers": headers.get(
          "Access-Control-Request-Headers"
        ),
        "Access-Control-Max-Age": "86400",
      };
      return new Response(null, { headers: responseHeaders });
    }

    const proxyRequest = new Request(destUrl, {
      method,
      headers: {
        ...headers,
        Origin: "",
      },
    });

    try {
      const response = await fetch(proxyRequest);
      const responseHeaders = new Headers(response.headers);
      responseHeaders.set("Access-Control-Allow-Origin", "*");
      responseHeaders.set("Access-Control-Allow-Credentials", "true");
      responseHeaders.set("Access-Control-Allow-Methods", "*");

      return new Response(response.body, {
        status: response.status,
        statusText: response.statusText,
        headers: responseHeaders,
      });
    } catch (error) {
      return new Response("Error occurred while fetching the resource.", {
        status: 500,
      });
    }
  },
};
```

The first section of the code deals with extracting relevant information from the incoming
request. It destructures the `method`, `url`, and `headers` properties from the `request`
object, which represents the client's request.

Next, it extracts the destination URL from the query string. It extracts the URL parameter
using the `searchParams.get()` method. If the destination URL isn't provided, the function
returns a `Response` object with an error message and a status code of 400 (Bad Request).

The code then checks if the request method is `OPTIONS`. The `OPTIONS` method is used in
CORS preflight[^8] requests to determine if the actual request is safe to send. If the
request is an `OPTIONS` request and contains specific headers indicating a CORS preflight
request (`Origin` and `Access-Control-Request-Method`), the function generates a response
with appropriate CORS headers. The response headers include `Access-Control-Allow-Origin` to
reflect the client's origin, `Access-Control-Allow-Methods` set to `*`, allowing any HTTP
method, `Access-Control-Allow-Headers` based on the requested headers, and
`Access-Control-Max-Age` set to `86400` seconds (one day) to cache the preflight response.

If the request is not an `OPTIONS` request or doesn't meet the CORS preflight conditions,
the code continues execution. It creates a new `Request` object named `proxyRequest` uses
the extracted destination URL and sets the method and headers of the original request. The
`Origin` header is removed to prevent CORS restrictions when forwarding the request.

The subsequent code performs the actual request forwarding. It uses `fetch` to send the
`proxyRequest` to the destination URL. If the fetch is successful, the code proceeds to
process the response. It creates a new `Headers` object from the response's headers and
modifies them to include the necessary CORS headers.

Finally, the function constructs a `Response` object using the response `body`, `status`,
`statusText`, and modified `headers`. If an error occurs during the fetch operation, the
code catches the error and returns a `Response` object with an error message and a status
code of 500 (Internal Server Error).

Once you've pasted the snippet, you can redeploy the service from your local machine with:

```sh
wrangler deploy
```

This will deploy the service immediately:

```txt
 ⛅️ wrangler 3.0.0
------------------
Total Upload: 1.52 KiB / gzip: 0.57 KiB
Uploaded cors-proxy (0.51 sec)
Published cors-proxy (0.38 sec)
  https://<your-deployed-service>
Current Deployment ID: f300ac99-c15e-4e30-a910-a56d81c10b95
```

I've removed my root domain from the above output since I'm using the free version of
Workers and don't want people to exhaust my free request quota. Haha, security by obscurity!
But once you've deployed your proxy server, you can go to the following URL from your
browser:

```txt
https://<your-deployed-service>/?url=https://mozilla.com
```

This will send you to the Mozilla website through the deployed function. Now you can use it
just like the free CORS proxy. Try it out by dropping the following snippet to your browser
console. This is exactly the same as the previous `fetch` snippet but the only difference is
this time, we're using our own proxy server that we control:

```js
// Notice how we're prepending CORS URL before the target URL
fetch("https://<your-deployed-service>?url=https://mozilla.org")
  .then((response) => response.text())
  .then((data) => {
    // Do something with the received data
    console.log(data);
  })
  .catch((error) => {
    // Handle any errors that occurred during the request
    console.error("Error:", error);
  });
```

Don't forget to replace the `<your-deployed-service>` URL with your own service. This will
result in a successful request. You can also interactively send requests to the destination
URLs via the Cloudflare Workers dashboard. Go to your Cloudflare dashboard, head over to the
Workers section, and select your deployed serverless function:

![cloudflare worker editor][image_1]

### Deploying the service with GitHub Actions

For one-off services, `wrangler deploy` in the local machine works perfectly but I usually
don't consider a project fully done until I've automated away the whole process. So, I wrote
a quick GitHub Actions workflow to run the linters and deploy the service automatically when
a new commit is pushed to the `main` branch. Here's how it looks:

```yml
# .github/workers/ci.yml

name: Deploy

on:
  push:
    branches:
      - main

  # Allow running this workflow manually.
  workflow_dispatch:


jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: "lts/*"
          cache: npm
          cache-dependency-path: cors-proxy/package-lock.json
      - name: Install dependencies
        working-directory: ./cors-proxy
        run: |
          npm install
      - name: Run linter
        working-directory: ./cors-proxy
        run: |
          npx prettier --check .

  deploy:
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v3
      - name: Publish
        uses: cloudflare/wrangler-action@2.0.0
        with:
          apiToken: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          workingDirectory: "cors-proxy"
```

For this to work, you'll need to create a Cloudflare API key[^9] and add it to the GitHub
Secrets[^10] of your proxy server's repository. Here's the complete workflow[^11] file.

[^1]: [Cloudflare Workers](https://workers.cloudflare.com/)

[^2]: [CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)

[^3]:
    [Let's talk about CORS](https://medium.com/bigcommerce-developer-blog/lets-talk-about-cors-84800c726919)

[^4]: [cors.sh](https://cors.sh/)

[^5]: [CORS proxy](https://httptoolkit.com/blog/cors-proxies)

[^6]: [Complete implementation](https://github.com/rednafi/cors-proxy)

[^7]: [Wrangler](https://developers.cloudflare.com/workers/wrangler/)

[^8]:
    [Preflight request](https://developer.mozilla.org/en-US/docs/Glossary/Preflight_request)

[^9]:
    [Cloudflare API Key](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/)

[^10]:
    [GitHub Secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets)

[^11]: [CI](https://github.com/rednafi/cors-proxy/blob/main/.github/workflows/ci.yml)

[image_1]:
    https://blob.rednafi.com/static/images/cors_proxy_with_cloudflare_workers/img_1.png
