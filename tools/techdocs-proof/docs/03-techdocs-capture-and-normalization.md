# 3. TechDocs Capture and Normalization

The TechDocs side of the proof has two jobs:

1. preserve exactly what was published for the run;
2. convert the rendered presentation into stable contract facts.

Those jobs are kept separate. Raw page evidence remains available even if a later parser rule changes.

## Discovery

### Why use both the reference root and sitemap

The API reference root provides links visible through the rendered navigation. The sitemap provides a site-wide inventory produced by the publishing system. Either source alone can lag or omit a page during a deployment.

The script takes the union:

```python
urls = links_from_reference_root | links_from_sitemap
```

Then it removes:

- fragment identifiers;
- duplicate URLs;
- static asset extensions;
- unresolved `${...}` and encoded-template URLs;
- generic numeric HTTP error reference pages.

The final list is sorted before fetching.

### Discovery is not endpoint parsing

A discovered page is not automatically an API operation. Discovery answers:

```text
Which rendered reference pages should be preserved?
```

Parsing answers:

```text
Which of those pages contains a recognizable operation contract?
```

The proof keeps all discovered pages but only emits endpoint records for pages containing a method-and-route declaration.

## Fetch behavior

The default run uses 16 workers. The script caps the value at 32 to avoid turning a monitor into an accidental load generator.

Each fetch has:

- a 30-second timeout;
- a named user agent;
- up to three attempts;
- short bounded backoff;
- retries only for likely transient failures.

Permanent HTTP failures are not retried indefinitely. After workers finish, the proof compares counts:

```text
discovered = 516
fetched    = 516
```

If `fetched < discovered`, the run stops.

## Rendering HTML into parseable text

The published page is HTML, but the route and parameter sections are easier to parse after a text normalization that turns the same page into the same lines every time.

The normalizer:

1. removes script, style, and noscript blocks;
2. inserts line boundaries at headings, paragraphs, sections, list items, table rows, preformatted blocks, and line breaks;
3. preserves link destinations near link text as `[link: URL]`;
4. removes remaining tags;
5. decodes HTML entities;
6. normalizes line endings and horizontal whitespace;
7. removes blank lines and a small set of known sign-in/vendor footer noise;
8. keeps a schema switcher as a `[variants: ...; rendered: ...]` line;
9. redacts credential-shaped values;
10. writes a terminal newline.

The result is not intended to be beautiful Markdown. It is a stable, readable evidence format that retains rendered words and URLs.

## Page filenames

A page filename includes a sanitized URL path plus a short SHA-256 suffix:

```text
linode-api__reference__get-linode-instance__1a2b3c4d5e.md
```

The suffix prevents collisions when two URLs sanitize to the same path-like name.

Each page begins with:

```yaml
---
source_url: https://techdocs.akamai.com/linode-api/reference/get-linode-instance
---
```

`url-index.json` maps every source URL to its preserved page file and byte count.

## Recognizing an operation

The route parser expects the rendered route declaration to contain:

- one supported method: `GET`, `POST`, `PUT`, `PATCH`, or `DELETE`;
- optional route-level `deprecated` text in the rendered declaration;
- `https://api.linode.com` or `https://monitor-api.linode.com`;
- `{apiVersion}`;
- a route suffix.

Two hosts, not one. The monitor metrics read is served from its own host, and a
regex pinned to `api.linode.com` reads that page as documenting no operation at
all, so the whole route drops out of the comparison rather than one parameter.
The host is a serving detail, not part of the route key: both hosts produce the
same normalized path, and the protobuf side declares a path with no host.

Whitespace around the displayed URL is removed. The raw path is retained, while the comparison path is normalized.

Example rendered route:

```text
GET https://api.linode.com / {apiVersion} /linode/instances/{linodeId}
```

Raw representation:

```text
/{apiVersion}/linode/instances/{linodeId}
```

Normalized comparison representation:

```text
/v4/linode/instances/{param}
```

Parameter placeholder names are erased in the route key because these are semantically the same route shape:

```text
/v4/widgets/{widgetId}
/v4/widgets/{id}
```

Parameter identity is checked separately from the route key.

## API version handling

The rendered `{apiVersion}` path parameter is examined before being removed from ordinary path parameters.

If its allowed values are exactly:

```text
v4beta
```

the operation version is `v4beta`. Otherwise the operation version is `v4`.

The version distinction remains in the operation record even though route normalization uses a stable path form.

## Section ranges

The parser scans forward from the route line for these section markers:

```text
Path Params
Query Params
Body Params
Responses
```

Each section begins after its marker and ends at the next marker. Parsing stops after the response range is established.

This bounded approach prevents a parameter-like phrase elsewhere on the page from being classified as an API parameter.

## Schema switchers, and what the page does not publish

