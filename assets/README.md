# Marketplace Asset Inventory and Exception

Status: no custom marketplace asset is included in the initial package.

| Asset | Present | Reason / provenance |
|---|---:|---|
| Railway deploy button | Remote badge only | Railway-owned `https://railway.com/button.svg`; verify final slug before publish |
| Custom icon | No | Explicit exception requested; avoids copied CLIProxyAPI, provider, or Railway marks |
| Screenshot | No | Management UI can expose provider/account context; not needed for first publication |
| Logo | No | Upstream/provider marks intentionally excluded |
| Video/GIF | No | Not required |

## Exception

The initial listing uses text-only product identity and, if Railway requires
one, a neutral platform default. Independent QA and the Publish Approval Packet
must accept this exception. Publication must stop if Railway requires a custom
asset and no original, reviewed, provenance-recorded file exists.

Any future icon must be original and MIT-licensed by its creator. Any future
screenshot must come from a disposable instance with synthetic provider-free
state and must remove keys, OAuth/device codes, cookies, accounts, models,
prompts/responses, Railway IDs/domains, browser storage, and timestamps that
identify a user. Record creator/source, date, permission, modifications, and
intended channel here before use.
