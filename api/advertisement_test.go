package api

import (
	"fmt"
	"testing"
)

func TestAdvertisementParse(t *testing.T) {
	var ad1 Advertisement =  "1 in: opus; PCMU; PCMA;\n" + "2 out: opus; PCMU; PCMA;\n"
	ad1Info, err := ad1.Parse()
	if err != nil {
		t.Fatalf("error parsing ad %v", err)
	}
	fmt.Printf("parsed %v", ad1Info)
}
