package kvstore

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func encodeCommand(args ...string) []byte {
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(args)))
	b.WriteString("\r\n")
	for _, arg := range args {
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(arg)))
		b.WriteString("\r\n")
		b.WriteString(arg)
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

type Reply struct {
	Type    byte
	Status  string
	Error   string
	Integer int64
	Bulk    []byte
	IsNil   bool
	Array   []Reply
}

func readReply(r *bufio.Reader) (*Reply, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 1 {
		return nil, io.ErrUnexpectedEOF
	}
	switch line[0] {
	case '+':
		return &Reply{Type: '+', Status: strings.TrimSuffix(line[1:], "\r\n")}, nil
	case '-':
		return &Reply{Type: '-', Error: strings.TrimSuffix(line[1:], "\r\n")}, nil
	case ':':
		n, err := strconv.ParseInt(strings.TrimSpace(line[1:]), 10, 64)
		if err != nil {
			return nil, err
		}
		return &Reply{Type: ':', Integer: n}, nil
	case '$':
		n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return &Reply{Type: '$', IsNil: true}, nil
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return &Reply{Type: '$', Bulk: buf[:n]}, nil
	case '*':
		n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, fmt.Errorf("invalid RESP array length: %d", n)
		}
		items := make([]Reply, 0, n)
		for i := 0; i < n; i++ {
			item, err := readReply(r)
			if err != nil {
				return nil, err
			}
			items = append(items, *item)
		}
		return &Reply{Type: '*', Array: items}, nil
	default:
		return nil, fmt.Errorf("unknown RESP type: %q", line[0])
	}
}

func (r *Reply) BulkString() string {
	if r == nil || r.IsNil {
		return ""
	}
	return string(r.Bulk)
}
