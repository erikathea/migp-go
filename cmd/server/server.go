// Copyright (c) 2021 Cloudflare, Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/cloudflare/migp-go/pkg/migp"
	"github.com/cloudflare/migp-go/pkg/mutator"
	"database/sql"
	_ "github.com/lib/pq"
)

// newServer returns a new server initialized using the provided configuration
func newServer(cfg migp.ServerConfig) (*server, error) {
	migpServer, err := migp.NewServer(cfg)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", "user=cs-db password=hacker dbname=cs-db sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	kv, err := newKVStore(db)
	if err != nil {
		return nil, err
	}

	return &server{
		migpServer: migpServer,
		kv:         kv,
	}, nil
}

// server wraps a MIGP server and backing KV store
type server struct {
	migpServer *migp.Server
	kv         *kvStore
}

// handler handles client requests
func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/evaluate", s.handleEvaluate)
	mux.HandleFunc("/config", s.handleConfig)
	return mux
}

// insert encrypts a credential pair and stores it in the configured KV store
func (s *server) insert(username, password, metadata []byte, numVariants int, includeUsernameVariant, phaseOne bool) error {
	var (
        newEntry []byte
        err error
    )
	bucketIDHex := migp.BucketIDToHex(s.migpServer.BucketID(username))
	log.Println("----ID ", bucketIDHex)
	if phaseOne {
		newEntry, err := s.migpServer.EncryptBucketEntry(username, password, migp.MetadataBreachedPassword, metadata)
		if err != nil {
			return err
		}

		err = s.kv.Append(bucketIDHex, newEntry)
		if err != nil {
			return err
		}
		log.Println("newEntry ", base64.StdEncoding.EncodeToString(newEntry))

		if includeUsernameVariant {
			newEntry, err = s.migpServer.EncryptBucketEntry(username, nil, migp.MetadataBreachedUsername, metadata)
			if err != nil {
				return err
			}

			err = s.kv.Append(bucketIDHex, newEntry)
			if err != nil {
				return err
			}
			log.Println("-- includeUsernameVariant ", base64.StdEncoding.EncodeToString(newEntry))
		}
		return nil
	} else {
		passwordVariants := mutator.NewRDasMutator().Mutate(password, numVariants)
		for _, variant := range passwordVariants {
			newEntry, err = s.migpServer.EncryptBucketEntry(username, variant, migp.MetadataSimilarPassword, metadata)
			if err != nil {
				return err
			}

			err = s.kv.Append(bucketIDHex, newEntry)
			if err != nil {
				return err
			}
			log.Println("-- password variant ", base64.StdEncoding.EncodeToString(newEntry))

		}
	}

	bucketContents, err := s.kv.Get(bucketIDHex)
	log.Println("content ", base64.StdEncoding.EncodeToString(bucketContents))
	log.Println("ID ", bucketIDHex)

	return nil
}

// handleIndex returns a welcome message
func (s *server) handleIndex(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Welcome to the MIGP demo server\n")
}

// handleConfig returns the MIGP configuration
func (s *server) handleConfig(w http.ResponseWriter, req *http.Request) {
	encoder := json.NewEncoder(w)
	cfg := s.migpServer.Config().Config
	if err := encoder.Encode(cfg); err != nil {
		log.Println("Writing response failed:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// handleEvaluate serves a request from a MIGP client
func (s *server) handleEvaluate(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Println("Request body reading failed:", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var request migp.ClientRequest
	if err := json.Unmarshal(body, &request); err != nil {
		log.Println("Request body unmarshal failed:", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}

	migpResponse, err := s.migpServer.HandleRequest(request, s.kv)
	if err != nil {
		log.Println("HandleRequest failed:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	respBody, err := migpResponse.MarshalBinary()
	log.Println(respBody)
	if err != nil {
		log.Println("Response serialization failed:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	if _, err := w.Write(respBody); err != nil {
		log.Println("Writing response failed:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
