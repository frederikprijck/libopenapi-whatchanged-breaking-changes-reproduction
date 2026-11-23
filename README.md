# libopenapi `what-changed`: required property added to a response-only schema is reported as breaking

Adding a property to a schema that only **responses** use, and listing it in `required`, cannot break a client: the server now always sends one more field. [`pb33f/libopenapi`](https://github.com/pb33f/libopenapi) v0.44.0 recognises this in the simplest case, and still reports it as **breaking** in cases 03 to 06 below. Cases 07 and 08 are controls for a related question about closed schemas. This repository reproduces each case.

This was found through [`openapi-changes`](https://github.com/pb33f/openapi-changes), which uses libopenapi's `what-changed` package to classify changes. The reproduction uses libopenapi directly.

## Background

[pb33f/libopenapi#719](https://github.com/pb33f/libopenapi/issues/719) reported that adding a required property was breaking for every schema, whichever direction the data flows. It was fixed by [#725](https://github.com/pb33f/libopenapi/pull/725) and released in v0.44.0. The fix works out which schemas are used only by responses (or only by requests) and relaxes the rule for those. It is conservative: whenever it cannot prove the use, it keeps the old rule. That is where the cases below fall through.

## Reproduce

Requires Go 1.26 or newer.

```sh
go run .
```

The program ([`main.go`](main.go)) compares `base.yaml` with `head.yaml` in every directory under [`cases/`](cases), using `libopenapi.CompareDocuments` from libopenapi v0.44.0. In cases 01 to 06 the only change is that `Widget` gains an `extra` property and `extra` is added to `required`. Cases 07 and 08 change `Widget` differently, as described under [Cases](#cases). `Widget` is only used by a response in every case.

Output:

```
case                               changes  breaking
01-direct-open                     2        0
02-array-open                      2        0
03-oneof                           2        1   <- expected 0
04-closed                          2        1   <- expected 0
05-discriminator-elsewhere         2        1   <- expected 0
06-oneof-closed-discriminator      2        1   <- expected 0
07-closed-optional                 1        0
08-closed-existing-property        1        0

4 of 8 cases are reported as breaking although only a response schema changed.
```

`changes` is the total number of changes in the case and `breaking` is how many of them are reported as breaking. The program exits with status 1 when a case is reported as breaking.

In cases 01 to 06 the two changes are the new property and the new `required` entry. Only the `required` entry is reported as breaking. Cases 07 and 08 have a single change each.

## Cases

| Case | The response schema is | Breaking | Why (libopenapi v0.44.0) |
|---|---|---|---|
| [`01-direct-open`](cases/01-direct-open) | `$ref` to `Widget` | 0 | Control. The fix works. |
| [`02-array-open`](cases/02-array-open) | array of `$ref` to `Widget` | 0 | Control. The fix works. |
| [`03-oneof`](cases/03-oneof) | `oneOf` of an array and an object wrapper, each referencing `Widget` | 1 | `oneOf` branches are counted as used by both requests and responses ([schema_usage.go#L266-L267](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L266-L267)), so `Widget` is never "response only". `allOf` and `anyOf` keep the real direction ([#L254-L255](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L254-L255)). |
| [`04-closed`](cases/04-closed) | `$ref` to `Widget` with `additionalProperties: false` | 1 | For a closed response schema, a required name stays breaking unless it was already declared in the old `properties` ([`requiredAdditionRule`, #L53-L74](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L53-L74)). |
| [`05-discriminator-elsewhere`](cases/05-discriminator-elsewhere) | `$ref` to `Widget`, plus an **unrelated** `/pets` route whose schema has a `discriminator` | 1 | Reaching a `discriminator` (or `$dynamicRef`, `$recursiveRef`, `$id`) makes the usage walk give up ([#L272-L275](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L272-L275)), and then no usage information exists for the whole document ([#L111-L139](https://github.com/pb33f/libopenapi/blob/v0.44.0/what-changed/model/schema_usage.go#L111-L139)). |
| [`06-oneof-closed-discriminator`](cases/06-oneof-closed-discriminator) | `oneOf`, closed, plus the `/pets` route | 1 | All three of the above. |
| [`07-closed-optional`](cases/07-closed-optional) | `$ref` to a closed `Widget`, which gains an **optional** `extra` property | 0 | Same as case 04, except `extra` is not listed in `required`. Not reported. |
| [`08-closed-existing-property`](cases/08-closed-existing-property) | `$ref` to a closed `Widget` that already declares `extra`, which becomes required | 0 | The name was already declared, so the closed-schema rule does not apply. Not reported. |

Case 05 is identical to case 01 except for the extra `/pets` route in both `base.yaml` and `head.yaml`. The `Widget` change and its usage are untouched, but the verdict changes.

```sh
diff cases/01-direct-open/head.yaml cases/05-discriminator-elsewhere/head.yaml
```

Cases 04 and 07 have the same `base.yaml`, and their `head.yaml` files differ in one line: whether `extra` is listed in `required`. A client that validates strictly against the old closed schema rejects both responses, because it does not know the `extra` property, yet only case 04 is reported.

```sh
diff cases/04-closed/head.yaml cases/07-closed-optional/head.yaml
```

## What I would expect

- **`oneOf`:** follow the branches with the direction of the enclosing schema, like `allOf` and `anyOf`. The `oneOf` rationale in the source is not documented, so there may be a reason to treat it as shared that I am missing.
- **Discriminator:** limit the fallback to the schemas the walk could not follow, instead of the whole document. One unrelated discriminator currently disables the response-only classification for every schema in the spec.
- **Closed schemas:** this one is deliberate and arguable (a client that validates strictly against the old closed schema would reject the new property). It is however inconsistent with how the new property itself is classified: in case 04 the added property is **not** reported as breaking (2 changes, 1 breaking), only the matching `required` entry is, and in case 07 the same property without `required` is not reported at all (1 change, 0 breaking).

## Other tools and versions

- The `openapi-changes` CLI bundles its own libopenapi. I ran `openapi-changes summary cases/<case>/base.yaml cases/<case>/head.yaml` for every case with two builds:

  | Case | `openapi-changes` v0.4.4 release | v0.4.4 built against libopenapi v0.44.0 |
  |---|---|---|
  | `01-direct-open` | 1 breaking | 0 breaking |
  | `02-array-open` | 1 breaking | 0 breaking |
  | `03-oneof` to `06-oneof-closed-discriminator` | 1 breaking | 1 breaking |
  | `07-closed-optional` | 0 breaking | 0 breaking |
  | `08-closed-existing-property` | 1 breaking | 0 breaking |

  The v0.4.4 release (2026-11-19) predates the fix, and its `go.mod` pins libopenapi `v0.42.6-0.20261114224705-db27aae96380`. The second column matches the output of `go run .`.
- [#719](https://github.com/pb33f/libopenapi/issues/719) lists `oasdiff` 1.39.0 as reporting 0 for the plain response-only case. I have not run oasdiff on the cases here.
