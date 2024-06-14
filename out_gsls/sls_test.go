package main

import (
	"fmt"
	"testing"
)

func TestSLS(t *testing.T) {
	client, err := NewSLS("/Users/qi/GolandProjects/fluent-bit-go-plugins/config.yaml")
	if err != nil {
		panic(err)
	}
	fmt.Println(client)
}
