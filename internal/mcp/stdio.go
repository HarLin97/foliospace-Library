package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func (s *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	reader := bufio.NewReader(input)
	for {
		body, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			resp := fail(nil, -32700, "parse error: "+err.Error())
			if err := writeMessage(output, resp); err != nil {
				return err
			}
			continue
		}

		resp := s.Handle(ctx, req)
		if resp.JSONRPC == "" && resp.ID == nil {
			continue
		}
		if err := writeMessage(output, resp); err != nil {
			return err
		}
	}
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	return body, err
}

func writeMessage(output io.Writer, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	header := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
	_, err = io.Copy(output, bytes.NewReader(append(header, body...)))
	return err
}
