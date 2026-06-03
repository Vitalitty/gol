package pkg

type API struct {
}

func NewAPI() *API {
	return &API{}
}

func (a *API) FindSSHConfig(host string) *SSHPathConfig {
	sshConfig, ok := SnapshotGlobalState().FindSSHConfig("", host)
	if !ok {
		return nil
	}
	return &sshConfig
}
