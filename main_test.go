package main

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"
)

var (
	base64Bytes  []byte
	base64String string
)

func init() {
	var err error

	base64Bytes, err = ioutil.ReadFile("testdata/base64.txt")
	if err != nil {
		panic(err)
	}

	base64String = string(base64Bytes)
}

func TestEmailFileNotFound(t *testing.T) {
	err := findAttachments("no-file")
	if err.Error() != "Unable to open email file" {
		t.Fail()
	}
}

func TestOneAttachment(t *testing.T) {
	channel = make(chan Attachment, 1)

	err := findAttachments("testdata/one-attachment.txt")
	if err != nil {
		t.Fatal(err)
	}

	entry := <-channel
	if entry.Filename != "uct.zip" {
		t.Fatal("Incorrect attachment name")
	}

	if entry.Id != "f_g1jcgjo20" {
		t.Fatal("Incorrect attachment id")
	}

	if string(entry.Data) != base64String {
		t.Fatal("Incorrect attachment data")
	}
}

func TestDecoder(t *testing.T) {
	channel = make(chan Attachment, 1)
	wg = sync.WaitGroup{}

	file, err := ioutil.TempFile(os.TempDir(), "dec_")
	if err != nil {
		t.Fail()
	}

	attachment := Attachment{"abc123", "name.zip", base64Bytes, file}
	channel <- attachment

	wg.Add(1)
	go decodeWorker()
	wg.Wait()

	data, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fail()
	}

	if len(data) != 174 {
		t.Fatal("Incorrectly decoded")
	}
}
