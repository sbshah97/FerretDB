// Copyright 2021 FerretDB Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package driver provides low-level wire protocol driver for testing.
package driver

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"

	"github.com/FerretDB/FerretDB/internal/util/lazyerrors"
	"github.com/FerretDB/FerretDB/internal/wire"
)

// Conn represents a single connection.
//
// It is not safe for concurrent use.
//
// Logger is used only with debug level.
type Conn struct {
	c net.Conn
	r *bufio.Reader
	w *bufio.Writer
	l *slog.Logger
}

// Connect creates a new connection for the given MongoDB URI and logger.
//
// Context can be used to cancel the connection attempt.
// Canceling the context after the connection is established has no effect.
func Connect(ctx context.Context, uri string, l *slog.Logger) (*Conn, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, lazyerrors.Error(err)
	}

	if u.Scheme != "mongodb" {
		return nil, lazyerrors.Errorf("invalid scheme %q", u.Scheme)
	}

	if u.Opaque != "" {
		return nil, lazyerrors.Errorf("invalid URI %q", uri)
	}

	if _, _, err = net.SplitHostPort(u.Host); err != nil {
		return nil, lazyerrors.Error(err)
	}

	if u.User != nil {
		return nil, lazyerrors.Errorf("authentication is not supported")
	}

	if u.Path != "/" {
		return nil, lazyerrors.Errorf("path %q is not supported", u.Path)
	}

	for k := range u.Query() {
		switch k {
		default:
			return nil, lazyerrors.Errorf("query parameter %q is not supported", k)
		}
	}

	l.DebugContext(ctx, "Connecting...", slog.String("uri", uri))

	d := net.Dialer{}

	c, err := d.DialContext(ctx, "tcp", u.Host)
	if err != nil {
		return nil, lazyerrors.Error(err)
	}

	return &Conn{
		c: c,
		r: bufio.NewReader(c),
		w: bufio.NewWriter(c),
		l: l,
	}, nil
}

// Close closes the connection.
func (c *Conn) Close() error {
	var err error

	c.l.Debug("Closing...")

	if e := c.w.Flush(); e != nil {
		err = lazyerrors.Error(e)
	}

	if e := c.c.Close(); e != nil && err == nil {
		err = lazyerrors.Error(e)
	}

	return err
}

// Read reads the next message from the connection.
func (c *Conn) Read() (*wire.MsgHeader, wire.MsgBody, error) {
	header, body, err := wire.ReadMessage(c.r)
	if err != nil {
		return nil, nil, lazyerrors.Error(err)
	}

	c.l.Debug(
		fmt.Sprintf("<<<\n%s", body.String()),
		slog.Int("length", int(header.MessageLength)),
		slog.Int("id", int(header.ResponseTo)),
		slog.Int("response_to", int(header.ResponseTo)),
		slog.String("opcode", header.OpCode.String()),
	)

	return header, body, nil
}

// Write writes the given message to the connection.
func (c *Conn) Write(header *wire.MsgHeader, body wire.MsgBody) error {
	c.l.Debug(
		fmt.Sprintf(">>>\n%s", body.String()),
		slog.Int("length", int(header.MessageLength)),
		slog.Int("id", int(header.ResponseTo)),
		slog.Int("response_to", int(header.ResponseTo)),
		slog.String("opcode", header.OpCode.String()),
	)

	if err := wire.WriteMessage(c.w, header, body); err != nil {
		return lazyerrors.Error(err)
	}

	if err := c.w.Flush(); err != nil {
		return lazyerrors.Error(err)
	}

	return nil
}

// WriteRaw writes the given raw bytes to the connection.
func (c *Conn) WriteRaw(b []byte) error {
	c.l.Debug(fmt.Sprintf(">>> %d raw bytes", len(b)))

	if _, err := c.w.Write(b); err != nil {
		return lazyerrors.Error(err)
	}

	if err := c.w.Flush(); err != nil {
		return lazyerrors.Error(err)
	}

	return nil
}

// Request sends the given request to the connection and returns the response.
func (c *Conn) Request(ctx context.Context, header *wire.MsgHeader, body wire.MsgBody) (*wire.MsgHeader, wire.MsgBody, error) {
	// TODO https://github.com/FerretDB/FerretDB/issues/4146
	panic("not implemented")
}
