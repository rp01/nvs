#!/usr/bin/env -S deno run --allow-net --allow-read --allow-write --allow-run --allow-env

import { join } from "https://deno.land/std@0.208.0/path/mod.ts";
import { ensureDir, exists } from "https://deno.land/std@0.208.0/fs/mod.ts";
import { decompress } from "https://deno.land/x/zip@v1.2.5/mod.ts";

interface NodeRelease {
    version: string;
    url: string;
    filename: string;
    ext: string;
}

class NodeVersionSwitcher {
    private homeDir: string;
    private nvsDir: string;
    private versionsDir: string;
    private currentFile: string;
    private binDir: string;

    constructor() {
        this.homeDir = Deno.env.get("HOME") || Deno.env.get("USERPROFILE") || "";
        this.nvsDir = join(this.homeDir, ".nvs");
        this.versionsDir = join(this.nvsDir, "versions");
        this.currentFile = join(this.nvsDir, "current");
        this.binDir = join(this.nvsDir, "bin");
    }

    async init(): Promise<void> {
        await ensureDir(this.nvsDir);
        await ensureDir(this.versionsDir);
        await ensureDir(this.binDir);

        // Copy self to bin directory if not already there
        await this.installSelf();
    }

    private async installSelf(): Promise<void> {
        const currentExe = Deno.execPath();
        const targetPath = join(this.binDir, Deno.build.os === "windows" ? "nvs.exe" : "nvs");

        if (!await exists(targetPath)) {
            console.log("Installing NVS to ~/.nvs/bin/");
            await Deno.copyFile(currentExe, targetPath);

            if (Deno.build.os !== "windows") {
                await Deno.chmod(targetPath, 0o755);
            }

            console.log(`NVS installed to: ${targetPath}`);
            console.log(`Add ${this.binDir} to your PATH to use 'nvs' command globally`);
        }
    }

    private getNodeRelease(version: string, targetOs?: string, targetArch?: string): NodeRelease {
        const platform = targetOs || Deno.build.os;
        const arch = targetArch || Deno.build.arch;

        let platformName: string;
        let ext: string;

        switch (platform) {
            case "windows":
            case "win":
                platformName = "win";
                ext = "zip";
                break;
            case "darwin":
            case "macos":
                platformName = "darwin";
                ext = "tar.gz";
                break;
            case "linux":
                platformName = "linux";
                ext = "tar.xz";
                break;
            default:
                throw new Error(`Unsupported platform: ${platform}. Supported: windows, darwin, linux`);
        }

        // Normalize architecture names
        let archName: string;
        switch (arch) {
            case "x86_64":
            case "x64":
            case "amd64":
                archName = "x64";
                break;
            case "aarch64":
            case "arm64":
                archName = "arm64";
                break;
            case "x86":
            case "i386":
            case "ia32":
                archName = "x86";
                break;
            default:
                // Use as-is for other architectures
                archName = arch;
                break;
        }

        const filename = `node-v${version}-${platformName}-${archName}.${ext}`;

        return {
            version,
            url: `https://nodejs.org/dist/v${version}/${filename}`,
            filename,
            ext
        };
    }

