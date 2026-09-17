package bedrock

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

// This file decodes the application/vnd.amazon.eventstream binary framing.
//
// Wire format (all integers big-endian):
//
//	[total length   u32]
//	[headers length  u32]
//	[prelude CRC32   u32]   ← CRC over the first 8 bytes
//	[headers         headers_length]
//	[payload         total_length - headers_length - 16]
//	[message CRC32   u32]   ← CRC over everything except the trailing 4 bytes

const (
	esPreludeLen      = 8
	esPreludeCRCLen   = 4
	esMessageCRCLen   = 4
	esHeaderBlockOff  = esPreludeLen + esPreludeCRCLen
	esMinMessageLen   = esHeaderBlockOff + esMessageCRCLen
)

// eventStreamMessage is one decoded eventstream frame.
type eventStreamMessage struct {
	Headers map[string]string
	Payload []byte
}

// decodeMessage decodes a single, fully buffered eventstream message.
// Returns an error if framing is malformed or either CRC mismatches.
func decodeMessage(frame []byte) (eventStreamMessage, error) {
	if len(frame) < esMinMessageLen {
		return eventStreamMessage{}, fmt.Errorf("eventstream: frame too short (%d bytes)", len(frame))
	}
	total := int(binary.BigEndian.Uint32(frame[0:4]))
	if total != len(frame) {
		return eventStreamMessage{}, fmt.Errorf("eventstream: framed length %d != buffer %d", total, len(frame))
	}
	headersLen := int(binary.BigEndian.Uint32(frame[4:8]))
	preludeCRC := binary.BigEndian.Uint32(frame[8:12])
	computedPreludeCRC := crc32.ChecksumIEEE(frame[0:esPreludeLen])
	if computedPreludeCRC != preludeCRC {
		return eventStreamMessage{}, fmt.Errorf("eventstream: prelude CRC mismatch")
	}
	msgCRC := binary.BigEndian.Uint32(frame[total-esMessageCRCLen : total])
	computedMsgCRC := crc32.ChecksumIEEE(frame[0 : total-esMessageCRCLen])
	if computedMsgCRC != msgCRC {
		return eventStreamMessage{}, fmt.Errorf("eventstream: message CRC mismatch")
	}

	headersBytes := frame[esHeaderBlockOff : esHeaderBlockOff+headersLen]
	payload := frame[esHeaderBlockOff+headersLen : total-esMessageCRCLen]
	headers, err := parseHeaders(headersBytes)
	if err != nil {
		return eventStreamMessage{}, err
	}
	return eventStreamMessage{Headers: headers, Payload: payload}, nil
}

// decodeEventStream reads all messages from a byte buffer. Handles messages
// that span the buffer in any alignment.
func decodeEventStream(data []byte) ([]eventStreamMessage, error) {
	var messages []eventStreamMessage
	offset := 0
	for offset < len(data) {
		remaining := data[offset:]
		if len(remaining) < 4 {
			return nil, fmt.Errorf("eventstream: truncated message at offset %d", offset)
		}
		total := int(binary.BigEndian.Uint32(remaining[0:4]))
		if total < esMinMessageLen {
			return nil, fmt.Errorf("eventstream: total length %d below minimum", total)
		}
		if len(remaining) < total {
			return nil, fmt.Errorf("eventstream: incomplete message at offset %d", offset)
		}
		msg, err := decodeMessage(remaining[:total])
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
		offset += total
	}
	return messages, nil
}

// parseHeaders decodes the header section of an eventstream message.
// Each header is [name_len u8][name][value_type u8][value...].
// We surface all types as strings for ease of consumption — Bedrock only
// uses string-valued headers in practice.
func parseHeaders(buf []byte) (map[string]string, error) {
	out := make(map[string]string)
	p := 0
	for p < len(buf) {
		if p+1 > len(buf) {
			return nil, fmt.Errorf("eventstream: truncated header name length")
		}
		nameLen := int(buf[p])
		p++
		if p+nameLen > len(buf) {
			return nil, fmt.Errorf("eventstream: truncated header name")
		}
		name := string(buf[p : p+nameLen])
		p += nameLen
		if p+1 > len(buf) {
			return nil, fmt.Errorf("eventstream: truncated header value type")
		}
		vType := buf[p]
		p++
		switch vType {
		case 0: // bool true
			out[name] = "true"
		case 1: // bool false
			out[name] = "false"
		case 2: // byte
			if p+1 > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated byte header")
			}
			out[name] = fmt.Sprintf("%d", int8(buf[p]))
			p++
		case 3: // short
			if p+2 > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated short header")
			}
			out[name] = fmt.Sprintf("%d", int16(binary.BigEndian.Uint16(buf[p:p+2])))
			p += 2
		case 4: // integer
			if p+4 > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated integer header")
			}
			out[name] = fmt.Sprintf("%d", int32(binary.BigEndian.Uint32(buf[p:p+4])))
			p += 4
		case 5: // long
			if p+8 > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated long header")
			}
			out[name] = fmt.Sprintf("%d", int64(binary.BigEndian.Uint64(buf[p:p+8])))
			p += 8
		case 6: // byte array
			if p+2 > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated byte-array header length")
			}
			ln := int(binary.BigEndian.Uint16(buf[p : p+2]))
			p += 2
			if p+ln > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated byte-array header data")
			}
			out[name] = string(buf[p : p+ln])
			p += ln
		case 7: // string
			if p+2 > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated string header length")
			}
			ln := int(binary.BigEndian.Uint16(buf[p : p+2]))
			p += 2
			if p+ln > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated string header data")
			}
			out[name] = string(buf[p : p+ln])
			p += ln
		case 8: // timestamp (ms since epoch as i64)
			if p+8 > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated timestamp header")
			}
			out[name] = fmt.Sprintf("%d", int64(binary.BigEndian.Uint64(buf[p:p+8])))
			p += 8
		case 9: // uuid
			if p+16 > len(buf) {
				return nil, fmt.Errorf("eventstream: truncated uuid header")
			}
			out[name] = fmt.Sprintf("%x", buf[p:p+16])
			p += 16
		default:
			return nil, fmt.Errorf("eventstream: unknown header value type %d", vType)
		}
	}
	return out, nil
}
