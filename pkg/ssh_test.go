package pkg

import (
	"errors"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestIsRetryableSFTPChannelError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "channel open rejected",
			err:  errors.New("ssh: rejected: connect failed (open failed)"),
			want: true,
		},
		{
			name: "permission denied",
			err:  errors.New("permission denied"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isRetryableSFTPChannelError(test.err); got != test.want {
				t.Fatalf("isRetryableSFTPChannelError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

func TestRemoveCachedSSHClientDeletesOnlyMatchingHost(t *testing.T) {
	previous := GlobalSSHClients
	t.Cleanup(func() {
		clientMutex.Lock()
		GlobalSSHClients = previous
		clientMutex.Unlock()
	})

	clientMutex.Lock()
	GlobalSSHClients = map[string]*ssh.Client{
		"ssh-a": nil,
		"ssh-b": nil,
	}
	clientMutex.Unlock()

	removeCachedSSHClient(&SSHConfig{SourceID: "ssh-a", Host: "host-a", Port: "22"})

	clientMutex.Lock()
	defer clientMutex.Unlock()
	if _, ok := GlobalSSHClients["ssh-a"]; ok {
		t.Fatal("host-a cache entry was not removed")
	}
	if _, ok := GlobalSSHClients["ssh-b"]; !ok {
		t.Fatal("host-b cache entry was removed")
	}
}

func TestSSHClientKeyUsesSourceIDAndUserFallback(t *testing.T) {
	if got := sshClientKey(&SSHConfig{SourceID: "ssh-a", User: "user", Host: "host-a", Port: "22"}); got != "ssh-a" {
		t.Fatalf("sshClientKey with source id = %q, want ssh-a", got)
	}
	if got := sshClientKey(&SSHConfig{User: "user", Host: "host-a", Port: "22"}); got != "user@host-a:22" {
		t.Fatalf("sshClientKey fallback = %q, want user@host-a:22", got)
	}
}
