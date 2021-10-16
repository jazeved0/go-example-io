# go-example-io

```
go-example-io --mode [mode] --path [path]
```

## ℹ️ About

This repository contains a single short Go program that performs sequential reads and writes to a file over a short amount of time. It is used to test [rAdvisor](https://github.com/elba-docker/radvisor)'s block I/O instrumentation capabilities. The program has three "modes" (selected using the `--mode` parameter):

- `"write"` - this writes 1 MiB of cryptographically random bytes to a file at `--path [path]` 32 KiB at a time, sleeping 0.5 seconds in between each write.
- `"read"` - this reads an entire file at `--path [path]` 32 KiB at a time, sleeping 0.5 seconds in between each read. At the end, it computes the SHA-256 hash of the file and prints it out.
- `"combined"` - this runs the program first in the write mode, and then in the read mode on the same file at `--path [path]`. It also sleeps before, in between, and after (which is useful to have rAdvisor detect the container before it starts performing I/O).

## 🚀 Getting Started

The provided Dockerfile builds the binary and sets its command to be the combined mode, which means it can be used directly to test rAdvisor:

```sh
# (from the root of the repository):
docker build . -t go-example-io
docker run --name go-example-io go-example-io:latest
```

If you wish to run rAdvisor alongside this program, see [rAdvisor](https://github.com/elba-docker/radvisor)'s README for information on how to install and run it.
