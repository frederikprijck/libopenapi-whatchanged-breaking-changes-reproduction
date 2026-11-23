# libopenapi `what-changed`: required property added to a response-only schema is reported as breaking

Adding a property to a schema that only **responses** use, and listing it in `required`, cannot break a client: the server now always sends one more field. [`pb33f/libopenapi`](https://github.com/pb33f/libopenapi) v0.44.0 recognises this in the simplest case, and still reports it as **breaking** in the cases below. This repository reproduces each of them.

This was found through [`openapi-changes`](https://github.com/pb33f/openapi-changes), which uses libopenapi's `what-changed` package to classify changes. The reproduction uses libopenapi directly.

## Background

[pb33f/libopenapi#719](https://github.com/pb33f/libopenapi/issues/719) reported that adding a required property was breaking for every schema, whichever direction the data flows. It was fixed by [#725](https://github.com/pb33f/libopenapi/pull/725) and released in v0.44.0. The fix works out which schemas are used only by responses (or only by requests) and relaxes the rule for those. It is conservative: whenever it cannot prove the use, it keeps the old rule. That is where the cases below fall through.

## Reproduce

Requires Go 1.26 or newer.

```sh
go run .
```

The program ([`main.go`](main.go)) compares `base.yaml` with `head.yaml` in every directory under [`cases/`](cases), using `libopenapi.CompareDocuments` from libopenapi v0.44.0. In every case the only change is that `Widget` gains an `extra` property and `extra` is added to `required`. `Widget` is only used by a response.

Output:

```
case                               changes  breaking  expected
01-direct-open                     2        0         0  ok
02-array-open                      2        0         0  ok
03-oneof                           2        1         0  UNEXPECTED
04-closed                          2        1         0  UNEXPECTED
05-discriminator-elsewhere         2        1         0  UNEXPECTED
06-oneof-closed-discriminator      2        1         0  UNEXPECTED

4 of 6 cases are reported as breaking although only a response gained a required property.
```

The two changes in each case are the new property and the new `required` entry. Only the `required` entry is reported as breaking.

## Cases

| Case | The response schema is | Breaking | Why (libopenapi v0.44.0) |
|---|---|---|---|
| [`01-direct-open`](cases/01-direct-open) | `$ref` to `Widget` | 0 | Control. The fix works. |
| [`02-array-open`](cases/02-array-open) | array of `$ref` to `Widget` | 0 | Control. The fix works. |
| [`03-oneof`](cases/03-oneof) | `oneOf` of an array and an object wrapper, each referencing `Widget` | 1 | `oneOf` branches are counted as used by both requests and responses ([schema_usage.go#L266-L267](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L266-L267)), so `Widget` is never "response only". `allOf` and `anyOf` keep the real direction ([#L254-L255](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L254-L255)). |
| [`04-closed`](cases/04-closed) | `$ref` to `Widget` with `additionalProperties: false` | 1 | For a closed response schema, a required name stays breaking unless it was already declared in the old `properties` ([`requiredAdditionRule`, #L53-L74](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L53-L74)). |
| [`05-discriminator-elsewhere`](cases/05-discriminator-elsewhere) | `$ref` to `Widget`, plus an **unrelated** `/pets` route whose schema has a `discriminator` | 1 | Reaching a `discriminator` (or `$dynamicRef`, `$recursiveRef`, `$id`) makes the usage walk give up ([#L272-L275](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L272-L275)), and then no usage information exists for the whole document ([#L111-L139](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L111-L139)). |
| [`06-oneof-closed-discriminator`](cases/06-oneof-closed-discriminator) | `oneOf`, closed, plus the `/pets` route | 1 | All three of the above. |

Case 05 is identical to case 01 except for the extra `/pets` route in both `base.yaml` and `head.yaml`. The `Widget` change and its usage are untouched, but the verdict changes.

```sh
diff cases/01-direct-open/head.yaml cases/05-discriminator-elsewhere/head.yaml
```

## What I would expect

- **`oneOf`:** follow the branches with the direction of the enclosing schema, like `allOf` and `anyOf`. The `oneOf` rationale in the source is not documented, so there may be a reason to treat it as shared that I am missing.
- **Discriminator:** limit the fallback to the schemas the walk could not follow, instead of the whole document. One unrelated discriminator currently disables the response-only classification for every schema in the spec.
- **Closed schemas:** this one is deliberate and arguable (a client that validates strictly against the old closed schema would reject the new property). It is however inconsistent with how the new property itself is classified: in case 04 the added property is **not** reported as breaking (2 changes, 1 breaking), only the matching `required` entry is.

## Other tools and versions

- The `openapi-changes` CLI bundles its own libopenapi. I ran `openapi-changes summary cases/<case>/base.yaml cases/<case>/head.yaml` for every case with two builds:

  | Case | `openapi-changes` v0.4.4 release | v0.4.4 built against libopenapi v0.44.0 |
  |---|---|---|
  | `01-direct-open` | 1 breaking | 0 breaking |
  | `02-array-open` | 1 breaking | 0 breaking |
  | `03-oneof` to `06-oneof-closed-discriminator` | 1 breaking | 1 breaking |

  The v0.4.4 release (2026-11-19) predates the fix, and its `go.mod` pins libopenapi `v0.42.6-0.20261114224705-db27aae96380`. The second column matches the output of `go run .`.
- [#719](https://github.com/pb33f/libopenapi/issues/719) lists `oasdiff` 1.39.0 as reporting 0 for the plain response-only case. I have not run oasdiff on the cases here.
