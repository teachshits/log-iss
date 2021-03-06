package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type IssConfig struct {
	Deploy                    string
	ForwardDest               string
	ForwardDestConnectTimeout time.Duration
	HttpPort                  string
	Tokens                    Tokens
	EnforceSsl                bool
}

func NewIssConfig() (*IssConfig, error) {
	config := new(IssConfig)

	deploy, err := MustEnv("DEPLOY")
	if err != nil {
		return nil, err
	}
	config.Deploy = deploy

	forwardDest, err := MustEnv("FORWARD_DEST")
	if err != nil {
		return nil, err
	}
	config.ForwardDest = forwardDest

	var forwardDestConnectTimeout int
	forwardDestConnectTimeoutEnv := os.Getenv("FORWARD_DEST_CONNECT_TIMEOUT")
	if forwardDestConnectTimeoutEnv != "" {
		forwardDestConnectTimeout, err = strconv.Atoi(forwardDestConnectTimeoutEnv)
		if err != nil {
			return nil, fmt.Errorf("Unable to parse FORWARD_DEST_CONNECT_TIMEOUT: %s", err)
		}
	} else {
		forwardDestConnectTimeout = 10
	}
	config.ForwardDestConnectTimeout = time.Duration(forwardDestConnectTimeout) * time.Second

	httpPort, err := MustEnv("PORT")
	if err != nil {
		return nil, err
	}
	config.HttpPort = httpPort

	tokenMap, err := MustEnv("TOKEN_MAP")
	if err != nil {
		return nil, err
	}
	tokens, err := ParseTokenMap(tokenMap)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse tokens: %s", err)
	}
	config.Tokens = tokens

	if os.Getenv("ENFORCE_SSL") == "1" {
		config.EnforceSsl = true
	}

	return config, nil
}

func MustEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("ENV[%s] is required", key)
	}
	return value, nil
}
