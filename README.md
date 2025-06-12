# NVS - Node Version Switcher

[![GitHub release](https://img.shields.io/github/release/rp01/nvs.svg)](https://github.com/rp01/nvs/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-windows%20%7C%20macos%20%7C%20linux-lightgrey.svg)]()

**🚀 A fast, lightweight Node.js version manager that requires NO admin
privileges!**

Built with Deno and compiled to a single binary, NVS solves the common pain
points of existing Node version managers:

- ✅ **No admin/root privileges required** - Works entirely in user space
- ✅ **Single binary** - No dependencies, no installation scripts
- ✅ **Cross-platform** - Windows, macOS, and Linux support
- ✅ **Cross-architecture** - Install versions for different
  platforms/architectures
- ✅ **Fast switching** - Instant version changes without system modifications
- ✅ **Isolated installations** - Each version completely separate

## 🎯 Why NVS?

Traditional Node version managers like nvm have several limitations:

| Problem                              | Traditional NVM | NVS Solution              |
| ------------------------------------ | --------------- | ------------------------- |
| Requires admin privileges on Windows | ❌              | ✅ No admin needed        |
| Complex installation process         | ❌              | ✅ Single binary download |
| System-wide PATH modifications       | ❌              | ✅ User-space only        |
| Platform-specific versions only      | ❌              | ✅ Cross-platform support |
| Dependencies on system tools         | ❌              | ✅ Zero dependencies      |

## 📦 Installation

### Quick Install

Download the latest binary for your platform from
[releases](https://github.com/rp01/nvs/releases):

```bash
# Linux/macOS
curl -L https://github.com/rp01/nvs/releases/latest/download/nvs-linux-x64 -o nvs
chmod +x nvs
sudo mv nvs /usr/local/bin/  # Optional: for global access

# Windows
# Download nvs-windows-x64.exe and place it in your PATH
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/rp01/nvs.git
cd nvs

# Build with Deno
deno compile --allow-all --output nvs nvs.ts

# Cross-compile for other platforms
deno compile --allow-all --target x86_64-pc-windows-msvc --output nvs-windows.exe nvs.ts
deno compile --allow-all --target x86_64-apple-darwin --output nvs-macos nvs.ts
deno compile --allow-all --target x86_64-unknown-linux-gnu --output nvs-linux nvs.ts
```

## 🚀 Quick Start

```bash
# Install your first Node.js version
nvs install 20.5.0

# Switch to the installed version
nvs use 20.5.0

# List all installed versions
nvs list

# Install multiple versions
nvs install 18.17.0
nvs install 22.16.0

# Switch between versions instantly
nvs use 18.17.0
```

## 📖 Usage

### Basic Commands

```bash
# Install a Node.js version
nvs install <version>

# Switch to a version
nvs use <version>

# List installed versions
nvs list

# Show current active version
nvs current

# Uninstall a version
nvs uninstall <version>

# Show help
nvs help
```

### Cross-Platform Installation

Install Node.js versions for different platforms and architectures:

```bash
# Install for different operating systems
nvs install 20.5.0 --os linux --arch x64
nvs install 20.5.0 --os windows --arch x64
nvs install 20.5.0 --os darwin --arch arm64

# Use cross-platform versions (with compatibility warning)
nvs use 20.5.0 --os linux --arch x64
```

### Supported Platforms & Architectures

| Platform | Aliases           | Architectures         |
| -------- | ----------------- | --------------------- |
| Windows  | `windows`, `win`  | `x64`, `x86`, `arm64` |
| macOS    | `darwin`, `macos` | `x64`, `arm64`        |
| Linux    | `linux`           | `x64`, `x86`, `arm64` |

## 🔧 Advanced Usage

### Environment Setup

After switching versions, NVS provides activation commands:

```bash
# After running 'nvs use 20.5.0'
export PATH="/home/user/.nvs/versions/20.5.0/bin:$PATH"

# Or use the generated activation script
source ~/.nvs/activate.sh  # Linux/macOS
# or
~/.nvs/activate.bat        # Windows
```

### Team Development

Share Node.js versions across your team:

```bash
# .nvmrc equivalent - install specific version for project
nvs install 18.17.0
nvs use 18.17.0

# Install versions for different deployment targets
nvs install 18.17.0 --os linux --arch arm64  # ARM servers
nvs install 18.17.0 --os linux --arch x64    # x64 servers
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Setup Node.js with NVS
  run: |
    curl -L https://github.com/rp01/nvs/releases/latest/download/nvs-linux-x64 -o nvs
    chmod +x nvs
    ./nvs install 18.17.0
    ./nvs use 18.17.0
    export PATH="$HOME/.nvs/versions/18.17.0/bin:$PATH"
```

## 📁 Directory Structure

NVS organizes everything in `~/.nvs/`:

```
~/.nvs/
├── bin/                    # NVS binary location
│   └── nvs                 # Self-installed binary
├── versions/               # Installed Node.js versions
│   ├── 18.17.0/           # Native platform version
│   ├── 20.5.0-linux-x64/ # Cross-platform version
│   └── 22.16.0-darwin-arm64/
├── current                 # Current version marker
├── current-bin/           # Symlinks to current version
└── activate.sh            # Activation script
```

## 🛠️ Development

### Prerequisites

- [Deno](https://deno.land/) 1.36+

### Running from Source

```bash
# Run directly with Deno
deno run --allow-all nvs.ts install 20.5.0

# Or compile and run
deno compile --allow-all --output nvs nvs.ts
./nvs install 20.5.0
```

### Testing

```bash
# Test installation
./nvs install 18.17.0
./nvs list
./nvs use 18.17.0

# Test cross-platform
./nvs install 20.5.0 --os linux --arch x64
./nvs list
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Setup

1. Fork the repository
2. Clone your fork: `git clone https://github.com/rp01/nvs.git`
3. Create a feature branch: `git checkout -b feature/amazing-feature`
4. Make your changes
5. Test thoroughly
6. Commit your changes: `git commit -am 'Add amazing feature'`
7. Push to the branch: `git push origin feature/amazing-feature`
8. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file
for details.

## 🙏 Acknowledgments

- [Node.js](https://nodejs.org/) - For providing the runtime we're managing
- [Deno](https://deno.land/) - For the fantastic runtime that makes this
  possible
- [nvm](https://github.com/nvm-sh/nvm) - For inspiration on Node version
  management

## 🐛 Issues & Support

- **Bug Reports**: [GitHub Issues](https://github.com/rp01/nvs/issues)
- **Feature Requests**: [GitHub Issues](https://github.com/rp01/nvs/issues)
- **Questions**: [GitHub Discussions](https://github.com/rp01/nvs/discussions)

## 🔄 Changelog

### v1.0.0

- Initial release
- Cross-platform Node.js version management
- Single binary distribution
- No admin privileges required
- Cross-architecture support

---

**Made with ❤️ using Deno**
