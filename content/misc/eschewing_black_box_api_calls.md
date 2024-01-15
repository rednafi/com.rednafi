---
title: Eschewing black box API calls
date: 2024-01-15
tags:
    - Python
    - JavaScript
    - Go
---

I love dynamically typed languages as much as the next person. They let us make ergonomic
API calls like this:

```python
import httpx

# Sync call for simplicity
r = httpx.get("https://dummyjson.com/products/1").json()
print(r["id"], r["title"], r["description"])
```

or this:

```js
fetch("https://dummyjson.com/products/1")
  .then((res) => res.json())
  .then((json) => console.log(json.id, json.type, json.description));
```

In both cases, running the snippets will return:

```txt
1 'iPhone 9' 'An apple mobile which is nothing like apple'
```

Unless you've worked with a statically typed language that enforces more constraints, it's
hard to appreciate how incredibly convenient it is to be able to call and use an API
endpoint without having to deal with types or knowing anything about its payload structure.
You can treat the API response as a black box and deal with everything in runtime.

For example, Go wouldn't even allow you to do so in such a loosey-goosey way. To consume the
API, you'd need to create a struct in the essence of the return payload and then unmarshal
the payload into it.

Here's the complete response payload that `curl -s https://dummyjson.com/products/1 | jq`
returns:

```json
{
  "id": 1,
  "title": "iPhone 9",
  "description": "An apple mobile which is nothing like apple",
  "price": 549,
  "discountPercentage": 12.96,
  "rating": 4.69,
  "stock": 94,
  "brand": "Apple",
  "category": "smartphones",
  "thumbnail": "https://cdn.dummyjson.com/product-images/1/thumbnail.jpg",
  "images": [
    "https://cdn.dummyjson.com/product-images/1/1.jpg",
    "https://cdn.dummyjson.com/product-images/1/2.jpg",
    "https://cdn.dummyjson.com/product-images/1/3.jpg",
    "https://cdn.dummyjson.com/product-images/1/4.jpg",
    "https://cdn.dummyjson.com/product-images/1/thumbnail.jpg"
  ]
}
```

This is how you'd call the same API endpoint in Go. I'm using this json-to-go[^1] service to
generate the Go struct instead of handwriting it:

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

// Define the struct that reflects the response payload
type Product struct {
    ID                 int      `json:"id"`
    Title              string   `json:"title"`
    Description        string   `json:"description"`
    Price              int      `json:"price"`
    DiscountPercentage float64  `json:"discountPercentage"`
    Rating             float64  `json:"rating"`
    Stock              int      `json:"stock"`
    Brand              string   `json:"brand"`
    Category           string   `json:"category"`
    Thumbnail          string   `json:"thumbnail"`
    Images             []string `json:"images"`
}

func main() {
    // Ignore error handling for brevity
    var product Product

    response, _ := http.Get("https://dummyjson.com/products/1")
    defer response.Body.Close()

    body, _ := io.ReadAll(response.Body)

    _ = json.Unmarshal(body, &product) // project the response into the struct

    // Do processing with product instance
    fmt.Println(product.ID, product.Title, product.Description)
}
```

This will give us the same output as the Python and JS code snippets:

```txt
1 iPhone 9 An apple mobile which is nothing like apple
```

Above, we had to create a new struct type to represent the response payload, instantiate it,
and unmarshal the JSON payload into the struct before we were able to process it.

Notice that we're only using 3 fields and ignoring the rest. In this case, you can get away
with only including those 3 fields in the struct type, and Go will do the right thing:

```go
// ... same as before

type Product struct {
    ID          int     `json:"id"`
    Title       string  `json:"title"`
    Description string  `json:"description"`
}

