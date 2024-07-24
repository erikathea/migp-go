export DB_CONNECTION_ST="user=csdb password=Blu3sho3s21! dbname=csdb host=az-db-pg.postgres.database.azure.com sslmode=require"

go build -o bin/ ./cmd/...

cat testdata/test_breach.txt | bin/server -config=./localconfig -phaseone=true -start-server=false
cat testdata/test_breach.txt | bin/server -config=./localconfig -phaseone=false -start-server=false -num-variants=1