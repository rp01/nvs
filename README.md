# NVS - Node Version Switcher

[![GitHub release](https://img.shields.io/github/release/rp01/nvs.svg)](https://github.com/rp01/nvs/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-windows%20%7C%20macos%20%7C%20linux-lightgrey.svg)]()

```
███╗   ██╗██╗   ██╗███████╗
████╗  ██║██║   ██║██╔════╝
██╔██╗ ██║██║   ██║███████╗
██║╚██╗██║╚██╗ ██╔╝╚════██║
██║ ╚████║ ╚████╔╝ ███████║
╚═╝  ╚═══╝  ╚═══╝  ╚══════╝
```

**🚀 A fast, lightweight Node.js version manager that requires NO admin privileges!**

Built with Go and featuring a beautiful interactive TUI, NVS is a single binary with zero dependencies.

## ✨ Features

- ✅ **No admin/root privileges required** - Works entirely in user space
- ✅ **Interactive TUI** - Beautiful terminal interface, just run `nvs`
- ✅ **Single binary** - No dependencies, no installation scripts
- ✅ **Cross-platform** - Windows, macOS, and Linux
- ✅ **VPN/Proxy friendly** - Built-in TLS skip option for corporate networks
- ✅ **Fast switching** - Instant version changes
- ✅ **Lightweight** - ~10MB binary size

## 📸 Screenshots

### Interactive Mode
Just run `nvs` with no arguments:

```
███╗   ██╗██╗   ██╗███████╗
████╗  ██║██║   ██║██╔════╝
██╔██╗ ██║██║   ██║███████╗
██║╚██╗██║╚██╗ ██╔╝╚════██║
██║ ╚████║ ╚████╔╝ ███████║
╚═╝  ╚═══╝  ╚═══╝  ╚══════╝
  Node Version Switcher • No admin required

╭──────────────────────────────────────────────╮
│                                              │
│  ▸ 📦  Install Node.js                       │
│        Download and install a new version   │
│                                              │
│    📋  List/Switch                           │
│    🗑️   Uninstall                            │
│    🔧  Setup                                 │
│    🔓  Toggle TLS Skip                       │
│    ❓  Help                                  │
│    👋  Exit                                  │
│                                              │
│  ──────────────────────────────────────────  │
│   2 installed   Active: v22.22.0             │
│                                              │
╰──────────────────────────────────────────────╯

  ↑↓ navigate │ ⏎ select │ q quit
```

## 📦 Installation

### Download Binary

Download the latest binary for your platform from [Releases](https://github.com/rp01/nvs/releases):

| Platform | Download |
|----------|----------|
| Windows x64 | `nvs-windows-amd64.exe` |
| Windows ARM64 | `nvs-windows-arm64.exe` |
| macOS Intel | `nvs-macos-amd64` |
| macOS Apple Silicon | `nvs-macos-arm64` |
| Linux x64 | `nvs-linux-amd64` |
| Linux ARM64 | `nvs-linux-arm64` |

### Quick Install

**macOS / Linux:**
```bash
# Download (replace with your platform)
curl -L https://github.com/rp01/nvs/releases/latest/download/nvs-linux-amd64 -o nvs
chmod +x nvs
./nvs setup
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri "https://github.com/rp01/nvs/releases/latest/download/nvs-windows-amd64.exe" -OutFile "nvs.exe"
.\nvs.exe setup
```

### Build from Source

```bash
git clone https://github.com/rp01/nvs.git
cd nvs
go build -o nvs .
./nvs setup
```

## 🚀 Quick Start

### Interactive Mode (Recommended)

Just run `nvs` with no arguments to launch the interactive TUI:

```bash
nvs
```

Use arrow keys to navigate, Enter to select.

### CLI Mode

```bash
# Install a Node.js version
nvs install 22
nvs install lts
nvs install 20.10.0

# Switch versions
nvs use 22

# List installed versions
nvs list

# Show current version
nvs current

# Uninstall a version
nvs uninstall 20
```

## 📖 Commands

| Command | Description |
|---------|-------------|
| `nvs` | Launch interactive TUI |
| `nvs install <version>` | Install a Node.js version |
| `nvs use <version>` | Switch to a version |
| `nvs list` | List installed versions |
| `nvs current` | Show active version |
| `nvs uninstall <version>` | Remove a version |
| `nvs setup` | Initialize NVS and configure PATH |
| `nvs help` | Show help message |

### Version Formats

| Format | Example | Description |
|--------|---------|-------------|
| Major | `22` | Latest 22.x.x |
| Full | `22.10.0` | Specific version |
| LTS | `lts` | Latest LTS release |
| Latest | `latest` | Latest available |

## 🔐 Corporate VPN / Proxy Support

If you're behind a corporate VPN (Cato, Zscaler, etc.) that does TLS inspection, you may encounter certificate errors.

### CLI Flag
```bash
nvs install 22 --insecure
nvs --insecure install lts
```

### Interactive Mode
Select **"🔓 Toggle TLS Skip"** from the menu before installing.

## 📁 Directory Structure

NVS stores everything in `~/.nvs/`:

```
~/.nvs/
├── bin/           # NVS binary
│   └── nvs
├── versions/      # Installed Node.js versions
│   ├── v20.10.0/
│   └── v22.22.0/
└── current        # Symlink to active version
```

## ⚙️ PATH Configuration

After running `nvs setup`, add to your shell config:

**Bash (~/.bashrc):**
```bash
export PATH="$HOME/.nvs/bin:$HOME/.nvs/current/bin:$PATH"
```

**Zsh (~/.zshrc):**
```bash
export PATH="$HOME/.nvs/bin:$HOME/.nvs/current/bin:$PATH"
```

**PowerShell:**
```powershell
$env:Path = "$env:USERPROFILE\.nvs\bin;$env:USERPROFILE\.nvs\current;$env:Path"
```

**Windows CMD:**
```cmd
set PATH=%USERPROFILE%\.nvs\bin;%USERPROFILE%\.nvs\current;%PATH%
```

## 🛠️ Development

### Prerequisites
- Go 1.24+

### Build

```bash
# Build for current platform
go build -o nvs .

# Cross-compile
GOOS=windows GOARCH=amd64 go build -o nvs-windows.exe .
GOOS=darwin GOARCH=arm64 go build -o nvs-macos-arm64 .
GOOS=linux GOARCH=amd64 go build -o nvs-linux .
```

### Dependencies

NVS uses only the [Charm](https://charm.sh/) ecosystem for the TUI:
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - TUI components
- `github.com/charmbracelet/lipgloss` - Styling

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Commit your changes: `git commit -am 'Add amazing feature'`
4. Push to the branch: `git push origin feature/amazing-feature`
5. Open a Pull Request

## 📝 License

MIT License - see [LICENSE](LICENSE) for details.

## 🐛 Issues & Support

- **Bug Reports**: [GitHub Issues](https://github.com/rp01/nvs/issues)
- **Feature Requests**: [GitHub Issues](https://github.com/rp01/nvs/issues)

---

**Made with ❤️ in Go**