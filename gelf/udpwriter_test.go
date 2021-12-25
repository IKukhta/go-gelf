// Copyright 2012 SocialCode. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package gelf

import (
	"compress/flate"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func TestNewUDPWriter(t *testing.T) {
	w, err := NewUDPWriter("")
	if err == nil || w != nil {
		t.Errorf("New didn't fail")
		return
	}
}

func sendAndRecvRaw(t *testing.T, msgData string, compress CompressType) ([]byte, error) {
	t.Helper()
	var err error
	var r *Reader
	var w *UDPWriter

	if r, err = NewReader("127.0.0.1:0"); err != nil {
		return nil, fmt.Errorf("NewReader: %s", err)
	}
	defer r.Close()

	if w, err = NewUDPWriter(r.Addr()); err != nil {
		return nil, fmt.Errorf("NewUDPWriter: %s", err)
	}
	w.CompressionType = compress

	if _, err = w.Write([]byte(msgData)); err != nil {
		return nil, fmt.Errorf("w.Write: %s", err)
	}

	w.Close()
	b := make([]byte, 512)
	i, err := r.Read(b)
	if err != nil {
		return nil, fmt.Errorf("r.Read: %s", err)
	}

	return b[:i], nil
}

func sendAndRecv(t *testing.T, msgData string, compress CompressType) (*Message, error) {
	t.Helper()
	var err error
	var r *Reader
	var w *UDPWriter

	if r, err = NewReader("127.0.0.1:0"); err != nil {
		return nil, fmt.Errorf("NewReader: %s", err)
	}
	defer r.Close()

	if w, err = NewUDPWriter(r.Addr()); err != nil {
		return nil, fmt.Errorf("NewUDPWriter: %s", err)
	}
	w.CompressionType = compress

	if _, err = w.Write([]byte(msgData)); err != nil {
		return nil, fmt.Errorf("w.Write: %s", err)
	}

	w.Close()
	return r.ReadMessage()
}

func sendAndRecvMsg(t *testing.T, msg *Message, compress CompressType) (*Message, error) {
	t.Helper()
	var err error
	var r *Reader
	var w *UDPWriter

	if r, err = NewReader("127.0.0.1:0"); err != nil {
		return nil, fmt.Errorf("NewReader: %s", err)
	}
	defer r.Close()

	if w, err = NewUDPWriter(r.Addr()); err != nil {
		return nil, fmt.Errorf("NewUDPWriter: %s", err)
	}
	w.CompressionType = compress

	if err = w.WriteMessage(msg); err != nil {
		return nil, fmt.Errorf("w.Write: %s", err)
	}

	w.Close()
	return r.ReadMessage()
}

// tests read raw single-message (non-chunked) messages that are split over
// multiple lines
func TestReadRawSmallMultiLine(t *testing.T) {
	for _, i := range []CompressType{CompressGzip, CompressZlib, CompressNone} {
		msgData := "awesomesauce\nbananas"

		raw, err := sendAndRecvRaw(t, msgData, i)
		if err != nil {
			t.Errorf("sendAndRecv: %s", err)
			return
		}

		if string(raw) != msgData {
			t.Errorf("raw: expected %q, got %q", msgData, string(raw))
			return
		}
	}
}

// tests single-message (non-chunked) messages that are split over
// multiple lines
func TestWriteSmallMultiLine(t *testing.T) {
	for _, i := range []CompressType{CompressGzip, CompressZlib, CompressNone} {
		msgData := "awesomesauce\nbananas"

		msg, err := sendAndRecv(t, msgData, i)
		if err != nil {
			t.Errorf("sendAndRecv: %s", err)
			return
		}

		if msg.Short != "awesomesauce" {
			t.Errorf("msg.Short: expected %s, got %s", "awesomesauce", msg.Full)
			return
		}

		if msg.Full != msgData {
			t.Errorf("msg.Full: expected %s, got %s", msgData, msg.Full)
			return
		}
	}
}

// tests single-message (non-chunked) messages that are a single line long
func TestWriteSmallOneLine(t *testing.T) {
	msgData := "some awesome thing\n"
	msgDataTrunc := msgData[:len(msgData)-1]

	msg, err := sendAndRecv(t, msgData, CompressGzip)
	if err != nil {
		t.Errorf("sendAndRecv: %s", err)
		return
	}

	// we should remove the trailing newline
	if msg.Short != msgDataTrunc {
		t.Errorf("msg.Short: expected %s, got %s",
			msgDataTrunc, msg.Short)
		return
	}

	if msg.Full != "" {
		t.Errorf("msg.Full: expected %s, got %s", msgData, msg.Full)
		return
	}

	fileExpected := "/go-gelf/gelf/udpwriter_test.go"
	if !strings.HasSuffix(msg.Extra["_file"].(string), fileExpected) {
		t.Errorf("msg.File: expected %s, got %s", fileExpected,
			msg.Extra["_file"].(string))
		return
	}

	if len(msg.Extra) != 2 {
		t.Errorf("extra fields in %v (expect only file and line)", msg.Extra)
		return
	}
}

func TestGetCaller(t *testing.T) {
	file, line := getCallerIgnoringLogMulti(1000)
	if line != 0 || file != "???" {
		t.Errorf("didn't fail 1 %s %d", file, line)
		return
	}

	file, _ = getCaller(0)
	if !strings.HasSuffix(file, "/gelf/udpwriter_test.go") {
		t.Errorf("not udpwriter_test.go 1? %s", file)
	}

	file, _ = getCallerIgnoringLogMulti(0)
	if !strings.HasSuffix(file, "/gelf/udpwriter_test.go") {
		t.Errorf("not udpwriter_test.go 2? %s", file)
	}
}

// tests single-message (chunked) messages
func TestWriteBigChunked(t *testing.T) {
	randData := make([]byte, 4096)
	if _, err := rand.Read(randData); err != nil {
		t.Errorf("cannot get random data: %s", err)
		return
	}
	msgData := "awesomesauce\n" + base64.StdEncoding.EncodeToString(randData)

	for _, i := range []CompressType{CompressGzip, CompressZlib} {
		msg, err := sendAndRecv(t, msgData, i)
		if err != nil {
			t.Errorf("sendAndRecv: %s", err)
			return
		}

		if msg.Short != "awesomesauce" {
			t.Errorf("msg.Short: expected %s, got %s", msgData, msg.Full)
			return
		}

		if msg.Full != msgData {
			t.Errorf("msg.Full: expected %s, got %s", msgData, msg.Full)
			return
		}
	}
}

// tests messages with extra data
func TestExtraData(t *testing.T) {

	// time.Now().Unix() seems fine, UnixNano() won't roundtrip
	// through string -> float64 -> int64
	extra := map[string]interface{}{
		"_a":    10 * time.Now().Unix(),
		"C":     9,
		"_file": "udpwriter_test.go",
		"_line": 186,
	}

	short := "quick"
	full := short + "\nwith more detail"
	m := Message{
		Version:  "1.0",
		Host:     "fake-host",
		Short:    string(short),
		Full:     string(full),
		TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
		Level:    6, // info
		Facility: "udpwriter_test",
		Extra:    extra,
		RawExtra: []byte(`{"woo": "hoo"}`),
	}

	for _, i := range []CompressType{CompressGzip, CompressZlib} {
		msg, err := sendAndRecvMsg(t, &m, i)
		if err != nil {
			t.Errorf("sendAndRecv: %s", err)
			return
		}

		if msg.Short != short {
			t.Errorf("msg.Short: expected %s, got %s", short, msg.Full)
			return
		}

		if msg.Full != full {
			t.Errorf("msg.Full: expected %s, got %s", full, msg.Full)
			return
		}

		if len(msg.Extra) != 3 {
			t.Errorf("extra extra fields in %v", msg.Extra)
			return
		}

		if int64(msg.Extra["_a"].(float64)) != extra["_a"].(int64) {
			t.Errorf("_a didn't roundtrip (%v != %v)", int64(msg.Extra["_a"].(float64)), extra["_a"].(int64))
			return
		}

		if string(msg.Extra["_file"].(string)) != extra["_file"] {
			t.Errorf("_file didn't roundtrip (%v != %v)", msg.Extra["_file"].(string), extra["_file"].(string))
			return
		}

		if int(msg.Extra["_line"].(float64)) != extra["_line"].(int) {
			t.Errorf("_line didn't roundtrip (%v != %v)", int(msg.Extra["_line"].(float64)), extra["_line"].(int))
			return
		}
	}
}

func BenchmarkWriteBestSpeed(b *testing.B) {
	r, err := NewReader("127.0.0.1:0")
	if err != nil {
		b.Fatalf("NewReader: %s", err)
	}
	defer r.Close()

	go io.Copy(ioutil.Discard, r)
	w, err := NewUDPWriter(r.Addr())
	if err != nil {
		b.Fatalf("NewUDPWriter: %s", err)
	}
	w.CompressionLevel = flate.BestSpeed
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.WriteMessage(&Message{
			Version:  "1.1",
			Host:     w.hostname,
			Short:    "short message",
			Full:     "full message",
			TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
			Level:    6, // info
			Facility: w.Facility,
			Extra:    map[string]interface{}{"_file": "1234", "_line": "3456"},
		})
	}
}