Some operations render their Body Params section behind a client-side switcher.
The NodeBalancer config create page opens its body section with `UDP TCP HTTP
HTTPS`; the destination create page puts one on the `details` parameter with
`Akamai Object Storage` and `Custom HTTPS`.

The fetched HTML carries the fields of one variant. The others are not hidden in
the DOM and no per-variant URL reaches them: the switcher is a `<select>` whose
options are labels, and the fields it swaps in are built in the browser. The one
place the other variants' fields do appear is the OpenAPI document the page
hydrates from, and chapter 1 rules that out as a second external authority.

So the normalizer keeps what the page does publish, which is the labels:

```text
[variants: UDP, TCP, HTTP, HTTPS; rendered: UDP]
```

A marker with a parameter head above it belongs to that parameter. A marker with
no head above it governs the whole section, and the operation record carries the
labels and the one that was rendered.

The comparison reads that as a limit on the documented side rather than as a
complete body. See chapter 5.

## Parameter parsing

A parameter begins with a name and a recognized rendered type. Accepted type families include:

- string and strings;
- integer and integers;
- number and numbers;
- boolean and booleans;
- UUID and UUIDs;
- object and objects;
- array;
- array of a recognized type;
- map of a recognized type;
- password;
- date-time;
- URL;
- nullable forms rendered with `| null` or with `or null`;
- an array's item qualifier rendered as a trailing `, unique`.

The site names a string's format where it has one, so `url` and `uuid` and
`date-time` normalize to `string` for comparison: the protobuf side carries the
scalar and nothing finer.

A parameter whose schema is a switcher renders no type token at all. Its head is
the name alone, or the name followed by `required`, and the switcher marker on
the next line is what separates that head from ordinary prose. The type recorded
for it is `object`, which is the only schema shape the site renders a switcher
for.

Within the parameter block, the parser recognizes:

```text
Required
Deprecated
Defaults to <value>
Allowed: <value>
```

Additional allowed values may appear on following lines. Values are deduplicated and sorted.

Defaults are parsed as JSON when possible. This preserves the difference between:

```json
false
0
null
"false"
```

If JSON parsing fails, the rendered scalar remains a string.

## Location is part of identity

The same name can appear in different locations:

```text
path.id
query.id
body.id
```

The normalized contract therefore stores location explicitly. Defaults and enums use a location-qualified key during endpoint parsing:

```text
query.page
body.type
```

The comparator only ignores location when a normalized name is unique enough on the operation to prove the intended match. If multiple candidates remain, it emits `parameter_location_ambiguous`.

## Endpoint record

A parsed endpoint record contains evidence useful before flattening:

```json
{
  "api_version": "v4",
  "method": "GET",
  "path": "/v4/linode/instances/{param}",
  "raw_path": "/{apiVersion}/linode/instances/{linodeId}",
  "operation_id": "get-linode-instance",
  "deprecated": false,
  "docs_url": "https://techdocs.akamai.com/linode-api/reference/get-linode-instance",
  "source_page": "pages/...md",
  "status_codes": [200],
  "parameters": {
    "path": [],
    "query": [],
    "body": []
  },
  "parameter_defaults": {},
  "parameter_enums": {},
  "deprecated_parameters": []
}
```

The actual page remains the evidence source. The endpoint record is the parser's structured interpretation.

## Normalized contract shape

`techdocs-contracts.json` separates operation and parameter facts into sorted arrays:

```json
{
  "source_authority": "techdocs-rendered-pages",
  "routes": [],
  "parameters": [],
  "defaults": [],
  "enums": [],
  "deprecated_routes": [],
  "deprecated_parameters": []
}
```

A route record:

```json
{
  "api_version": "v4",
  "method": "GET",
  "path": "/v4/widgets/{param}",
  "deprecated": false,
  "source_url": "https://techdocs.akamai.com/linode-api/reference/get-widget"
}
```

A parameter record:

```json
{
  "api_version": "v4",
  "method": "GET",
  "path": "/v4/widgets/{param}",
  "location": "query",
  "name": "page",
  "type": "integer",
  "required": false,
  "default": 1,
  "enum": [],
  "deprecated": false
}
```

All arrays are sorted by stable JSON serialization. Network completion order and filesystem enumeration order do not affect the contract.

## Fail-closed cases on the TechDocs side

The run must stop when:

- no pages are discovered;
- any discovered page cannot be fetched after retries;
- a page index points to a missing page;
- two pages parse to the same normalized operation key;
- no endpoint contracts are produced;
- the contract authority is not exactly `techdocs-rendered-pages`;
- deprecated replacement evidence cannot be loaded when needed.

A partial or ambiguous snapshot is not “close enough.” It changes the meaning of absence findings and therefore cannot be compared safely.