// ... same as before
```

While this is less work than having to mimic the whole structure of the JSON output in the
struct definition, it's still not winning any medals for brevity against the Python and JS
snippets.

Dynamically processing a JSON payload is nice as long as you're working on a throwaway
script. Anything more, it becomes a headache since the reader won't have any idea about what
the API response looks like without looking at the documentation or traces.

Also, type safety is an issue. Since the imperative examples don't assume the structure of
the response, you'll be surprised with a runtime error if you've made an incorrect
assumption about the response structure. Sure, having to write a struct is a chore, but the
free documentation and the type safety are things that you don't get with the black box API
calls.

Statically typed languages force you to maintain good hygiene while working with JSON
payloads. Declaratively embedding the payload structure directly into the codebase is
immensely beneficial. But how do you do that in a language like Python?

If you want to go with what's in the standard library, you can handroll a dataclass like
this and project the return payload onto it:

```python
# ...

from dataclasses import dataclass
from typing import Self


@dataclass(slots=True)
class Product:
    id: int
    title: str
    description: str
    price: int
    discount_percentage: float
    rating: float
    stock: int
    brand: str
    category: str
    thumbnail: str
    images: list[str]

    @classmethod
    def from_dict(cls, d: dict) -> Self:
        # This is needed to reconcile the snake_case and camelCase variables
        discount_percentage = d.pop("discountPercentage", None)
        return cls(discount_percentage=discount_percentage, **d)


# ...
```

Then just call `Product.from_dict` and pass the output of `response.json()` as before.

This way, the API response is documented in the code, and the reader won't have to depend on
out-of-band information while reading the code. However, you can see that hand rolling data
classes can quickly become hairy when you have a large JSON payload and need to reconcile
the discrepancies between snake case and camel case variables. We had to add a custom
`from_dict` class method to convert the camel case variables to their snake case
counterparts in Python.

Also, unlike Go, you can't define a structure to represent only a portion of the whole
payload in Python without adding extra code to ignore the rest of the fields that aren't
relevant to you.

Pydantic[^2] shines here. It not only allows you to define a class to represent a partial
payload structure, but also applies runtime validation to guarantee operational type safety.
As a bonus, you can use a tool like this[^3] to generate pydantic classes from JSON:

```python
from pydantic import BaseModel, Field


class Product(BaseModel):
    id: int
    title: str
    description: str
    price: int
    discount_percentage: float = Field(alias="discountPercentage")
    rating: float
    stock: int
    brand: str
    category: str
    thumbnail: str
    images: list[str]
```

You can project your response onto the data class with `Product(**response.json())` and get
a rich object that also validates the incoming values. This will work the same way with
partially defined classes:

```python
# ...

from pydantic import BaseModel


class Product(BaseModel):
    id: int
    title: str
    description: str


# ...
```

Here's a complete example:

```python
import httpx
from pydantic import BaseModel
import asyncio


# Partially defined Pydantic model to represent the response
class Product(BaseModel):
    id: int
    title: str
    description: str


async def main() -> None:
    async with httpx.AsyncClient() as client:
        response = await client.get("https://dummyjson.com/products/1")
        response.raise_for_status()
        data = response.json()
        product = Product(**data)
        print(product.id, product.title, product.description)


if __name__ == "__main__":
    asyncio.run(main())
```

In the JS land, you can adopt TypeScript and zod[^4] to achieve a similar result:

```ts
// index.ts
import { z } from "zod";

const ProductSchema = z.object({
  id: z.number(),
  title: z.string(),
  description: z.string(),
  // Other fields can be added if needed
});

type Product = z.infer<typeof ProductSchema>;

fetch("https://dummyjson.com/products/1")
  .then((response) => response.json())
  .then((data) => ProductSchema.parse(data))
  .then((product: Product) => {
    console.log(product.id, product.title, product.description);
  });
```

I find the added verbosity for type safety and readability well worth it.

Fin!

[^1]: [json-to-go](https://mholt.github.io/json-to-go/)
[^2]: [pydantic](https://docs.pydantic.dev/latest/)
[^3]: [json-to-pydantic](https://jsontopydantic.com/)
[^4]: [zod](https://github.com/colinhacks/zod)
