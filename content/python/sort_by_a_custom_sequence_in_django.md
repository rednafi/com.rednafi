---
title: Sorting a Django queryset by a custom sequence of an attribute
date: 2023-05-09
tags:
    - Python
    - Django
---

I needed a way to sort a Django queryset based on a custom sequence of an attribute.
Typically, Django allows sorting a queryset by any attribute on the model or related to
it in either ascending or descending order. However, what if you need to sort the
queryset following a custom sequence of attribute values?

Suppose, you're working with a model called `Product` where you want to sort the rows of
the table based on a list of product ids that are already sorted in a particular order.
Here's how it might look:

```python
# List of product ids
id_list = [3, 1, 2, 4, 8, 7, 5, 6]

# We want to sort the products queryset in such a way that the records
# appear in the same order specified in the id_list.
products = Product.objects.all()
```

Turns out, this is a great case where Django's `Case` and `When` can come in handy.
Django exposes the underlying SQL's way of performing conditional logic via `CASE` and
`WHEN` statements. These allow you to return different values or expressions based on
some criteria. Think of them as similar to `IF-THEN-ELSE` statements in other
programming languages. There are two types of CASE expressions: simple and searched.

## Simple CASE Expression

A simple `CASE` expression compares an input expression to a list of values and returns
the corresponding result. Here is the syntax:

```txt
CASE input_expression
    WHEN value1 THEN result1
    WHEN value2 THEN result2
    ...
    ELSE default_result
END
```

The `input_expression` can be any valid SQL expression. The data types of the
`input_expression` and each value must be the same or must be an implicit conversion.

The `WHEN` clauses are evaluated in order, from top to bottom. The first one that
matches the `input_expression` determines the result of the `CASE` expression. If none
of the values match, the `ELSE` clause is executed. If the `ELSE` clause is omitted and
no values match, the `CASE` expression returns `NULL`. For example, suppose we have a
table called `products` with the following data:

```txt
| id | name | price | category |
| -- | ---- | ----- | -------- |
| 1  | A    | 10    | X        |
| 2  | B    | 20    | Y        |
| 3  | C    | 30    | Z        |
| 4  | D    | 40    | X        |
| 5  | E    | 50    | Y        |
```

We can use a simple CASE expression to assign a label to each product based on its
category:

```sql
SELECT id, name, price,
CASE category
    WHEN 'X' THEN 'Low'
    WHEN 'Y' THEN 'Medium'
    WHEN 'Z' THEN 'High'
END AS label
FROM products;
```

The output would be:

```txt
| id | name | price | label  |
| -- | ---- | ----- | ------ |
| 1  | A    | 10    | Low    |
| 2  | B    | 20    | Medium |
| 3  | C    | 30    | High   |
| 4  | D    | 40    | Low    |
| 5  | E    | 50    | Medium |
```

## Searched CASE Expression

A searched `CASE` expression evaluates a list of Boolean expressions and returns the
corresponding result. Here is the syntax for a searched CASE expression:

```txt
CASE
    WHEN condition1 THEN result1
    WHEN condition2 THEN result2
    ...
    ELSE default_result
END
```

The conditions can be any valid Boolean expressions. Just like simple `CASE`
expressions, here also, the data types of each result must be the same or must be an
implicit conversion.

As before, the `WHEN` clauses are evaluated in order, from top to bottom. The first one
that evaluates to `TRUE` determines the result of the `CASE` expression. If none of the
conditions are `TRUE`, the `ELSE` clause is executed. If the `ELSE` clause is omitted
and no conditions are `TRUE`, the `CASE` expression returns `NULL`.

For example, we can use a searched `CASE` expression to calculate a discount for each
product based on its price:

```sql
SELECT id, name, price,
CASE
    WHEN price < 20 THEN price * 0.9 --10% discount
    WHEN price BETWEEN 20 AND 40 THEN price *0.8 --20% discount
ELSE price *0.7 --30% discount
END AS discounted_price
FROM products;
```

The output would be:

```txt
| id | name | price | discounted_price |
| -- | ---- | ----- | ---------------- |
| 1  | A    | 10    | 9                |
| 2  | B    | 20    | 16               |
| 3  | C    | 30    | 24               |
| 4  | D    | 40    | 32               |
| 5  | E    | 50    | 35               |
```

