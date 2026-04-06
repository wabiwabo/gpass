// Package cbor provides minimal CBOR (RFC 8949) encoding for
// compact binary data exchange. Supports major types 0-5 for
// integers, byte strings, text strings, arrays, maps, and tags.
package cbor

import (
	"encoding/binary"
	"errors"
	"math"
)

// Major types per RFC 8949.
const (
	MajorUint     byte = 0 << 5
	MajorNegInt   byte = 1 << 5
	MajorBytes    byte = 2 << 5
	MajorText     byte = 3 << 5
	MajorArray    byte = 4 << 5
	MajorMap      byte = 5 << 5
	MajorTag      byte = 6 << 5
	MajorSimple   byte = 7 << 5
)

// Simple values.
const (
	SimpleTrue  byte = 0xf5
	SimpleFalse byte = 0xf4
	SimpleNull  byte = 0xf6
)

// Encoder builds CBOR output.
type Encoder struct {
	buf []byte
}

// NewEncoder creates an encoder with initial capacity.
func NewEncoder(capacity int) *Encoder {
	return &Encoder{buf: make([]byte, 0, capacity)}
}

// Bytes returns the encoded CBOR data.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// Reset clears the encoder for reuse.
func (e *Encoder) Reset() {
	e.buf = e.buf[:0]
}

