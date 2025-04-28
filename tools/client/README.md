# client

client is a client that attaches aiagent server mode.

Simulate aiagent REPL mode user experience.

Its goal is to help me debug aiagent server mode, especially of chat stream API.

Not supposed to become a full-fledged client.
Therefore, most server API and not hard-coded configs would not be supported.

## Usage

### Normal

Change the code as you preferred, and use the lovely green triangle Run button of your IDE,
run the `func main()` of `tools/client/main.go`.

### Scratch

Let's assume there is nothing, apart from the toolchains like `git` or `go`.

Choose your favored workspace, maybe `cd /tmp`.

build

```shell
git clone https://github.com/hyisen/aiagent

cd aiagent

# generate generated code
go run tools/gen/main.go

# build server
go build

# build client
go build ./tools/client
```

server

```shell
# check the usage first
./aiagent -h

# Only for the first time, or you find it runs unexpectedly.
# create DB and apply DDLs
./aiagent --mode=migrate
# If failed, read its output, most likely you shall execute `rm db`.

# get your API key, assume it's sk-1234567-this-is-a-fake-key
./aiagent --DeepSeekAPIKey=sk-1234567-this-is-a-fake-key --mode=server
# Will block TTY, use another or embrace helpers such as nohup.
# For invalid port situation, debug it yourself, the easiest solution may be reboot OS.
```

client

### client

```shell
./client  
```