package payload

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/google/uuid"
)

type Payload struct {
	ID   uuid.UUID
	Data []byte
}

const (
	UUIDSize       = 16
	MaxDataSize    = 1024
	MaxPayloadSize = 2*UUIDSize + MaxDataSize
)

func (p Payload) Bytes() []byte {
	res := append(p.ID[:], p.Data...)

	return append(res, p.ID[:]...)
}

func New(id uuid.UUID, data []byte) Payload {
	return Payload{
		ID:   id,
		Data: data,
	}
}

func splitUntilDelimiter(delim []byte) bufio.SplitFunc {
	return func(data []byte, atEof bool) (int, []byte, error) {
		index := bytes.Index(data, delim)
		if index == -1 {
			return len(data), data, nil
		}

		return index + len(delim), data[:index], nil
	}
}

func Read(r io.Reader) (*Payload, error) {
	const op = "payload.Read"

	buf := make([]byte, UUIDSize)
	if _, err := r.Read(buf); err != nil {
		return nil, fmt.Errorf("%s: could not read payload ID: %w", op, err)
	}

	id, err := uuid.FromBytes(buf)
	if err != nil {
		slog.Error("could not parse ID",
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("%s: could not parse payload ID: %w", op, err)
	}

	var data bytes.Buffer

	scanner := bufio.NewScanner(r)
	scanner.Split(splitUntilDelimiter(id[:]))
	for scanner.Scan() {
		chunk := scanner.Bytes()
		if len(chunk) == 0 {
			break
		}

		data.Write(chunk)

		if !scanner.Scan() {
			break
		}
	}

	return &Payload{ID: id, Data: data.Bytes()}, nil
}

func IsTimeout(err error) bool {
	var e net.Error
	return errors.As(err, &e) && e.Timeout()
}
