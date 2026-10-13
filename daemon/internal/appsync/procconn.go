// Package appsync watches process starts and exits through the kernel proc
// connector and tells which scenario the running apps call for.
package appsync

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"
)

// Kernel ABI (linux/connector.h, linux/cn_proc.h).
const (
	cnIdxProc         = 1
	cnValProc         = 1
	procCnMcastListen = 1
	procCnMcastIgnore = 2
	procEventExec     = 0x00000002
	procEventExit     = 0x80000000

	nlmsgHdrLen = 16
	cnMsgLen    = 20 // cb_id (8) + seq + ack + len (u16) + flags (u16)
)

// event is one exec or exit of a process (thread-group leader).
type event struct {
	exec bool
	pid  int
}

// procConn is a NETLINK_CONNECTOR socket subscribed to proc events. It
// needs CAP_NET_ADMIN.
type procConn struct {
	fd     int
	filter bool // the kernel takes an event mask (6.6+): no fork/uid/... events
}

func openProcConn() (*procConn, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM|syscall.SOCK_CLOEXEC, syscall.NETLINK_CONNECTOR)
	if err != nil {
		return nil, fmt.Errorf("netlink connector socket: %w", err)
	}
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: cnIdxProc}); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("bind proc connector: %w", err)
	}
	// A receive timeout lets the reader notice that the context is done.
	tv := syscall.Timeval{Sec: 1}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("proc connector timeout: %w", err)
	}
	// Bursts of execs (a build, a shell loop) must not overflow the buffer.
	_ = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 1<<20)
	c := &procConn{fd: fd, filter: kernelAtLeast(6, 6)}
	if err := c.control(procCnMcastListen); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("proc connector listen: %w", err)
	}
	return c, nil
}

func (c *procConn) control(op uint32) error {
	// struct proc_input {mcast_op, event_type} on 6.6+; before, only the op
	// (a longer message makes old kernels ignore the request).
	n := 4
	if c.filter {
		n = 8
	}
	b := make([]byte, nlmsgHdrLen+cnMsgLen+n)
	le := binary.LittleEndian
	le.PutUint32(b[0:], uint32(len(b)))       // nlmsg_len
	le.PutUint16(b[4:], syscall.NLMSG_DONE)   // nlmsg_type
	le.PutUint32(b[12:], uint32(os.Getpid())) // nlmsg_pid
	le.PutUint32(b[16:], cnIdxProc)           // cb_id.idx
	le.PutUint32(b[20:], cnValProc)           // cb_id.val
	le.PutUint16(b[32:], uint16(n))           // len
	le.PutUint32(b[36:], op)
	if c.filter {
		le.PutUint32(b[40:], procEventExec|procEventExit)
	}
	return syscall.Sendto(c.fd, b, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK})
}

// errOverrun means the socket buffer overflowed and events were lost.
var errOverrun = errors.New("proc connector: events lost")

// read waits up to the receive timeout for the next datagram and returns
// its exec/exit events (none on timeout).
func (c *procConn) read(buf []byte) ([]event, error) {
	n, _, err := syscall.Recvfrom(c.fd, buf, 0)
	if err != nil {
		switch err {
		case syscall.EAGAIN, syscall.EINTR:
			return nil, nil
		case syscall.ENOBUFS:
			return nil, errOverrun
		}
		return nil, err
	}
	return parse(buf[:n]), nil
}

// parse decodes the netlink messages of one datagram.
func parse(b []byte) []event {
	le := binary.LittleEndian
	var out []event
	for len(b) >= nlmsgHdrLen {
		l := int(le.Uint32(b[0:]))
		if l < nlmsgHdrLen || l > len(b) {
			break
		}
		msg := b[nlmsgHdrLen:l]
		// proc_event: what (u32), cpu (u32), timestamp_ns (u64), then the
		// union: exec {pid, tgid}, exit {pid, tgid, code, signal, ...}.
		if len(msg) >= cnMsgLen+24 && le.Uint32(msg[0:]) == cnIdxProc {
			ev := msg[cnMsgLen:]
			what := le.Uint32(ev[0:])
			pid, tgid := int(le.Uint32(ev[16:])), int(le.Uint32(ev[20:]))
			if pid == tgid {
				switch what {
				case procEventExec:
					out = append(out, event{exec: true, pid: pid})
				case procEventExit:
					out = append(out, event{exec: false, pid: pid})
				}
			}
		}
		l = (l + 3) &^ 3 // NLMSG_ALIGN
		if l >= len(b) {
			break
		}
		b = b[l:]
	}
	return out
}

func (c *procConn) close() {
	_ = c.control(procCnMcastIgnore)
	syscall.Close(c.fd)
}

// kernelAtLeast compares the running kernel release with major.minor.
func kernelAtLeast(major, minor int) bool {
	var u syscall.Utsname
	if syscall.Uname(&u) != nil {
		return false
	}
	var rel []byte
	for _, c := range u.Release {
		if c == 0 {
			break
		}
		rel = append(rel, byte(c))
	}
	var ma, mi int
	if _, err := fmt.Sscanf(string(rel), "%d.%d", &ma, &mi); err != nil {
		return false
	}
	return ma > major || ma == major && mi >= minor
}
