// Copyright 2019 Google LLC
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

// Package netlistener provides utilities for net.Listener.
package netlistener

import (
	"fmt"
	"net"

	"sync"

	"github.com/pkg/errors"
)

// ListenerHolder holds an opened port that can only be handed off to 1 go routine.
type ListenerHolder struct {
	number   int
	listener net.Listener
	addr     string
	sync.RWMutex
}

// Obtain returns the TCP listener. This method can only be called once and is thread-safe.
func (lh *ListenerHolder) Obtain() (net.Listener, error) {
	lh.Lock()
	defer lh.Unlock()
	listener := lh.listener
	lh.listener = nil
	if listener == nil {
		return nil, errors.WithStack(fmt.Errorf("cannot Obtain() listener for %d because already handed off", lh.number))
	}
	return listener, nil
}

// Number returns the port number.
func (lh *ListenerHolder) Number() int {
	return lh.number
}

// AddrString returns the address of the serving port.
// Use this over fmt.Sprintf(":%d", lh.Number()) because the address is represented differently in
// systems that prefer IPv4 and IPv6.
func (lh *ListenerHolder) AddrString() string {
	return lh.addr
}

// Close shutsdown the TCP listener.
func (lh *ListenerHolder) Close() error {
	lh.Lock()
	defer lh.Unlock()
	if lh.listener != nil {
		err := lh.listener.Close()
		lh.listener = nil
		return err
	}
	return nil
}

// NewFromPortNumber opens a TCP listener based on the port number provided.
func NewFromPortNumber(portNumber int) (*ListenerHolder, error) {
	addr := fmt.Sprintf(":%d", portNumber)
	conn, err := net.Listen("tcp", addr)

	tcpConn, ok := conn.Addr().(*net.TCPAddr)
	if !ok || tcpConn == nil {
		return nil, fmt.Errorf("net.Listen(\"tcp\", %s) did not return a *net.TCPAddr", addr)
	}

	if err == nil {
		return &ListenerHolder{
			number:   tcpConn.Port,
			listener: conn,
			addr:     conn.Addr().String(),
		}, nil
	}
	return nil, err
}
