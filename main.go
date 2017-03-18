package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

const (
	WORKER_COUNT              = 5
	HEADER_DISPOSITION        = "Content-Disposition: attachment; filename="
	HEADER_ATTACHMENT_ID      = "X-Attachment-Id:"
	HEADER_MULTIPART_BOUNDARY = "Content-Type: multipart/mixed; boundary="
	HEADER_MULTIPART          = "Content-Type: multipart/mixed;"
	HEADER_BOUNDARY           = "boundary="
)

var (
	baseDir, outDir string
	channel         chan Attachment
	wg              sync.WaitGroup
)

type Attachment struct {
	Id       string
	Filename string
	Data     []byte
	file     *os.File
}

func (a *Attachment) OutputWriter() (io.Writer, error) {
	if a.file != nil {
		return a.file, nil
	}

	outName := fmt.Sprintf("%s_%s", a.Id, a.Filename)
	outPath := filepath.Join(outDir, outName)

	var err error
	a.file, err = os.Create(outPath)
	if err != nil {
		return nil, errors.New("Unable to create attachment output file")
	}

	return a.file, nil
}

func (a *Attachment) CloseWriter() error {
	if a.file == nil {
		return nil
	}
	return a.file.Close()
}

func decodeWorker() error {
	for {
		entry := <-channel

		data, err := base64.StdEncoding.DecodeString(string(entry.Data))
		if err != nil {
			return errors.New("Error while decoding attachment")
		}

		wr, err := entry.OutputWriter()
		if err != nil {
			return err
		}

		_, err = wr.Write(data)
		if err != nil {
			return errors.New("Error while writing attachment file data")
		}

		entry.CloseWriter()

		wg.Done()
	}
}

func findAttachments(fullPath string) error {
	var boundary, attachId, attachName string

	rd, err := os.Open(fullPath)
	if err != nil {
		return errors.New("Unable to open email file")
	}
	defer rd.Close()

	multipart := false
	scanner := bufio.NewScanner(rd)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, HEADER_MULTIPART_BOUNDARY) {
			boundary = strings.TrimPrefix(line, HEADER_MULTIPART_BOUNDARY)
		}

		if strings.HasPrefix(line, HEADER_MULTIPART) {
			multipart = true
		}

		if multipart && strings.HasPrefix(line, HEADER_BOUNDARY) {
			boundary = strings.TrimPrefix(line, HEADER_BOUNDARY)
		}

		if strings.HasPrefix(line, HEADER_DISPOSITION) {
			attachName = strings.TrimPrefix(line, HEADER_DISPOSITION)
			attachName = strings.Replace(attachName, "\"", "", -1)
		}

		if strings.HasPrefix(line, HEADER_ATTACHMENT_ID) {
			attachId = strings.TrimPrefix(line, HEADER_ATTACHMENT_ID)
			attachId = strings.Trim(attachId, " ")
		}

		if len(attachId) > 0 && len(attachName) > 0 && len(boundary) > 0 {
			var data []byte

			if !strings.HasPrefix(boundary, "--") {
				boundary = strings.Replace(boundary, "\"", "", -1)
				boundary = fmt.Sprintf("--%s", boundary)
			}

			scanner.Scan()
			for scanner.Scan() {
				if strings.HasPrefix(scanner.Text(), boundary) {
					break
				} else {
					data = append(data, scanner.Bytes()...)
				}
			}

			if scanner.Err() != nil {
				panic(scanner.Err())
			}

			_, err = base64.StdEncoding.DecodeString(string(data))
			if err != nil {
				panic(err)
			}

			attachment := Attachment{attachId, attachName, data, nil}
			channel <- attachment
			wg.Add(1)

			attachId = ""
			attachName = ""
		}
	}

	return nil
}

func scanFiles(fullPath string, f os.FileInfo, err error) error {
	if !f.IsDir() {
		_, relName := path.Split(fullPath)
		fmt.Printf("%s\n", relName)

		err := findAttachments(fullPath)
		if err != nil {
			fmt.Println("  Unable to find attachments")
			return nil
		}
	}

	return nil
}

func main() {
	flag.StringVar(&baseDir, "base", "", "Specifies the location of email files")
	flag.Parse()

	if len(baseDir) == 0 {
		flag.Usage()
		return
	}

	outDir = filepath.Join(baseDir, "..", "detached")

	err := os.MkdirAll(outDir, 0755)
	if err != nil {
		fmt.Printf("Unable to create output directory %s\n", outDir)
		return
	}

	channel = make(chan Attachment, WORKER_COUNT)
	for i := 0; i < WORKER_COUNT; i++ {
		go decodeWorker()
	}

	fmt.Println("Scanning for attachments...")
	filepath.Walk(baseDir, scanFiles)

	wg.Wait()
}