func (e *Encoder) writeHead(major byte, val uint64) {
	switch {
	case val < 24:
		e.buf = append(e.buf, major|byte(val))
	case val <= math.MaxUint8:
		e.buf = append(e.buf, major|24, byte(val))
	case val <= math.MaxUint16:
		e.buf = append(e.buf, major|25, 0, 0)
		binary.BigEndian.PutUint16(e.buf[len(e.buf)-2:], uint16(val))
	case val <= math.MaxUint32:
		e.buf = append(e.buf, major|26, 0, 0, 0, 0)
		binary.BigEndian.PutUint32(e.buf[len(e.buf)-4:], uint32(val))
	default:
		e.buf = append(e.buf, major|27, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(e.buf[len(e.buf)-8:], val)
	}
}

// EncodeUint encodes an unsigned integer.
func (e *Encoder) EncodeUint(v uint64) {
	e.writeHead(MajorUint, v)
}

// EncodeInt encodes a signed integer.
func (e *Encoder) EncodeInt(v int64) {
	if v >= 0 {
		e.writeHead(MajorUint, uint64(v))
	} else {
		e.writeHead(MajorNegInt, uint64(-1-v))
	}
}

// EncodeBytes encodes a byte string.
func (e *Encoder) EncodeBytes(data []byte) {
	e.writeHead(MajorBytes, uint64(len(data)))
	e.buf = append(e.buf, data...)
}

// EncodeText encodes a text string.
func (e *Encoder) EncodeText(s string) {
	e.writeHead(MajorText, uint64(len(s)))
	e.buf = append(e.buf, s...)
}

// EncodeArrayHead encodes array length header. Items follow.
func (e *Encoder) EncodeArrayHead(length int) {
	e.writeHead(MajorArray, uint64(length))
}

// EncodeMapHead encodes map length header. Key-value pairs follow.
func (e *Encoder) EncodeMapHead(length int) {
	e.writeHead(MajorMap, uint64(length))
}

// EncodeBool encodes a boolean.
func (e *Encoder) EncodeBool(v bool) {
	if v {
		e.buf = append(e.buf, SimpleTrue)
	} else {
		e.buf = append(e.buf, SimpleFalse)
	}
}

// EncodeNull encodes a null value.
func (e *Encoder) EncodeNull() {
	e.buf = append(e.buf, SimpleNull)
}

// EncodeTag encodes a CBOR tag number.
func (e *Encoder) EncodeTag(tag uint64) {
	e.writeHead(MajorTag, tag)
}

// Decoder reads CBOR data.
type Decoder struct {
	data []byte
	pos  int
}

// NewDecoder creates a decoder.
func NewDecoder(data []byte) *Decoder {
	return &Decoder{data: data}
}

var (
	ErrUnexpectedEnd = errors.New("cbor: unexpected end of data")
	ErrInvalidType   = errors.New("cbor: invalid major type")
)

func (d *Decoder) readHead() (byte, uint64, error) {
	if d.pos >= len(d.data) {
		return 0, 0, ErrUnexpectedEnd
	}
	b := d.data[d.pos]
	d.pos++
	major := b & 0xe0
	info := b & 0x1f

	switch {
	case info < 24:
		return major, uint64(info), nil
	case info == 24:
		if d.pos >= len(d.data) {
			return 0, 0, ErrUnexpectedEnd
		}
		v := d.data[d.pos]
		d.pos++
		return major, uint64(v), nil
	case info == 25:
		if d.pos+2 > len(d.data) {
			return 0, 0, ErrUnexpectedEnd
		}
		v := binary.BigEndian.Uint16(d.data[d.pos:])
		d.pos += 2
		return major, uint64(v), nil
	case info == 26:
		if d.pos+4 > len(d.data) {
			return 0, 0, ErrUnexpectedEnd
		}
		v := binary.BigEndian.Uint32(d.data[d.pos:])
		d.pos += 4
		return major, uint64(v), nil
	case info == 27:
		if d.pos+8 > len(d.data) {
			return 0, 0, ErrUnexpectedEnd
		}
		v := binary.BigEndian.Uint64(d.data[d.pos:])
		d.pos += 8
		return major, uint64(v), nil
	}
	return 0, 0, ErrInvalidType
}

// DecodeUint decodes an unsigned integer.
func (d *Decoder) DecodeUint() (uint64, error) {
	major, val, err := d.readHead()
	if err != nil {
		return 0, err
	}
	if major != MajorUint {
		return 0, ErrInvalidType
	}
	return val, nil
}

// DecodeInt decodes a signed integer.
func (d *Decoder) DecodeInt() (int64, error) {
	major, val, err := d.readHead()
	if err != nil {
		return 0, err
	}
	switch major {
	case MajorUint:
		return int64(val), nil
	case MajorNegInt:
		return -1 - int64(val), nil
	}
	return 0, ErrInvalidType
}

// DecodeBytes decodes a byte string.
func (d *Decoder) DecodeBytes() ([]byte, error) {
	major, length, err := d.readHead()
	if err != nil {
		return nil, err
	}
	if major != MajorBytes {
		return nil, ErrInvalidType
	}
	if d.pos+int(length) > len(d.data) {
		return nil, ErrUnexpectedEnd
	}
	result := make([]byte, length)
	copy(result, d.data[d.pos:d.pos+int(length)])
	d.pos += int(length)
	return result, nil
}

// DecodeText decodes a text string.
func (d *Decoder) DecodeText() (string, error) {
	major, length, err := d.readHead()
	if err != nil {
		return "", err
	}
	if major != MajorText {
		return "", ErrInvalidType
	}
	if d.pos+int(length) > len(d.data) {
		return "", ErrUnexpectedEnd
	}
	s := string(d.data[d.pos : d.pos+int(length)])
	d.pos += int(length)
	return s, nil
}

// DecodeArrayHead decodes array head and returns length.
func (d *Decoder) DecodeArrayHead() (int, error) {
	major, val, err := d.readHead()
	if err != nil {
		return 0, err
	}
	if major != MajorArray {
		return 0, ErrInvalidType
	}
	return int(val), nil
}

// DecodeMapHead decodes map head and returns the number of pairs.
func (d *Decoder) DecodeMapHead() (int, error) {
	major, val, err := d.readHead()
	if err != nil {
		return 0, err
	}
	if major != MajorMap {
		return 0, ErrInvalidType
	}
	return int(val), nil
}

// DecodeBool decodes a boolean.
func (d *Decoder) DecodeBool() (bool, error) {
	if d.pos >= len(d.data) {
		return false, ErrUnexpectedEnd
	}
	b := d.data[d.pos]
	d.pos++
	switch b {
	case SimpleTrue:
		return true, nil
	case SimpleFalse:
		return false, nil
	}
	return false, ErrInvalidType
}

// Remaining returns unprocessed bytes count.
func (d *Decoder) Remaining() int {
	return len(d.data) - d.pos
}
