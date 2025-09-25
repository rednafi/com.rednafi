---
title: Using DNS record to share text data
date: 2023-07-17
slug: dns-record-to-share-text
aliases:
    - /misc/dns_record_to_share_text/
tags:
    - Shell
    - TIL
    - Networking
---

This morning, while browsing Hacker News, I came across a neat trick[^1] that allows you to
share textual data by leveraging DNS TXT records. It can be useful for sharing a small
amount of data in environments that restrict IP but allow DNS queries, or to bypass
censorship.

To test this out, I opened my domain registrar's panel and created a new TXT type DNS entry
with a base64 encoded message containing the poem **A Poison Tree** by William Blake. The
message can now be queried and decoded with the following shell command:

```sh
dig +short _poem.rednafi.com TXT | sed 's/[\" ]//g' | base64 -d
```

The command uses `dig` to query a TXT DNS record for `_poem.rednafi.com`, removes any double
quotes and spaces from the record value via `sed`, and then decodes the base64-encoded value
via `base64` to retrieve the original plaintext message that was stored in the TXT record.
Running this will return the decoded content of the record:

```txt
I was angry with my friend;
I told my wrath, my wrath did end.
I was angry with my foe:
I told it not, my wrath did grow.

And I watered it in fears,
Night & morning with my tears:
And I sunned it with smiles,
And with soft deceitful wiles.

And it grew both day and night.
Till it bore an apple bright.
And my foe beheld it shine,
And he knew that it was mine.

And into my garden stole,
When the night had veiled the pole;
In the morning glad I see;
My foe outstretched beneath the tree.
```

You can also encode image data and retrieve it in a similar manner. If your data is too
large to fit in a single record, you can split it into multiple records and concatenate them
on the receiving end.

However, there are some limitations to this approach. RFC 1035[^2] says that the total size
of a DNS resource record cannot exceed 65535 bytes. Also, the maximum length of the actual
text value in a single TXT record is 255 bytes or characters. This doesn't give us much room
to tunnel large amounts of data. Plus, DNS has well-known vulnerabilities like MITM attacks,
injection issues, cache poisoning, and DoS. So I'd refrain from transferring any data in
this manner that requires a layer of security. Protocols like DANE and DNSSEC aim to address
some of these concerns but their adoption is spotty at best. Still, I found the idea of
using DNS records as a simple database quite clever!

[^1]: [Use DNS TXT to share information](https://news.ycombinator.com/item?id=36754366)

[^2]:
    [Domain names — implementation & specification](https://www.rfc-editor.org/rfc/rfc1035)
