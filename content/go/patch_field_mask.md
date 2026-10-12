---
title: "Partial updates with field masks"
slug: patch-field-mask
date: 2026-10-12
description: >-
    Keep the value and the update intent in separate fields of a PATCH request.
tags:
    - Go
    - API
aliases: []
discussions: []
mermaid: false
type_label: ""
atprotoPath: /go/patch-field-mask/
atUri: "at://did:plc:fgtm2c26vfcj74rfmeggbyqj/site.standard.document/3mxplcfemlb2q"
---

u/SnooSongs6758 [asked on r/golang]:

> I'm creating a REST API using Golang `v1.25`, Gin `v1.10.0`, and Validator `v10.20.0`. I
> have a PATCH endpoint where, when the `address` attribute is:
>
> - `undefined` -> keep the original data
> - `null` -> delete the original data
> - contains something -> replace the original data with the new
>
> Right now I'm using a pointer for it, so I can know when it's `null`. The problem is that
> I couldn't find a way to differentiate when the attribute is `undefined` or `null`.
>
> Does anyone know a way to check if a specific attribute is `undefined`?

---

While the answers there explore how to support this case, I think we can simplify the design
and make our lives considerably easier. Currently the `address` field does this:

- a string value updates the stored address in place
- `null` deletes the stored address
- and `undefined` signals a missing value, which keeps the original data

It's mixing value and intent within a single field. We can use `address` for the plain value
and another field, `update_mask`, to list the fields the client wants to change. Google's
API design guide calls this a [field mask]. Protobuf has the same thing as its [FieldMask]
type.

Over HTTP, the client sends a `PATCH` with the new values in the body and the mask in the
`update_mask` query parameter, like `?update_mask=name,address`. The server changes only the
fields named in the mask.

Say the server has this user stored. Each request below starts from it:

```json
{"id":"1","name":"Alice","address":"Old Street 1"}
```

To update the address, the client sets the `update_mask` query parameter to `address` and
sends the new value in the body. The server changes the address and returns the updated
user:

```sh {mark="update_mask=address"}
# request
PATCH /users/1?update_mask=address HTTP/1.1
Content-Type: application/json

{"address": "New Street 2"}

# response
HTTP/1.1 200 OK
Content-Type: application/json

{"id":"1","name":"Alice","address":"New Street 2"}
```

To delete the address, the query parameter stays `update_mask=address`, and the body sends
`null`:

```sh {mark="update_mask=address"}
# request
PATCH /users/1?update_mask=address HTTP/1.1
Content-Type: application/json

{"address": null}

# response
HTTP/1.1 200 OK
Content-Type: application/json

{"id":"1","name":"Alice","address":""}
```

An empty body, `{}`, does the same thing, because `address` is still in the mask.

If the client only wants to change the name, it sends `update_mask=name`. The server leaves
`address` alone because the mask doesn't name it:

```sh {mark="update_mask=name"}
# request
PATCH /users/1?update_mask=name HTTP/1.1
Content-Type: application/json

{"name": "Bob"}

# response
HTTP/1.1 200 OK
Content-Type: application/json

{"id":"1","name":"Bob","address":"Old Street 1"}
```

On the Go side, the asker used a pointer so the handler could see `null`. With pointers, the
struct looks like this:

```go
type User struct {
    ID      string  `json:"id"`
    Name    *string `json:"name"`
    Address *string `json:"address"`
}
```

A nil `Address` can mean the client sent `null`, or it can mean the client didn't send the
key at all. That's the problem from the question.

With the mask, the handler doesn't need to tell them apart:

- `address` in the mask and a string in the body: the address gets updated
- `address` in the mask and `null` or no key in the body: the decoded address is `""`, so
  the address gets cleared
- `address` not in the mask: the handler ignores the address in the body

None of these cases need nil, so plain strings are enough:

```go
type User struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Address string `json:"address"`
}
```

In the handler, `in` is the request body and `u` is the stored user. The loop goes through
the mask and copies each named field from `in` to `u`. An unknown field name gets a 400:

```go {hl_lines=["18","20"]}
func (s *Server) patchUser(w http.ResponseWriter, r *http.Request) {
    mask := r.URL.Query().Get("update_mask")
    // ...

    var in User
    
    // json v2 API
    if err := json.UnmarshalRead(r.Body, &in); err != nil {
        http.Error(w, "invalid JSON body", http.StatusBadRequest)
        return
    }

    // ...
    u, ok := s.users[r.PathValue("id")]
    // ...

    for path := range strings.SplitSeq(mask, ",") {
        switch path {
        case "name":
            u.Name = in.Name
        case "address":
            u.Address = in.Address
        default:
            http.Error(w, "unknown field in update_mask: "+path,
                http.StatusBadRequest)
            return
        }
    }
    s.users[u.ID] = u
    // ...
}
```

> [!IMPORTANT]
>
> The handler only mutates the fields named in the mask. Every other field on the stored
> user stays as it was, even if the body sends a value for it.

So far `address` has been a string. If it were an object with `street` and `city`, a client
might want to change only the city. A mask with `address` covers the whole object. To target
only the city, the query parameter names the subfield with a dot, as
`update_mask=address.city`:

```sh {mark="update_mask=address.city"}
# request
PATCH /users/1?update_mask=address.city HTTP/1.1
Content-Type: application/json

{"address": {"city": "Berlin"}}
```

The handler then needs an `address.city` case in its switch.

The full example with a test is [on GitHub].

<!-- references -->
<!-- prettier-ignore-start -->

[asked on r/golang]:
    https://www.reddit.com/r/golang/comments/1ww034a/how_to_check_for_undefined_attributes_on_json/

[field mask]:
    https://google.aip.dev/134

[FieldMask]:
    https://protobuf.dev/reference/protobuf/google.protobuf/#field-mask

[on GitHub]:
    https://github.com/rednafi/examples/tree/main/patch-field-mask

<!-- prettier-ignore-end -->
