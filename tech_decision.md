## Query builder choice

[Jet](https://github.com/go-jet/jet) [Squirrel](https://github.com/Masterminds/squirrel) and [Goqu](https://github.com/doug-martin/goqu/) does  not support DDL

I found text/template lib to be difficult to handle composition of query parts

https://github.com/huandu/go-sqlbuilder seems a strong candidate.
But after trying it, it seems that it can not handle column definition (except by passing a slice in the right order), and so does not reduce that much the complexity