## Using searched CASE expression to order a queryset

With the intro explanations out of the way, here's how you can sort the `products` table
introduced in the previous section by a list of product ids:

```python
from django.db.models import Case, When
from .models import Product

product_ids = [4, 2, 1, 3, 5]
products = Product.objects.all()

preferred = Case(
    *(When(id=id, then=pos) for pos, id in enumerate(product_ids, start=1))
)
products_sorted = products.filter(id__in=product_ids).order_by(preferred)
```

Printing the queryset will return the following output:

```txt
<QuerySet [
    <Product: Product object (4)>,
    <Product: Product object (2)>,
    <Product: Product object (1)>,
    <Product: Product object (3)>,
    <Product: Product object (5)>
]>
```

Here, we're trying to sort the `products` queryset by the product ids in the same order
specified in the `product_ids` list. First, we build a `Case` expression where we're
iterating through the product ids and defining the designated positions of the ids based
on their positions in the list. Then we filter the `products` queryset by the ids and
pass the `preferred` expression to the `.order_by` method. To see the underlying SQL
generated by Django, you can print the result of `products_sorted.query`:

```sql
SELECT
    blog_product.id,
    blog_product.name,
    blog_product.price,
    blog_product.category
FROM blog_product
WHERE blog_product.id IN (4, 2, 1, 3, 5)
ORDER BY
    CASE
        WHEN blog_product.id = 4 THEN 1
        WHEN blog_product.id = 2 THEN 2
        WHEN blog_product.id = 1 THEN 3
        WHEN blog_product.id = 3 THEN 4
        WHEN blog_product.id = 5 THEN 5
        ELSE NULL
    END ASC;
```

You can directly run this query against the database and get the same result. Notice how
Django is taking advantage of searched `CASE` expression to sort the queryset in the
desired way. This also allows sorting by a custom sequence of an attribute related to
the target model. So you can do this:

```python
from django.db.models import Case, When
from .models import Product, Order

# Notice how we want to sort the products by the ids of the orders
order_ids = [4, 2, 1, 3, 5]
products = Product.objects.all()

preferred = Case(
    *(
        When(order__id=id, then=pos)
        for pos, id in enumerate(product_ids, start=1)
    )
)
products_sorted = products.filter(order__id__in=order_ids).order_by(preferred)
```

Here's what the `Product` and `Order` models look like:

```python
from django.db import models


class Product(models.Model):
    name = models.CharField(max_length=200)
    price = models.FloatField()
    category = models.CharField(max_length=200)


class Order(models.Model):
    product = models.ForeignKey(Product, on_delete=models.CASCADE)
    quantity = models.IntegerField()
    date = models.DateField()
```

`Order` has a foreign key relationship with `Product` and we're sorting the product
queryset based on a custom sequence of order ids. The query generates the SQL below:

```sql
SELECT
    blog_product.id,
    blog_product.name,
    blog_product.price,
    blog_product.category
FROM
    blog_product
    LEFT OUTER JOIN blog_order ON (blog_product.id = blog_order.product_id)
WHERE
    blog_order.id IN (4, 2, 1, 3, 5)
ORDER BY
    CASE
        WHEN blog_order.id = 4 THEN 1
        WHEN blog_order.id = 2 THEN 2
        WHEN blog_order.id = 1 THEN 3
        WHEN blog_order.id = 3 THEN 4
        WHEN blog_order.id = 5 THEN 5
        ELSE NULL
    END ASC
```

Running the query gives us the following output:

```txt
| id |   name    | price |  category  |
|----|-----------|-------|------------|
| 9  | Product 9 | 46.0  | Category 9 |
| 6  | Product 6 | 83.0  | Category 6 |
| 4  | Product 4 | 59.0  | Category 4 |
| 11 | Product 1 | 51.0  | Category 1 |
```

## References

* [Django get a queryset from an array of id's in a specific order - Stack Overflow]

[Django get a querySet from an array of id's in a specific order - Stack Overflow]: https://stackoverflow.com/questions/4916851/django-get-a-queryset-from-array-of-ids-in-specific-order
