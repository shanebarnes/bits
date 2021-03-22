package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/dustin/go-humanize"
)

type node struct {
	Name                      string   `json:"name"`
	CapacityInBytes           int64    `json:"capacity_bytes"`
	Objects                   []object `json:"objects"`
	freeSpaceInBytes          int64
	smallestObjectIndex       int
	smallestObjectSizeInBytes int64
}

type object struct {
	Name        string `json:"name"`
	SizeInBytes int64  `json:"size_bytes"`
}

func JsonDecode(reader io.Reader, writer io.Writer) error {
	decoder := json.NewDecoder(reader)

	// Read first token (e.g., '[')
	if _, err := jsonDecodeNextToken(decoder); err != nil {
		return err
	}

	sysState := []node{}
	for decoder.More() {
		node := node{}
		err := decoder.Decode(&node)
		switch err {
		case io.EOF, nil:
			sysState = append(sysState, node)
		default:
			return err
		}
	}

	// Read last token (e.g., ']')
	if _, err := jsonDecodeNextToken(decoder); err != nil {
		return err
	}

	for {
		maxFreeSpaceInBytes := int64(-1)
		minFreeSpaceInBytes := int64(-1)
		maxFreeSpaceNodeIndex := -1
		minFreeSpaceNodeIndex := -1

		for i, n := range sysState {
			n.freeSpaceInBytes = n.CapacityInBytes
			if len(n.Objects) > 0 {
				n.smallestObjectIndex = 0
				n.smallestObjectSizeInBytes = n.Objects[0].SizeInBytes
			}
			for j, o := range n.Objects {
				n.freeSpaceInBytes -= o.SizeInBytes
				if o.SizeInBytes < n.smallestObjectSizeInBytes {
					n.smallestObjectIndex = j
					n.smallestObjectSizeInBytes = o.SizeInBytes
				}
			}
			if n.freeSpaceInBytes > maxFreeSpaceInBytes {
				maxFreeSpaceInBytes = n.freeSpaceInBytes
				maxFreeSpaceNodeIndex = i
			}
			if n.freeSpaceInBytes < minFreeSpaceInBytes || minFreeSpaceInBytes == -1 {
				minFreeSpaceInBytes = n.freeSpaceInBytes
				minFreeSpaceNodeIndex = i
			}
		}

		if (maxFreeSpaceInBytes - minFreeSpaceInBytes) > humanize.TiByte {
			minNode := &sysState[minFreeSpaceNodeIndex]
			minObj := minNode.Objects[minNode.smallestObjectIndex]
			//fmt.Fprintf(writer, "Min node: %s, min object: %s\n", minNode.Name, minObj.Name)
			fmt.Fprintf(os.Stdout, "%s %s %s\n", sysState[minFreeSpaceNodeIndex].Name, sysState[maxFreeSpaceNodeIndex].Name, minObj.Name)
			maxNode := &sysState[maxFreeSpaceNodeIndex]
			maxNode.Objects = append(maxNode.Objects, minObj)
			minNode.Objects = append(minNode.Objects[:minNode.smallestObjectIndex], minNode.Objects[minNode.smallestObjectIndex+1:]...)
			//fmt.Fprintf(writer, "Min node objects: %v, max node objects: %v\n", minNode.Objects, maxNode.Objects)
		} else {
			fmt.Fprintf(writer, "Final free space diff: %s\n", humanize.Comma(maxFreeSpaceInBytes-minFreeSpaceInBytes))
			fmt.Fprintf(writer, "Final system state: %v\n", sysState)
			break
		}
	}

	return nil
}

func jsonDecodeNextToken(decoder *json.Decoder) (string, error) {
	if tok, err := decoder.Token(); err != nil {
		return "", err
	} else if delim, ok := tok.(json.Delim); !ok {
		return "", fmt.Errorf("Invalid JSON token '%s'", delim.String())
	} else {
		return delim.String(), nil
	}
}

func main() {
	err := JsonDecode(os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}