    private async downloadWithProgress(url: string, destination: string): Promise<void> {
        console.log(`Downloading from: ${url}`);

        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`Download failed: ${response.status} ${response.statusText}`);
        }

        const contentLength = response.headers.get("content-length");
        const totalSize = contentLength ? parseInt(contentLength, 10) : 0;

        const file = await Deno.open(destination, { write: true, create: true });
        const reader = response.body?.getReader();

        if (!reader) {
            throw new Error("Failed to get response reader");
        }

        let downloadedSize = 0;

        try {
            while (true) {
                const { done, value } = await reader.read();
                if (done) break;

                await file.write(value);
                downloadedSize += value.length;

                if (totalSize > 0) {
                    const percent = Math.round((downloadedSize / totalSize) * 100);
                    Deno.stdout.writeSync(new TextEncoder().encode(`\rProgress: ${percent}%`));
                }
            }
            console.log("\nDownload completed!");
        } finally {
            file.close();
        }
    }

    private async extractArchive(archivePath: string, extractPath: string, ext: string): Promise<void> {
        console.log("Extracting archive...");

        await ensureDir(extractPath);

        if (ext === "zip") {
            await decompress(archivePath, extractPath);
        } else {
            // Use system tar for .tar.gz and .tar.xz
            const tarCmd = ext === "tar.gz" ? ["tar", "-xzf"] : ["tar", "-xJf"];
            const process = new Deno.Command(tarCmd[0], {
                args: [...tarCmd.slice(1), archivePath, "-C", extractPath],
                stdout: "piped",
                stderr: "piped"
            });

            const { success } = await process.output();
            if (!success) {
                throw new Error("Failed to extract archive");
            }
        }

        // Clean up archive
        await Deno.remove(archivePath);
        console.log("Extraction completed");
    }

    async install(version: string, targetOs?: string, targetArch?: string, force: boolean = false): Promise<void> {
        const osInfo = targetOs ? ` for ${targetOs}` : "";
        const archInfo = targetArch ? `-${targetArch}` : "";

        console.log(`Installing Node.js v${version}${osInfo}${archInfo}...`);

        // Create unique directory name for cross-platform installs
        const versionKey = targetOs || targetArch
            ? `${version}-${targetOs || Deno.build.os}-${targetArch || Deno.build.arch}`
            : version;

        const versionDir = join(this.versionsDir, versionKey);

        // Handle --force: remove existing directory if it exists
        if (force && await exists(versionDir)) {
            console.log(`Forcing reinstall of Node.js v${version}${osInfo}${archInfo}`);
            await Deno.remove(versionDir, { recursive: true });
        } else if (!force && await exists(versionDir)) {
            console.log(`Node.js v${version}${osInfo}${archInfo} is already installed`);
            return;
        }

        const release = this.getNodeRelease(version, targetOs, targetArch);
        const downloadPath = join(this.nvsDir, release.filename);

        try {
            await this.downloadWithProgress(release.url, downloadPath);
            await this.extractArchive(downloadPath, versionDir, release.ext);

            // Reorganize extracted files
            const entries = [];
            for await (const entry of Deno.readDir(versionDir)) {
                entries.push(entry);
            }

            const extractedDir = entries.find(entry =>
                entry.isDirectory && entry.name.startsWith("node-")
            );

            if (extractedDir) {
                const extractedPath = join(versionDir, extractedDir.name);

                // Move contents up one level
                for await (const file of Deno.readDir(extractedPath)) {
                    const src = join(extractedPath, file.name);
                    const dest = join(versionDir, file.name);
                    await Deno.rename(src, dest);
                }

                await Deno.remove(extractedPath);
            }

            console.log(`Node.js v${version}${osInfo}${archInfo} installed successfully`);
            console.log(`📁 Installed to: ${versionDir}`);

            if (targetOs && targetOs !== Deno.build.os) {
                console.log(`⚠️  Note: This is a cross-platform installation for ${targetOs}`);
            }
        } catch (error: unknown) {
            console.error(`Failed to install Node.js v${version}${osInfo}${archInfo}:`, error instanceof Error ? error.message : String(error));

            // Cleanup on failure
            if (await exists(versionDir)) {
                await Deno.remove(versionDir, { recursive: true });
            }
            if (await exists(downloadPath)) {
                await Deno.remove(downloadPath);
            }
            throw error;
        }
    }

    async use(version: string, targetOs?: string, targetArch?: string): Promise<void> {
        // Create version key for lookup
        const versionKey = targetOs || targetArch
            ? `${version}-${targetOs || Deno.build.os}-${targetArch || Deno.build.arch}`
            : version;

        const versionDir = join(this.versionsDir, versionKey);

        if (!await exists(versionDir)) {
            console.error(`Node.js v${version} is not installed.`);

            if (targetOs || targetArch) {
                const osInfo = targetOs ? ` --os ${targetOs}` : "";
                const archInfo = targetArch ? ` --arch ${targetArch}` : "";
                console.error(`Run 'nvs install ${version}${osInfo}${archInfo}' first.`);
            } else {
                console.error(`Run 'nvs install ${version}' first.`);
            }
            return;
        }

        // Check if this is a cross-platform installation
        if (targetOs && targetOs !== Deno.build.os) {
            console.log(`⚠️  Warning: You're trying to use ${targetOs} binaries on ${Deno.build.os}`);
            console.log(`   This will likely not work. Consider installing for your current platform.`);
        }

        const binPath = Deno.build.os === "windows"
            ? versionDir
            : join(versionDir, "bin");

        if (!await exists(binPath)) {
            console.error(`Invalid Node.js installation for v${version}`);
            return;
        }

        // Save current version with full key
        await Deno.writeTextFile(this.currentFile, versionKey);

        // Create symlinks or batch files for easy access
        await this.createVersionLinks(versionKey, binPath);

        console.log(`✅ Switched to Node.js v${version}`);
        if (targetOs || targetArch) {
            console.log(`   Platform: ${targetOs || Deno.build.os}-${targetArch || Deno.build.arch}`);
        }
        console.log(`\n📍 Node.js binaries available at: ${binPath}`);

        if (Deno.build.os === "windows") {
            console.log(`\n🔧 To use globally, add to your PATH:`);
            console.log(`   set PATH=${binPath};%PATH%`);
            console.log(`\n   Or run: setx PATH "${binPath};%PATH%"`);
        } else {
            console.log(`\n🔧 To use globally, add to your PATH:`);
            console.log(`   export PATH="${binPath}:$PATH"`);
            console.log(`\n   Add this to your ~/.bashrc or ~/.zshrc for persistence`);
        }

        // Create activation script
        await this.createActivationScript(binPath);
    }

    private async createVersionLinks(version: string, binPath: string): Promise<void> {
        const linkDir = join(this.nvsDir, "current-bin");
        await ensureDir(linkDir);

        // Remove existing links
        try {
            await Deno.remove(linkDir, { recursive: true });
            await ensureDir(linkDir);
        } catch {
            // Directory might not exist
        }

        const isWindows = Deno.build.os === "windows";
        const nodeExe = isWindows ? "node.exe" : "node";
        const npmExe = isWindows ? "npm.cmd" : "npm";
        const npxExe = isWindows ? "npx.cmd" : "npx";

        const binaries = [nodeExe, npmExe, npxExe];

        for (const binary of binaries) {
            const sourcePath = join(binPath, binary);
            const linkPath = join(linkDir, binary);

            if (await exists(sourcePath)) {
                if (isWindows) {
                    // Create batch file wrapper for Windows
                    const batchContent = `@echo off\n"${sourcePath}" %*`;
                    await Deno.writeTextFile(linkPath.replace(/\.(exe|cmd)$/, ".bat"), batchContent);
                } else {
                    // Create symlink for Unix-like systems
                    try {
                        await Deno.symlink(sourcePath, linkPath);
                    } catch {
                        // Fallback: create shell script wrapper
                        const scriptContent = `#!/bin/bash\nexec "${sourcePath}" "$@"`;
                        await Deno.writeTextFile(linkPath, scriptContent);
                        await Deno.chmod(linkPath, 0o755);
                    }
                }
            }
        }
    }

    private async createActivationScript(binPath: string): Promise<void> {
        const isWindows = Deno.build.os === "windows";
        const scriptExt = isWindows ? ".bat" : ".sh";
        const scriptPath = join(this.nvsDir, `activate${scriptExt}`);

        let scriptContent: string;

        if (isWindows) {
            scriptContent = `@echo off
echo Activating Node.js environment...
set PATH=${binPath};%PATH%
echo Node.js path updated. You can now use 'node', 'npm', and 'npx' commands.
cmd /k`;
        } else {
            scriptContent = `#!/bin/bash
echo "Activating Node.js environment..."
export PATH="${binPath}:$PATH"
echo "Node.js path updated. You can now use 'node', 'npm', and 'npx' commands."
exec "$SHELL"`;
        }

        await Deno.writeTextFile(scriptPath, scriptContent);

        if (!isWindows) {
            await Deno.chmod(scriptPath, 0o755);
        }

        console.log(`\n🚀 Quick activation script created: ${scriptPath}`);
        console.log(`   Run this script to activate Node.js in a new shell session`);
    }

    async list(): Promise<void> {
        console.log("📦 Installed Node.js versions:\n");

        if (!await exists(this.versionsDir)) {
            console.log("   No versions installed");
            return;
        }

        const versions: string[] = [];
        for await (const entry of Deno.readDir(this.versionsDir)) {
            if (entry.isDirectory) {
                versions.push(entry.name);
            }
        }

        if (versions.length === 0) {
            console.log("   No versions installed");
            return;
        }

        const current = await this.getCurrentVersion();
        versions.sort((a, b) => {
            // Sort by version number first, then by platform
            const [verA] = a.split('-');
            const [verB] = b.split('-');
            const versionCompare = verA.localeCompare(verB, undefined, { numeric: true });
            return versionCompare !== 0 ? versionCompare : a.localeCompare(b);
        });

        versions.forEach(versionKey => {
            const marker = versionKey === current ? " ✅ (current)" : "";

            // Parse version key to show readable format
            const parts = versionKey.split('-');
            if (parts.length === 3) {
                const [version, os, arch] = parts;
                console.log(`   ${version} (${os}-${arch})${marker}`);
            } else {
                console.log(`   ${versionKey}${marker}`);
            }
        });
    }

    private async getCurrentVersion(): Promise<string | null> {
        try {
            const content = await Deno.readTextFile(this.currentFile);
            return content.trim();
        } catch {
            return null;
        }
    }

    async current(): Promise<void> {
        const current = await this.getCurrentVersion();
        if (current) {
            console.log(`Currently using Node.js v${current}`);
        } else {
            console.log("No version currently selected");
        }
    }

    async uninstall(version: string): Promise<void> {
        const versionDir = join(this.versionsDir, version);

        if (!await exists(versionDir)) {
            console.log(`Node.js v${version} is not installed`);
            return;
        }

        await Deno.remove(versionDir, { recursive: true });
        console.log(`Node.js v${version} uninstalled`);

        // Clear current if it was the uninstalled version
        const current = await this.getCurrentVersion();
        if (current === version) {
            try {
                await Deno.remove(this.currentFile);
            } catch {
                // File might not exist
            }
        }
    }

    async showHelp(): Promise<void> {
        const help = `
🚀 Node Version Switcher (NVS) - No Admin Required!

USAGE:
  nvs <command> [version] [options]

COMMANDS:
  install <version> [--os <os>] [--arch <arch>] [--force]   Install a Node.js version (force reinstall if exists)
  use <version> [--os <os>] [--arch <arch>]                 Switch to a Node.js version  
  list                                                      List all installed versions
  current                                                   Show currently active version
  uninstall <version>                                       Remove a Node.js version
  help                                                      Show this help message

CROSS-PLATFORM OPTIONS:
  --os <platform>       Target OS: windows, linux, darwin (default: current OS)
  --arch <architecture> Target arch: x64, arm64, x86 (default: current arch)

EXAMPLES:
  # Basic usage
  nvs install 18.17.0                    # Install for current platform
  nvs install 18.17.0 --force            # Force reinstall if already installed
  nvs use 18.17.0                        # Switch to v18.17.0
  nvs list                               # Show all installed versions
  
  # Cross-platform installation  
  nvs install 20.5.0 --os linux --arch x64      # Install Linux x64 version
  nvs install 18.17.0 --os windows --arch x64   # Install Windows x64 version
  nvs install 22.16.0 --os darwin --arch arm64  # Install macOS ARM64 version
  
  # Use cross-platform versions
  nvs use 20.5.0 --os linux --arch x64          # Use Linux version (if compatible)

SUPPORTED PLATFORMS:
  • Windows (windows, win) - x64, x86, arm64
  • macOS (darwin, macos) - x64, arm64  
  • Linux (linux) - x64, arm64, x86

FEATURES:
  ✅ No admin/root privileges required
  ✅ Cross-platform installation support
  ✅ Single binary - no dependencies
  ✅ Fast version switching
  ✅ Isolated installations
  ✅ Multiple architectures per version

All Node.js versions are installed to: ~/.nvs/versions/
    `;
        console.log(help);
    }
}