func BenchmarkWriteNoCompression(b *testing.B) {
	r, err := NewReader("127.0.0.1:0")
	if err != nil {
		b.Fatalf("NewReader: %s", err)
	}
	defer r.Close()

	go io.Copy(ioutil.Discard, r)
	w, err := NewUDPWriter(r.Addr())
	if err != nil {
		b.Fatalf("NewUDPWriter: %s", err)
	}
	w.CompressionLevel = flate.NoCompression
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.WriteMessage(&Message{
			Version:  "1.1",
			Host:     w.hostname,
			Short:    "short message",
			Full:     "full message",
			TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
			Level:    6, // info
			Facility: w.Facility,
			Extra:    map[string]interface{}{"_file": "1234", "_line": "3456"},
		})
	}
}

func BenchmarkWriteDisableCompressionCompletely(b *testing.B) {
	r, err := NewReader("127.0.0.1:0")
	if err != nil {
		b.Fatalf("NewReader: %s", err)
	}
	defer r.Close()

	go io.Copy(ioutil.Discard, r)
	w, err := NewUDPWriter(r.Addr())
	if err != nil {
		b.Fatalf("NewUDPWriter: %s", err)
	}
	w.CompressionType = CompressNone
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.WriteMessage(&Message{
			Version:  "1.1",
			Host:     w.hostname,
			Short:    "short message",
			Full:     "full message",
			TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
			Level:    6, // info
			Facility: w.Facility,
			Extra:    map[string]interface{}{"_file": "1234", "_line": "3456"},
		})
	}
}

func BenchmarkWriteDisableCompressionAndPreencodeExtra(b *testing.B) {
	r, err := NewReader("127.0.0.1:0")
	if err != nil {
		b.Fatalf("NewReader: %s", err)
	}
	defer r.Close()

	go io.Copy(ioutil.Discard, r)
	w, err := NewUDPWriter(r.Addr())
	if err != nil {
		b.Fatalf("NewUDPWriter: %s", err)
	}
	w.CompressionType = CompressNone
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.WriteMessage(&Message{
			Version:  "1.1",
			Host:     w.hostname,
			Short:    "short message",
			Full:     "full message",
			TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
			Level:    6, // info
			Facility: w.Facility,
			RawExtra: json.RawMessage(`{"_file":"1234","_line": "3456"}`),
		})
	}
}
