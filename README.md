# MIGP library

Forked from [Cloudflare's MIGP (Might I Get Pwned)](https://github.com/cloudflare/migp-go), this project is an implementation of MIGP 2.0 version. 
MIGP protocol can be used to build privacy-preserving compromised credential checking services.
Read [the MIGP 2.0 paper](https://eprint.iacr.org/2023/1848.pdf), [the original MIGP paper](https://arxiv.org/pdf/2109.14490.pdf), or the [blog post](https://blog.cloudflare.com/privacy-preserving-compromised-credential-checking) for more details.

## Quick start

### Build

	mkdir -p bin && go build -o bin/ ./cmd/...

### Test

	go test ./...

### Generate server configuration and start MIGP server

Start a server that processes and stores breach entries from the input file.

	cat testdata/test_breach.txt | bin/server &
	
### Query MIGP server

Read entries in from the input file and query a MIGP server.  By default, the
target is set to a locally-running MIGP server, but the target flag can be used
to target production MIGP servers such as https://migp.cloudflare.com.

	cat testdata/test_queries.txt | bin/client [--target <target-server>]

## Advanced usage

Run the client and server commands with `--help` for more options, including
custom configuration support.
