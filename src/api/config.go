package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
)

const demoAdminPassword = "alligator123"

type cosmosConfig struct {
	Endpoint  string
	Key       string
	Database  string
	Container string
	AllowHTTP bool
}

func loadCosmosConfig() (cosmosConfig, error) {
	names := []string{"COSMOS_ENDPOINT", "COSMOS_KEY", "COSMOS_DATABASE", "COSMOS_CONTAINER"}
	referenceNames := []string{"BOOKS_URI", "BOOKS_ACCOUNTKEY", "BOOKS_DATABASENAME", "BOOKS_CONTAINERNAME"}
	fromReference := false
	for _, name := range referenceNames {
		if _, present := os.LookupEnv(name); present {
			fromReference = true
			names = referenceNames
			break
		}
	}
	config := cosmosConfig{
		Endpoint: os.Getenv(names[0]), Key: os.Getenv(names[1]),
		Database: os.Getenv(names[2]), Container: os.Getenv(names[3]),
		AllowHTTP: os.Getenv("COSMOS_ALLOW_HTTP") == "true",
	}
	if !fromReference {
		config.Database = env(names[2], "catbooks")
		config.Container = env(names[3], "books")
	}
	for i, value := range []string{config.Endpoint, config.Key, config.Database, config.Container} {
		if value == "" {
			return cosmosConfig{}, fmt.Errorf("%s is required", names[i])
		}
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Hostname() == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return cosmosConfig{}, fmt.Errorf("%s must be an absolute HTTP(S) URL", names[0])
	}
	if endpoint.Scheme == "http" && !config.AllowHTTP {
		return cosmosConfig{}, fmt.Errorf("%s uses HTTP; set COSMOS_ALLOW_HTTP=true only for a trusted local emulator", names[0])
	}
	return config, nil
}

func listenAddress() (string, error) {
	if address := os.Getenv("HTTP_ADDR"); address != "" {
		return address, nil
	}
	if port := os.Getenv("PORT"); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", fmt.Errorf("PORT must be an integer between 1 and 65535")
		}
		return net.JoinHostPort("", strconv.Itoa(number)), nil
	}
	return ":8080", nil
}
