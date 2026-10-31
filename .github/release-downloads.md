## Downloads

**Windows** (x64)

- `GitGo-{version}-windows-amd64-setup.exe`: the installer, recommended. It installs GitGo in Program Files with Start menu and desktop shortcuts and an uninstaller, and installs the Microsoft WebView2 runtime if it is missing. Needs administrator rights.
- `GitGo-{version}-windows-amd64.exe`: the portable app. Nothing to install; run it from any folder. It uses the WebView2 runtime that comes with Windows 11 and current Windows 10, and offers to download it if it is missing.

**macOS** (Intel and Apple Silicon)

- `GitGo-{version}-macos-universal.dmg`: open it and drag GitGo to Applications.

**Linux** (x64)

- `gitgo_{version}_amd64.deb`: for Debian 12+, Ubuntu 22.04+ and distributions based on them. Install it with `sudo apt install ./gitgo_{version}_amd64.deb`, which also installs GTK 3 and WebKitGTK 4.1.
- `GitGo-{version}-linux-amd64.tar.gz`: for other distributions. The `gitgo` binary, with a desktop entry and icon to install by hand. Needs GTK 3 and WebKitGTK 4.1 (`webkit2gtk4.1` on Fedora, `webkit2gtk-4.1` on Arch).

**Other files**

- `SHA256SUMS`: the SHA-256 checksum of each file above, to check a download with `sha256sum` (Linux), `shasum -a 256` (macOS) or `Get-FileHash` (PowerShell).
- *Source code* (zip and tar.gz): added by GitHub. The source code at this tag, not a build.

### First launch

GitGo is not code-signed yet, so Windows and macOS ask you to confirm the first launch:

- **Windows:** when SmartScreen says *Windows protected your PC*, click **More info**, then **Run anyway**.
- **macOS:** when macOS says it could not verify GitGo, click **Done**, open **System Settings → Privacy & Security**, and click **Open Anyway** next to the message about GitGo.
