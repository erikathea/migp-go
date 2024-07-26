// Copyright (c) 2021 Cloudflare, Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/erikathea/migp-go/pkg/migp"
	"github.com/erikathea/migp-go/pkg/mutator"

	_ "github.com/lib/pq"
)

// newServer returns a new server initialized using the provided configuration
func newServer(cfg migp.ServerConfig) (*server, error) {
	migpServer, err := migp.NewServer(cfg)
	if err != nil {
		return nil, err
	}

	dbConnectionString := os.Getenv("DB_CONNECTION_ST")
	if dbConnectionString == "" {
		log.Println("DB_CONNECTION_ST environment variable not set. Using default localhost connection string.")
		dbConnectionString = "user=cs-db password=hacker dbname=cs-db sslmode=disable host=localhost"
	}

	log.Printf("Using database connection string: %s", dbConnectionString)
	db, err := sql.Open("postgres", dbConnectionString)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}

	kv, err := newKVStore(db)
	if err != nil {
		return nil, err
	}

	return &server{
		migpServer: migpServer,
		kv:		 kv,
	}, nil
}

// server wraps a MIGP server and backing KV store
type server struct {
	migpServer *migp.Server
	kv		 *kvStore
}

// handler handles client requests
func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/evaluate", s.handleEvaluate)
	mux.HandleFunc("/config", s.handleConfig)
	return mux
}

// GenerateRandomString generates a random λ-bits long string
func GenerateRandomString(bits int) ([]byte, error) {
	bytes := int(math.Ceil(float64(bits) / 8.0))
	randomBytes := make([]byte, bytes)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	return randomBytes, nil
}

// insert encrypts a credential pair and stores it in the configured KV store
func (s *server) insert(username, password, metadata []byte, numVariants int, includeUsernameVariant bool, phaseNum int, usePagPassGPT bool) error {
	var (
		newEntry []byte
		err error
		passwordVariants [][]byte
	)
	bucketIDHex := migp.BucketIDToHex(s.migpServer.BucketID(username))
	log.Println("----ID ", bucketIDHex)
	if phaseNum==1 {
		newEntry, err := s.migpServer.EncryptBucketEntry(username, password, migp.MetadataBreachedPassword, metadata)
		if err != nil {
			return err
		}

		if !s.kv.checkIfUnique(newEntry) {
			return errors.New("skipping duplicate entry")
		}

		err = s.kv.Append(bucketIDHex, newEntry)
		if err != nil {
			return err
		}
		s.kv.insertShadow(bucketIDHex, newEntry)
		log.Println("newEntry ", base64.StdEncoding.EncodeToString(newEntry))

		if includeUsernameVariant {
			newEntry, err = s.migpServer.EncryptBucketEntry(username, nil, migp.MetadataBreachedUsername, metadata)
			if err != nil {
				return err
			}

			if !s.kv.checkIfUnique(newEntry){
				log.Println("-- skipping duplicate username-variant entry")
			} else {
				err = s.kv.Append(bucketIDHex, newEntry)
				if err != nil {
					return err
				}
				s.kv.insertShadow(bucketIDHex, newEntry)
				log.Println("-- includeUsernameVariant ", base64.StdEncoding.EncodeToString(newEntry))
			}
		}
	} else if phaseNum==2 {
		if usePagPassGPT {
			cwd, err_ := os.Getwd()
			if err_ != nil {
			    fmt.Println("Error getting current working directory:", err_)
			    return err_
			}
			cmd := exec.Command(cwd+"/run_pagpassgpt.sh", string(password), fmt.Sprintf("%d", numVariants))

			var out bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err != nil {
				fmt.Println("Error executing script:", err)
				fmt.Println("Script stderr:", stderr.String())
				return err
			}
			outputStr := out.String()
			passwords := strings.Split(outputStr, "\n")
			passwordSet := make(map[string]struct{}) // Create a set to store unique passwords
			for _, password := range passwords {
				if password != "" {
					passwordSet[password] = struct{}{}
				}
			}
			for password := range passwordSet {
				passwordVariants = append(passwordVariants, []byte(password))
				log.Println("   gpt-variant ", string(password))
			}
		} else {
			passwordVariants = mutator.NewRDasMutator().Mutate(password, numVariants)
		}
		for _, variant := range passwordVariants {
			newEntry, err = s.migpServer.EncryptBucketEntry(username, variant, migp.MetadataSimilarPassword, metadata)
			// Ensure the value is unique before appending
			attempt := 0
			for !s.kv.checkIfUnique(newEntry) && attempt < 10 {
				randomString, _ := GenerateRandomString(256)
				altVariant := mutator.NewRDasMutator().Mutate(randomString, 1)
				newEntry, err = s.migpServer.EncryptBucketEntry(username, altVariant[0], migp.MetadataSimilarPassword, metadata)
				attempt++
			}
			if err != nil {
				return err
			}
			err = s.kv.Append(bucketIDHex, newEntry)
			if err != nil {
				return err
			}
			s.kv.insertShadow(bucketIDHex, newEntry)
		}
	}

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
