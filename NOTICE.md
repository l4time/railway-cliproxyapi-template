# Notices

Copyright 2026 Jordan Tanp and contributors.

The Railway packaging, wrapper, release controller, tests, and documentation in
this repository are licensed under the MIT License.

This package builds from
[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), copyright its
upstream contributors and distributed under the MIT License. The initial
runtime uses `v7.2.141` at immutable Docker manifest digest
`sha256:7f598ce64478a8a5f90ed76875e0e9b0e7d77b80e17184b13df18c3d5bdb3def`.
The embedded fallback image and bundled dependencies retain their upstream
notices and terms. At runtime, the package may download a newer official stable
CLIProxyAPI Linux archive and `checksums.txt` from the upstream GitHub Release;
the verified executable remains governed by CLIProxyAPI's MIT license and
upstream notices.

The image also bundles
[CLIProxyAPI Management Center](https://github.com/router-for-me/Cli-Proxy-API-Management-Center)
`v1.22.6` under its MIT license. Its `management.html` SHA-256 is
`e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4`.

Review both upstream projects' copyright, dependency, security, and license
notices whenever a pin changes. Provider software, services, subscriptions,
models, names, and credentials are not distributed by this package and remain
subject to their owners' terms.

No upstream/provider logo, screenshot, video, credential, or account data is
included. The initial marketplace package requests a no-custom-asset exception
documented in `assets/README.md`.
