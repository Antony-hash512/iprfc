# IPRFC

`iprfc` is a tool to download all RFCs in PDF form, store them on IPFS, and index them using the Lens search engine.

> [!NOTE]
> **2026 Restoration:** This utility is a fork of a 6-year-old `iprfc` project that had stopped working. In April 2026, this fork (by Antony-hash512) was fully restored and enhanced. Improvements include a reliable PDF source mirror, and a modernized CLI with resuming support (`--min.rfc`, `--overwrite`), an informative progress tracker, and a smart auto-stop mechanism to handle the end of the RFC numbering space gracefully.

# Installation

Before proceeding you'll need to have a valid install of Go 1.22+.

1) Download dependencies with `go mod download`
2) Build with `make build` and an executable called `iprfc` will be created in the current directory

# Usage

```
NAME:
   iprfc - a tool to download all known RFCs in PDF and add them to IPFS

USAGE:
   iprfc [global options] command [command options] [arguments...]

DESCRIPTION:
   It requires at a minimum being able to access a go-ipfs node, and optionally a Lens endpoint to index against

COMMANDS:
   download-and-save  download all known RFCs and save
   store-and-index    store RFCs onto IPFS and index
   help, h            Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --min.rfc value        the minimum (starting) rfc number to download (default: 1)
   --max.rfc value        the maximum rfc to download, 0 means no max (default: 1)
   --overwrite            overwrite existing PDF files instead of skipping them (default: false)
   --ipfs.endpoint value  the go-ipfs api endpoint to use (default: "127.0.0.1:5001")
   --lens.endpoint value  the lens grpc endpoint to use (default: "127.0.0.1:9998")
   --index                whether or not to initiate lens indexing (default: false)
   --help, -h             show help (default: false)
```

## Downloading RFCs

The most basic functionality of this tool consists of downloading all available RFCs in PDF format, saving them in the current directory. To prevent you from accidentally spamming the IETF website, the default setting is to download the first RFC, and then exit. This can be configured with the `--min.rfc` and `--max.rfc` flags.

**Examples:**

Download the first 10 RFCs:
```bash
./iprfc --max.rfc 10 download-and-save
```

Download RFCs 100 through 200 (useful for resuming):
```bash
./iprfc --min.rfc 100 --max.rfc 200 download-and-save
```

Download all available RFCs (auto-stops after 100 consecutive misses):
```bash
./iprfc --max.rfc 0 download-and-save
```

Re-download and overwrite already existing files:
```bash
./iprfc --max.rfc 10 --overwrite download-and-save
```

### Progress output

The utility prints real-time progress for every RFC it processes:

```
[OK]    rfc1.pdf  (downloaded: 1, skipped: 0, missed: 0)
[OK]    rfc2.pdf  (downloaded: 2, skipped: 0, missed: 0)
[SKIP]  rfc3.pdf (already exists)
[MISS]  rfc4.pdf  (not found on server)

=== Done in 3s. Downloaded: 2, Skipped: 1, Missed: 1 ===
```

### Smart auto-stop

When running in unlimited mode (`--max.rfc 0`), the utility automatically stops after **100 consecutive 404 responses**, indicating that the end of the RFC numbering space has been reached. This prevents infinite loops while still handling gaps in RFC numbering gracefully.

### Skip existing files

By default, already downloaded PDFs are **not re-downloaded**. Use `--overwrite` to force re-downloading. This allows you to safely resume interrupted downloads without wasting time or bandwidth.

## Storing On IPFS And Indexing

Before doing this you'll need to download the RFCs either using the `download-and-save` command, or doing it manually, but who wants to do it manually? ;)

Because not everyone will have access to a Lens gRPC endpoint (Lens is open-source btw so you can easily do this), the default behavior of the `store-and-index` command is simply to add the RFCs to IPFS. 

One thing to note is that this will pick up **ANY** PDF's in the current directory, so make sure you run this without any sensitive files in place.

To save all RFCs onto ipfs and not index run `./iprfc store-and-index`.

To save all RFCs onto IPFS and index run `./iprfc --index store-and-index`.
