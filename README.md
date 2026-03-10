# 🚔 Storage Police — Stop Storage Fraud

[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-BSD--2--Clause-blue?style=for-the-badge)](LICENSE.md)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=for-the-badge)](http://makeapullrequest.com)

**Storage Police** is a utility designed to expose fake storage media and fraudulent capacity labels. Whether it's an HDD, SSD, flash drive, or SD card, this tool ensures that what's on the label is what's actually in the hardware.

---

## 🚀 Core Mission

- **🔍 Expose Fake Capacity**: Detect fraudulent media that report more storage than they physically possess.
- **🚨 Detect Hardware Failures**: Identify disk errors, flash memory degradation, or other physical hardware issues.

## 🎁 Bonus Capabilities

- **⚡ Performance Benchmarking**: Real-time monitoring of sequential write and read speeds, serving as an unintentional benchmark.
- **🧹 Secure Data Wiping**: A dedicated `shred` mode to reliably overwrite devices with unpredictable random data.

## ✨ The "Secret Sauce" (Why it wins)

- **🕵️‍♀️ Unfakable Verification**: Uses an unpredictable BLAKE3-based sequence to prevent disk controllers from cheating via transparent data compression or deduplication.
- **📍 Byte-Perfect Testing**: Mirrors real-world conditions by ensuring every byte is physically stored and can be accurately retrieved.
- **🏍️ High-Efficiency Testing**: Specifically focused on retention and integrity, resulting in significantly shorter test times than general-purpose tools.
- **💻 Streamlined UX**: Inspired by `disktest` but redesigned for simplicity with intuitive parameters and clear, modern feedback.

---

## 📦 Installation

### 💾 Option 1: Pre-built Binaries (Quickest)
Download the latest pre-compiled binaries for Linux, macOS, and Windows from our [Releases Page](https://github.com/midnight-wonderer/storage-police/releases).

> [!NOTE]
> For full transparency and peace of mind, all pre-built binaries are compiled automatically directly from source via [GitHub Actions](https://github.com/midnight-wonderer/storage-police/actions/workflows/release.yml).

### 🛠 Option 2: Build From Source
If you have [Go](https://golang.org/doc/install) installed:

```bash
go install github.com/midnight-wonderer/storage-police@latest
```

---

## 🧭 Quick Start

### 1. Write
Fill the target device with a unique, deterministic pseudo-random sequence.

```bash
sudo storage-police write --seed "MySecretSeed" /dev/sdc
```
> [!TIP]
> Use a unique `seed` to ensure the pattern is specific to your test.
> You could use your nickname, for instance.

### 2. Read (Verify)
Verify that every byte written can be retrieved accurately.

```bash
sudo storage-police read --seed "MySecretSeed" /dev/sdc
```
> [!IMPORTANT]
> The `seed` must be identical to the one used during the `write` phase.

### 3. Read the docs
For all the extra goodies you might not know you want yet.

```bash
storage-police --help
```

---

## ⚠️ Critical Warning

> [!CAUTION]  
> **Data Loss is Permanent.**  
> The `write` commands will **completely wipe** the target storage device. Double-check your device path (e.g., `/dev/sdc`) before proceeding. We are not responsible for any accidental data loss.

### FWIW
1. **Sudo Access**: Most systems require elevated permissions to access raw block devices. If the utility is in your `$PATH`, try: `sudo $(which storage-police)`.
2. **Power Cycle**: For the most reliable "fake capacity" detection, unplug and replug the device after the `write` phase before starting the `read` phase. This clears any volatile cache.

---

## 🔬 Technical Details

### How it Works
The tool generates a unique, deterministic stream based on your provided seed. Without knowing the seed, it is virtually impossible for a malicious storage controller to "fake" a successful verification by predicting the next sequence.

### The Algorithm
The pseudo-random stream is generated using **BLAKE3** in Extendable Output Function (XOF) mode.
```text
OUTPUT_STREAM := BLAKE3_XOF(SEED)
```
This provides cryptographically strong randomness at speeds that typically exceed the storage media's physical limits.

### Is `shred` Secure?
Yes. The `shred` subcommand uses your system's cryptographically secure random number generator (CSPRNG) to initialize the seed, ensuring the data written is unpredictable.

> [!NOTE]  
> On SSDs, internal wear-leveling might prevent 100% physical erasure of every NAND cell. For ultra-sensitive data, physical destruction is recommended.

---

## ⚙️ Shell Completion

Enable command-line completion for a better experience:

```bash
# To try it now:
source <(storage-police completion bash)

# To make it permanent:
storage-police completion bash > ~/.local/share/bash-completion/completions/storage-police
```
*(Supports `bash`, `zsh`, and `fish`)*

---

## 💡 Alternatives

- [**disktest**](https://bues.ch/cms/hacking/disktest) - The primary inspiration for this tool.
- [**f3 (Fight Flash Fraud)**](https://github.com/AltraMayor/f3) - A classic tool for testing capacity and performance.

---

## 📜 License

Storage Police is released under the [BSD 2-Clause License](LICENSE.md).  
