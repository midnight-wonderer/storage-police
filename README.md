# `storage-police` - Catch Storage Fraudsters

Storage Police helps identify fake storage media, and fraudulent capacity labels—including HDDs, SSDs, flash drives, SD cards, TF cards, and other storage devices.

## Use cases

- **Detect Hardware Failures**: Identify disk errors, flash memory degradation, or other physical hardware issues.
- **Expose Fake Capacity**: Detect fraudulent media that report more storage than they physically possess.
- **Securely Wipe Data**: Use the `scrub` subcommand to reliably overwrite all data.

## How it Works

The tool __writes__ a unique pseudorandom sequence onto the target storage media. You can then subject the device to real-world conditions—such as summer heat—and use the tool to __retrieve and verify__ the sequence against the expected values to ensure the media can reliably hold your data.

## Quick starts

### Write

~~~bash
storage-police write --seed Genie /dev/sdc
~~~

The `seed` helps ensure the sequence written is unique to you.
You could use your nickname, for instance.

### Read (Verify)

~~~bash
storage-police read --seed Genie /dev/sdc
~~~

The `seed` must match the one used when writing the sequence.

### Notes

* **Warning**: The writing process will completely wipe your storage device. Ensure you do not have any important data on the device before proceeding.
* On most systems, you must run these commands with `sudo` to avoid "permission denied" errors.
* Power cycle the device (unplug, wait, and replug) before reading to verify data retention.

### Scrub (Extra Mode)

~~~bash
storage-police scrub /dev/sdc
~~~

Fill the device using a ramdomized seed.

## Why it is better than other tools

* **Unfakable Verification**: Uses an unpredictable pseudo-random sequence to prevent disk controllers from cheating via transparent data compression or deduplication.
* **Realistic Full-Disk Testing**: Mirrors real-world usage by ensuring every byte is physically stored and can be accurately retrieved after filling the media to capacity.
* **Optimized Performance**: Focuses strictly on data retention and integrity, resulting in significantly faster test times than general-purpose diagnostic tools.
* **Streamlined UX**: Inspired by `disktest` but redesigned for simplicity, offering a more intuitive interface with fewer, more focused parameters.

## Bonus

* **Performance Monitoring**: Shows sequential write and read performance during operations, serving as an unintentional benchmark tool.
* **Data Scrubbing**: The `scrub` subcommand is available to overwrite data on magnetic platters. It is essentially the `write` mode with a cryptographically randomized `seed`.

## Available Command-line Options

~~~bash
storage-police --help
~~~

## Shell completion

### Quick & Dirty

~~~sh
# bash
source <(storage-police completion bash)

# zsh
source <(storage-police completion zsh)

# fish
source <(storage-police completion fish)
~~~

### Permanent

~~~bash
# bash example
storage-police completion bash > ~/.local/share/bash-completion/completions/storage-police
~~~

## Technical Details & Algorithm

### Is it secure?
Yes. The tool generates a unique, deterministic stream based on your provided seed. Without knowing the seed, it is virtually impossible for a malicious storage controller to predict the sequence to "fake" a successful verification.

### Is it fast?
Yes. The sequence generation is highly optimized and will likely be much faster than your storage media's read or write speeds. The bottleneck will almost always be your storage hardware, not the sequence generator.

### What algorithm does it use?
The pseudo-random stream is generated using **BLAKE3** in Extendable Output Function (XOF) mode. This provides cryptographically strong randomness.

~~~
OUTPUT_STREAM := BLAKE3_XOF(SEED)
~~~

### Is the `scrub` mode secure?
The `scrub` subcommand writes cryptographically secure pseudorandom data, as discussed, to the device. The tool utilizes your system's cryptographically secure random number generator (CSPRNG) for seed initialization.

While the tool is as secure as a software-based approach can be, we cannot guarantee absolute data sanitization. For instance, modern SSDs utilize internal controllers for wear-leveling and block remapping, which may result in some physical cells not being overwritten, potentially leaving data recoverable via specialized forensic techniques.

## Alternatives
* [**disktest**](https://bues.ch/cms/hacking/disktest), which inspired this tool. Check it out if you enjoy fiddling with algorithms and parameters.
* [**f3** (Fight Flash Fraud)](https://github.com/AltraMayor/f3), the classic. It tests both capacity and performance using multiple access patterns, which makes the testing process take longer.

## License

Storage Police is released under the [BSD 2-Clause License](LICENSE.md).
