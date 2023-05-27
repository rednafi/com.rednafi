---
title: Bulk request Google search indexing with API
date: 2023-05-23
tags:
    - JavaScript
    - API
---

Recently, I've purchased a domain for this blog and migrated the content from
[rednafi.github.io][rednafi.com] to [rednafi.com]. This turned out to be a much bigger
hassle than I originally thought it'd be, mostly because, despite setting redirection
for almost all the URLs from the previous domain to the new one and submitting the new
[sitemap.xml] to the Search Console, Google kept indexing the older domain. To make
things worse, the search engine selected the previous domain as canonical, and no amount
of manual requests were changing the status in the last 30 days. Strangely, I didn't
encounter this issue with Bing, as it reindexed the new site within a week after I
submitted the [sitemap.xml] via their webmaster panel.

While researching this, one potential solution suggested that along with submitting the
sitemap via [Google Search Console][google-search-console], I'd have to make individual
indexing requests for each URL to encourange faster indexing. The problem is, I've got
quite a bit of content on this site, and it'll take forever for me to click through all
the links and request indexing that way. Naturally, I looked for a way to do this
programmatically. Luckily, I found out that there's an [indexing API] that allows you to
make bulk indexing requests programmatically. This has one big advantage—Google
[responds][api-submission] to API requests faster than indexing requests with sitemap
submission.

All you've to do is:

* List out the URLs that need to be indexed.
* Fullfil the [prerequisites][indexing-api] and download the private key JSON file
required to make requests to the API. From the docs:

    > *Every call to the Indexing API must be authenticated with an OAuth token that you
    get in exchange for your private key. Each token is good for a span of time. Google
    provides API client libraries to get OAuth tokens for a number of languages.*

    The private key file will look like this:
    ```json
    {
      "type": "service_account",
      "project_id": "...",
      "private_key_id": "...",
      "private_key": "...",
      "client_email": "...",
      "client_id": "...",
      "auth_uri": "...",
      "token_uri": "...",
      "auth_provider_x509_cert_url": "...",
      "client_x509_cert_url": "...",
      "universe_domain": "..."
    }
    ```

* Use an API client to make the requests.

In my case, this site's [sitemap][sitemap.xml] lists out all the URLs as follows:

```xml
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
xmlns:xhtml="http://www.w3.org/1999/xhtml">
<url>
    <loc>https://rednafi.com/tags/github/</loc>
    <lastmod>2023-05-21T00:00:00+00:00</lastmod>
</url>
<url>
    <loc>https://rednafi.com/tags/javascript/</loc>
    <lastmod>2023-05-21T00:00:00+00:00</lastmod>
</url>
...
```

Here's a NodeJS script that collects the URLs from `sitemap.xml` and makes requests to
the indexing API:

```js
// ES6 import
import { google } from "googleapis";
import { parseString } from "xml2js";
import fetch from "node-fetch";

import pkey from "./google-api-pkey.json" assert { type: "json" };

// Parse the sitemap.xml file and extract the URLs.
async function getUrls(url) {
  try {
    const response = await fetch(url);
    const xml = await response.text();
    let urls;
    parseString(xml, (err, result) => {
      if (err) {
        console.error("Error parsing XML:", err);
        return;
      }

      urls = result.urlset.url.map((url) => url.loc[0]);
    });
    return urls;
  } catch (error) {
    console.error("Error fetching sitemap:", error);
  }
}

// Initialize auth client
const jwtClient = new google.auth.JWT(
  pkey.client_email,
  null,
  pkey.private_key,
  ["https://www.googleapis.com/auth/indexing"],
  null
);

// Perfrom auth and make multiple API calls
jwtClient.authorize(async function (err, tokens) {
  if (err) {
    console.log(err);
    return;
  }

  const options = {
    url: "https://indexing.googleapis.com/v3/urlNotifications:publish",
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${tokens.access_token}`,
    },
    json: {
      url: "",
      type: "URL_UPDATED", // Means we want to request indexing
    },
  };

  try {
    const urls = await getUrls("https://www.rednafi.com/sitemap.xml");

    // There's a bulk endpoint but looping through the list
    // and making multiple requests is just as easy
    for (const url of urls) {
      options.json.url = url;
      const response = await fetch(options.url, {
        method: options.method,
        headers: options.headers,
        body: JSON.stringify(options.json),
      });
      const body = await response.json();
      console.log(body);
    }
  } catch (error) {
    console.error("Error:", error);
  }
});
```

Before executing the script, npm install `googleapis` and `xml2js`. Now running the
script will give you an output similar to this:

```txt
{
  urlNotificationMetadata: {
    url: 'https://rednafi.com/categories/',
    latestUpdate: {
      url: 'https://rednafi.com/categories/',
      type: 'URL_UPDATED',
      notifyTime: '2023-05-27T01:02:35.537421311Z'
    }
  }
}
{
  urlNotificationMetadata: {
    url: 'https://rednafi.com/search/',
    latestUpdate: {
      url: 'https://rednafi.com/search/',
      type: 'URL_UPDATED',
      notifyTime: '2023-05-27T01:02:35.789809492Z'
    }
  }
},
...
```

Here, the `getUrls` function is defined to fetch the `sitemap.xml` file from a specified
URL, parse the XML content, and extract the URLs. It uses the fetch function to retrieve
the file, then uses `xml2js` to parse the XML and extract the URLs from the result.

The script then initializes an authentication client using the imported private key and
specifies the required API scope. The `authorize` function is called to authenticate the
client and obtain access tokens. Inside the authorization callback, the script prepares
the necessary `options` for making API requests to the Google indexing API. It then
calls the `getUrls` function to fetch the URLs from the `sitemap.xml` file. For each
URL, it updates the `options` with the URL and makes a POST request to the Indexing API
to request indexing. The response from the API is then logged to the console.

And we're done!

## Resources

* [Indexing API quickstart][indexing-api]

[rednafi.com]: https://rednafi.com
[indexing-api]: https://developers.google.com/search/apis/indexing-api/v3/quickstart
[api-submission]: https://developers.google.com/search/apis/indexing-api/v3/quickstart#sitemaps
[sitemap.xml]: https://rednafi.com/sitemap.xml
[google-search-console]: https://search.google.com/search-console/about
[cors-proxy]: https://corsproxy.io
