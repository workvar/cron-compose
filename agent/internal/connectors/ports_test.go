package connectors

import (
	"strings"
	"testing"
)

func TestParseSsListenExtractsPidAndPort(t *testing.T) {
	raw := strings.Join([]string{
		"State Recv-Q Send-Q Local Address:Port Peer Address:PortProcess",
		`LISTEN 0 4096 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=823,fd=3))`,
		`LISTEN 0 511 *:3000 *:* users:(("node",pid=1452,fd=23))`,
		`LISTEN 0 511 [::]:443 [::]:* users:(("nginx",pid=900,fd=8))`,
		`LISTEN 0 4096 127.0.0.1:5432 0.0.0.0:* users:(("postgres",pid=555,fd=7))`,
		`ESTAB 0 0 127.0.0.1:22 127.0.0.1:9 users:(("sshd",pid=823,fd=4))`,
	}, "\n")

	got := parseSsListen(raw)
	want := []listenSock{
		{Proto: "tcp", Address: "0.0.0.0", Port: 22, PID: 823, Process: "sshd"},
		{Proto: "tcp", Address: "*", Port: 3000, PID: 1452, Process: "node"},
		{Proto: "tcp", Address: "[::]", Port: 443, PID: 900, Process: "nginx"},
		{Proto: "tcp", Address: "127.0.0.1", Port: 5432, PID: 555, Process: "postgres"},
	}
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestParseLsofListenExtractsPidAndPort(t *testing.T) {
	raw := strings.Join([]string{
		"COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME",
		"node      456 deploy 23u  IPv4  12345      0t0  TCP *:3000 (LISTEN)",
		"sshd      123 root   3u   IPv6  12346      0t0  TCP *:22 (LISTEN)",
		"nginx     900 www    8u   IPv4  12347      0t0  TCP 127.0.0.1:80 (LISTEN)",
		"curl      12  me     3u   IPv4  1          0t0  TCP 1.2.3.4:443 (ESTABLISHED)",
	}, "\n")

	got := parseLsofListen(raw)
	want := []listenSock{
		{Proto: "tcp", Address: "*", Port: 3000, PID: 456, Process: "node"},
		{Proto: "tcp", Address: "*", Port: 22, PID: 123, Process: "sshd"},
		{Proto: "tcp", Address: "127.0.0.1", Port: 80, PID: 900, Process: "nginx"},
	}
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestSystemdUnitFromCgroup(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"0::/system.slice/ssh.service\n", "ssh.service"},
		{"0::/system.slice/system-postgresql.slice/postgresql@14-main.service\n", "postgresql@14-main.service"},
		{"1:name=systemd:/system.slice/nginx.service\n", "nginx.service"},
		{"0::/init.scope\n", ""},
		{"0::/user.slice/user-1000.slice/session-3.scope\n", ""},
		{"0::/\n", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := systemdUnitFromCgroup(c.in)
		if got != c.want {
			t.Errorf("cgroup %q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestPortIsProtected(t *testing.T) {
	self := 99
	if !portIsProtected(listenSock{PID: 1, Process: "systemd"}, "", self) {
		t.Fatal("pid 1 must be protected")
	}
	if !portIsProtected(listenSock{PID: self, Process: "agent"}, "foo.service", self) {
		t.Fatal("the agent pid must be protected")
	}
	if !portIsProtected(listenSock{PID: 12, Process: "croncompose-agent"}, "foo.service", self) {
		t.Fatal("the agent binary must be protected")
	}
	if !portIsProtected(listenSock{PID: 12, Process: "node"}, "croncompose-agent.service", self) {
		t.Fatal("the agent unit must be protected")
	}
	if portIsProtected(listenSock{PID: 12, Process: "node"}, "api.service", self) {
		t.Fatal("ordinary unit should not be protected")
	}
}

func TestAttachOwnersKeepsOnlyKnownPidsAndMarksProtected(t *testing.T) {
	socks := []listenSock{
		{Proto: "tcp", Address: "*", Port: 3000, PID: 10, Process: "node"},
		{Proto: "tcp", Address: "*", Port: 22, PID: 1, Process: "sshd"},
		{Proto: "tcp", Address: "*", Port: 80, PID: 99, Process: "nginx"},
		{Proto: "tcp", Address: "*", Port: 9, PID: 50, Process: "croncompose-agent"},
	}
	owners := map[int]portOwner{
		10: {Ref: "3", Name: "api"},
		50: {Ref: "croncompose-agent.service", Name: "croncompose-agent"},
	}
	got := attachOwners(socks, owners, 7)
	if len(got) != 2 {
		t.Fatalf("len: got %d want 2 (%+v)", len(got), got)
	}
	if got[0].Ref != "3" || got[0].Port != 3000 || got[0].Protected {
		t.Errorf("api row: %+v", got[0])
	}
	if got[1].Port != 9 || !got[1].Protected {
		t.Errorf("agent row should be listed but protected: %+v", got[1])
	}
}
