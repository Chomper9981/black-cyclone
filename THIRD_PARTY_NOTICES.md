# Third-party notices

> Black Cyclone tự nó theo giấy phép MIT (xem `LICENSE`). Các bản phát hành có kèm
> hoặc build cùng phần mềm của bên thứ ba dưới đây, mỗi phần giữ giấy phép riêng của nó.

Black Cyclone is licensed under the MIT License (see `LICENSE`). Release builds
include or are built with the third-party software listed below, each under its
own license. The license texts are in the `licenses/` folder next to this file.

Black Cyclone is an independent project. It is not affiliated with, endorsed by
or sponsored by the sing-box project, SagerNet, Cloudflare or MaxMind.

## sing-box (Windows and Linux builds)

The Windows installer and the Linux install ship `sing-box` as a separate
program (`bin/sing-box.exe` on Windows, `bin/sing-box` on Linux). Black Cyclone
starts it as its own process and talks to it through a JSON configuration file
and a local HTTP API; it does not link against or contain sing-box code.

- Version: 1.14.3, the official, unmodified release binary
- Project: https://github.com/SagerNet/sing-box
- Corresponding source code: https://github.com/SagerNet/sing-box/tree/v1.14.3
  (also downloadable as https://github.com/SagerNet/sing-box/archive/refs/tags/v1.14.3.tar.gz)
- License: GNU General Public License v3.0 or later, full text in `licenses/GPL-3.0.txt`

The sing-box license notice, reproduced as published:

```
Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

In addition, no derivative work may use the name or imply association
with this application without prior consent.
```

If the links above ever stop working, open an issue on the Black Cyclone
repository and a copy of the corresponding source will be provided.

The self-hosted server set up by `server/bcserver.sh` downloads the same
official sing-box release directly from GitHub onto the VPS; nothing is
redistributed by this project in that case.

## geoip-vn.srs (Windows and Linux builds)

`bin/geoip-vn.srs`, the list of Vietnamese IP ranges used by the
"Việt Nam đi thẳng" routing option.

- Source: https://github.com/SagerNet/sing-geoip (branch `rule-set`), unmodified
- License of sing-geoip: GNU General Public License v3.0 or later
  (Copyright (C) 2022 by nekohasekai), full text in `licenses/GPL-3.0.txt`
- Data: built by sing-geoip from https://github.com/Dreamacro/maxmind-geoip.

  This product includes GeoLite2 data created by MaxMind, available from
  https://www.maxmind.com.

## Go libraries compiled into the app

The desktop app (Windows and Linux) is built with the Go libraries below. Their
complete license texts, generated from the exact module versions used, are in
`licenses/go-desktop.txt`.

| Module | License |
|---|---|
| Go standard library | BSD-3-Clause |
| github.com/wailsapp/wails/v2 | MIT |
| github.com/wailsapp/go-webview2 | MIT |
| github.com/energye/systray | Apache-2.0 |
| github.com/tkrajina/go-reflector | Apache-2.0 |
| github.com/godbus/dbus/v5 | BSD-2-Clause |
| github.com/google/uuid | BSD-3-Clause |
| github.com/pkg/browser, github.com/pkg/errors | BSD-2-Clause |
| golang.org/x/crypto, net, sys, text | BSD-3-Clause |
| git.sr.ht/~jackmordaunt/go-toast/v2 | MIT or Unlicense |
| github.com/bep/debounce | MIT |
| github.com/gabriel-vasile/mimetype | MIT |
| github.com/go-ole/go-ole | MIT |
| github.com/leaanthony/go-ansi-parser, slicer, u | MIT |
| github.com/rivo/uniseg | MIT |
| github.com/samber/lo | MIT |

The Windows app uses the Microsoft Edge WebView2 Runtime, which comes with
Windows 11. The installer carries Microsoft's WebView2 Evergreen Bootstrapper
(`MicrosoftEdgeWebview2Setup.exe`, redistributable under Microsoft's WebView2
distribution terms) and runs it only when the runtime is missing; the runtime
itself is downloaded from Microsoft. On Linux the app uses the system's GTK and
WebKitGTK libraries.

## Android app

The Android app does not contain sing-box. Its core is Go, built with gomobile;
complete license texts are in `licenses/go-android.txt`.

| Component | License |
|---|---|
| golang.zx2c4.com/wireguard (wireguard-go) | MIT |
| Go standard library, golang.org/x/crypto, mobile, net, sys | BSD-3-Clause |
| AndroidX (core, activity, lifecycle) and Jetpack Compose | Apache-2.0 |
| Kotlin standard library | Apache-2.0 |

The Apache License 2.0 text is in `licenses/Apache-2.0.txt`. The APK carries
these files (with `LICENSE` and this notice) under `assets/licenses/`; the app
shows them in Cài đặt → Giới thiệu → Giấy phép mã nguồn mở.

## Updating this file

After changing Go dependencies, regenerate the license texts:

```sh
go run ./tools/licenses -goos windows,linux -tags desktop,production -o licenses/go-desktop.txt .
go run ./tools/licenses -C mobile/core -goos android -o licenses/go-android.txt ./rcvcore golang.org/x/mobile/bind/seq
```

When the bundled sing-box version changes (`windows/fetch-bins.ps1`,
`linux/fetch-bins.sh`, `server/bcserver.sh`), update the version and source
links in the sing-box section above.
