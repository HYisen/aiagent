# client

client is a client that attaches aiagent server mode.

Since the server has been deployed and shall be always online,
client has become the official suggested way to interact with server,
comparing to REPL mode or RESTFul endpoints without a client.

## Usage

### Binary

Checkout results of workflow client, the pipeline shall have pre-built binary for various platforms stored in artifacts.

### IDE

Change the code as you preferred, and use the lovely green triangle Run button of your IDE,
run the `func main()` of `tools/client/main.go`.

### Scratch

Let's assume there is nothing, apart from the toolchains like `git` or `go`.

Choose your favored workspace, maybe `cd /tmp`.

build

```shell
git clone https://github.com/hyisen/aiagent

cd aiagent

go generate ./...

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