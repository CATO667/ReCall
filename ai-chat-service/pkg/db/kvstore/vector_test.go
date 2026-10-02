package kvstore

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestReadReplyArray(t *testing.T) {
	raw := "*4\r\n$1\r\na\r\n$1\r\n0\r\n$1\r\nb\r\n$1\r\n1\r\n"
	r := bufio.NewReader(bytes.NewReader([]byte(raw)))
	reply, err := readReply(r)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Type != '*' || len(reply.Array) != 4 {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if reply.Array[0].BulkString() != "a" || reply.Array[1].BulkString() != "0" {
		t.Fatalf("unexpected first pair: %s %s", reply.Array[0].BulkString(), reply.Array[1].BulkString())
	}
}

func TestEncodeVector(t *testing.T) {
	s := encodeVector([]float32{1, 0, 0.5})
	if !strings.Contains(s, "1") || !strings.Contains(s, "0.5") {
		t.Fatalf("unexpected vector encoding: %s", s)
	}
}

func TestCosineSimilarity(t *testing.T) {
	if CosineSimilarity(0) != 1 {
		t.Fatalf("expected similarity 1 for distance 0")
	}
	if CosineSimilarity(0.5) != 0.5 {
		t.Fatalf("expected similarity 0.5")
	}
}
