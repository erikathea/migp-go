export DB_CONNECTION_ST=""

go build -o bin/ ./cmd/...

cat testdata/test_breach.txt | bin/server -config=./localconfig -phaseone=true -start-server=false
cat testdata/test_breach.txt | bin/server -config=./localconfig -phaseone=false -start-server=false -num-variants=1