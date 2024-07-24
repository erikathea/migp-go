# MIGP library

Forked from [Cloudflare's MIGP (Might I Get Pwned)](https://github.com/cloudflare/migp-go), this project is an implementation of MIGP 2.0 version. 
MIGP protocol can be used to build privacy-preserving compromised credential checking services.
Read [the MIGP 2.0 paper](https://eprint.iacr.org/2023/1848.pdf), [the original MIGP paper](https://arxiv.org/pdf/2109.14490.pdf), or the [blog post](https://blog.cloudflare.com/privacy-preserving-compromised-credential-checking) for more details.

## Quick start

### Build

	mkdir -p bin && go build -o bin/ ./cmd/...

### Test

	go test ./...


### MIGP Configuration
Default `config` file is included in the repo. Please set a `privateKey` using OPRF Suite 256.

	// Generate a private key using the OPRF suite
	"github.com/cloudflare/circl/oprf"
	suite := oprf.SuiteP256
	privateKey, err := oprf.GenerateKey(suite, rand.Reader)

	// Serialize the private key to a hex string
    privateKeyBytes,_ := privateKey.MarshalBinary()
    privateKeyHex := hex.EncodeToString(privateKeyBytes)
    fmt.Printf("Serialized OPRF private key: %s\n", privateKeyHex)


### PostgreSQL as KV Store

This version uses PostgreSQL for key-value storage. Please set `DB_CONNECTION_ST` environment variable. 

By default, there is a hard-coded (not ideal) default localhost connection string you can modify in the code `user=csdb password=hacker dbname=cs-db sslmode=disable host=localhost`.

	`echo $DB_CONNECTION_ST`
	`export DB_CONNECTION_ST="user=csdb password=hacker dbname=cs-db host=az-db-pg.postgres.database.azure.com sslmode=require"`


### Start MIGP Data Processing

*Phase 1* Storing username-password

	`cat testdata/test_breach.txt | bin/server -config=./config -phaseone=true -username-variant=true`

*Phase 2* Storing username-password variants

	`cat testdata/test_breach.txt | bin/server -config=./config -phasetwo=true -num-variants=10`



### Start MIGP server

Start a local server that processes and stores breach entries from the input file.

	bin/server -start-server=true -config=./config
	


### Query MIGP server

Read entries in from the input file and query a MIGP server.  By default, the
target is set to a locally-running MIGP server, but the target flag can be used
to target production MIGP servers such as https://migp.cloudflare.com.

	cat testdata/test_queries.txt | bin/client [--target <target-server>]

## Advanced usage

Run the client and server commands with `--help` for more options, including
custom configuration support.
