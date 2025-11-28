# amdecrypt

A CLI tool for decrypting Apple Music songs in conjunction with a [wrapper server](https://github.com/WorldObservationLog/wrapper).

## 📋 Prerequisites

Must be added to your system's PATH:

- [mp4decrypt](https://www.bento4.com/downloads/)

## 📦 Installation

1. Download the latest binary for your platform from the [releases page](https://github.com/glomatico/amdecrypt/releases/latest)
2. Extract the archive
3. Add the binary to your system's PATH

## 🚀 Usage

This tool is designed to be called by other programs rather than used directly. For example, [gamdl](https://github.com/glomatico/gamdl) will automatically invoke amdecrypt when needed.

### Manual Usage

If needed, you can also run it directly from the command line:

```bash
amdecrypt <agentIp> <mp4decryptPath> <id> <key> <inputPath> <outputPath>
```

### Arguments

| Argument         | Description                      |
| ---------------- | -------------------------------- |
| `agentIp`        | IP address of the wrapper server |
| `mp4decryptPath` | Path to the mp4decrypt binary    |
| `id`             | Track ID                         |
| `key`            | FairPlay Streaming Key           |
| `inputPath`      | Path to the encrypted file       |
| `outputPath`     | Path for the decrypted output    |

## ⚠️ Disclaimer

This tool was mostly created with AI assistance.
