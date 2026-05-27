<p align="center">
  <a href="https://github.com/Vitalitty/gol">
    <img alt="gol" src="https://imgur.com/sktoYPP.png" width="120">
  </a>
</p>

<h1 align="center">
  Logs Viewer
</h1>

<p align="center">
  View realtime logs in your fav browser<br>
  Advanced regex search<br>
  Low Mem Footprint<br>
  Single binary
</p>

<h3 align="center">
  Supports
</h3>

<p align="center">
  Docker Container logs from path<br>
  Docker Container logs<br>
  SSH remote logs<br>
  STDIN logs<br>
  Local logs<br>
  Tar logs<br>
</p>

- **Quick Setup:** One command to install and run.

- **Hassle Free:** Doesn't require elastic search or other shebang.

- **Platform:** Supports (arm64, arch64, Mac, Mac M1, Ubuntu and Windows).

- **Flexible:** View docker logs, remote logs over ssh, files on disk and piped inputs in browser.

- **Intelligent** Smartly judges log level, and dates.

- **Search** Fast search with regex.

- **Realtime** Tail logs in real time in browser.

- **Log Rotation** Supports log rotation and watch for new log files.

- **Embed in GO** Easily embed in your existing Go app.

<h1 align="center">
  View in Browser
</h1>

<p align="center">
 Intuitive UI to view logs in browser
</p>

<p align="center">
  <a href="https://github.com/Vitalitty/gol">
    <img alt="gol" src="https://imgur.com/fBK0hGa.png">
  </a>
</p>

### Install using curl

Use this method if go is not installed on your server

```bash
curl -sL https://raw.githubusercontent.com/Vitalitty/gol/main/install.sh | sh
```

### Run with Docker

Container examples for local files, Docker logs, SSH keys, custom flags, and
volume permissions are available in [docker/README.md](docker/README.md).

## Examples

### CLI - Basic Example

```sh
# run in current directory for pattern
gol "*log" "access/*log.tar.gz"
```

### CLI - Advanced Examples

All patterns work in combination with each other.

```sh
# search using pipe and file patterns
demsg | gol -f="/var/log/*.log"

# over ssh
# port optional (default 22), password optional (default ''), private_key optional (default $HOME/.ssh/id_rsa)
gol -s="user@host[:port] [password=/path/to/password] [private_key=/path/to/key] /app/*logs"

# Docker all container logs
gol -d=""

# Docker specific container logs
gol -d="container-id"

# Docker specific path on a container
gol -d="container-id /app/logs.log"

# All patterns combined
gol -d="container-id" \
    -d="container-id /app/logs.log" \
    -s="user@host[:port] [password=/path/to/password] [private_key=/path/to/key] /app/*logs" \
    -f="/var/log/*.log"
```

### Embed in GO

If you don't want to use the CLI on a separate port and want to integrate within your existing Go app.


```go
import (
	"fmt"
	"net/http"

	"github.com/Vitalitty/gol"
)

func main() {
    // init with options of file path you want to watch
    g := gol.NewGol(func(o *gol.GolOptions) error {
        o.FilePaths = []string{"*.log"}
        return nil
    })

    // register following two routes
    http.HandleFunc("/gol/api", g.Adapter(g.NewAPIHandler().Get))
    http.HandleFunc("/gol", g.Adapter(g.NewAssetsHandler().Get))

    // start server as usual
    http.ListenAndServe("localhost:8080", nil)
}
```

## Fork Maintenance

This repository is the maintained `Vitalitty/gol` fork of the original
[`kevincobain2000/gol`](https://github.com/kevincobain2000/gol) project.
New embedded integrations should import `github.com/Vitalitty/gol`.

## Limitations

- **Docker Logs:** Only supports logs from containers running on the same machine.
- **fmt, stdout:** For embedded use, fmt and stdout logs are not intercepted.

  **Tip:** If you want to capture, then run your app by piping output as `./app >> logs.log`.


## Development Notes

Prerequisites:

- Go 1.26.3
- Node.js 24 LTS
- npm, using the checked-in lockfile with `npm ci`

```sh
# Get some fake logs
mkdir -p testdata
while true; do date >> testdata/test.log; sleep 1; done

# Start the API
cd frontend
go run main.go --cors=4321 --open=false -f="../testdata/*log"
# API development on http://localhost:3003/api

# Start the frontend
npm ci
npm run dev
# Frontend development on http://localhost:4321/
```

`npm audit --omit=dev` should remain clean. A full `npm audit` can report a
dev-only advisory through `@astrojs/check` and its language-server YAML
dependency chain; keep `@astrojs/check` current and reassess when upstream
publishes a compatible fix.