// Main CLI function
async function main(): Promise<void> {
    const args = Deno.args;
    const nvs = new NodeVersionSwitcher();

    await nvs.init();

    if (args.length === 0) {
        await nvs.showHelp();
        return;
    }

    const command = args[0];
    const version = args[1];

    try {
        switch (command) {
            case "install": {
                if (!version) {
                    console.error("❌ Please specify a version to install");
                    console.log("   Example: nvs install 18.17.0");
                    console.log("   Cross-platform: nvs install 18.17.0 --os linux --arch x64");
                    return;
                }

                // Parse additional arguments for cross-platform installation
                let targetOs: string | undefined;
                let targetArch: string | undefined;
                let force = false;

                for (let i = 2; i < args.length; i++) {
                    if (args[i] === "--os" && i + 1 < args.length) {
                        targetOs = args[i + 1];
                        i++; // Skip next argument
                    } else if (args[i] === "--arch" && i + 1 < args.length) {
                        targetArch = args[i + 1];
                        i++; // Skip next argument
                    } else if (args[i] === "--force") {
                        force = true;
                    }
                }

                await nvs.install(version, targetOs, targetArch, force);
                break;
            }

            case "use": {
                if (!version) {
                    console.error("❌ Please specify a version to use");
                    console.log("   Example: nvs use 18.17.0");
                    console.log("   Cross-platform: nvs use 18.17.0 --os linux --arch x64");
                    return;
                }

                // Parse additional arguments for cross-platform usage
                let targetOs: string | undefined;
                let targetArch: string | undefined;

                for (let i = 2; i < args.length; i++) {
                    if (args[i] === "--os" && i + 1 < args.length) {
                        targetOs = args[i + 1];
                        i++; // Skip next argument
                    } else if (args[i] === "--arch" && i + 1 < args.length) {
                        targetArch = args[i + 1];
                        i++; // Skip next argument
                    }
                }

                await nvs.use(version, targetOs, targetArch);
                break;
            }

            case "list":
            case "ls":
                await nvs.list();
                break;

            case "uninstall":
            case "remove":
            case "rm":
                if (!version) {
                    console.error("❌ Please specify a version to uninstall");
                    console.log("   Example: nvs uninstall 18.17.0");
                    return;
                }
                await nvs.uninstall(version);
                break;

            case "current":
                await nvs.current();
                break;

            case "help":
            case "--help":
            case "-h":
                await nvs.showHelp();
                break;

            default:
                console.error(`❌ Unknown command: ${command}`);
                console.log("   Run 'nvs help' to see available commands");
                Deno.exit(1);
        }
    } catch (error: unknown) {
        console.error(`❌ Error: ${error instanceof Error ? error.message : String(error)}`);
        Deno.exit(1);
    }
}

if (import.meta.main) {
    await main();
}