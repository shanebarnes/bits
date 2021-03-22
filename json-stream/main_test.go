package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJsonDecode_StdinArray(t *testing.T) {
	assert.Nil(t, JsonDecode(bytes.NewBuffer([]byte("[{}]")), os.Stdout))
}

func TestJsonDecode_StdinEmpty(t *testing.T) {
	reader := bytes.NewBuffer(nil)
	assert.Equal(t, io.EOF, JsonDecode(reader, os.Stdout))
}

func TestJsonDecode_StdinEmptyArray(t *testing.T) {
	assert.Nil(t, JsonDecode(bytes.NewBuffer([]byte("[]")), os.Stdout))
}

func TestJsonDecode_StdinEmptyObject(t *testing.T) {
	assert.NotNil(t, io.EOF, JsonDecode(bytes.NewBuffer([]byte("{}")), os.Stdout))
}

func TestJsonDecode_StdinMalformedArray(t *testing.T) {
	assert.NotNil(t, JsonDecode(bytes.NewBuffer([]byte("[}")), os.Stdout))
}
