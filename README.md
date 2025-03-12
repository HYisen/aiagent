# AI Agent

## Goal

A server side app that provide HTTP interface to access LLM service for me.

## Usage

```bash
# gen code are not included in git, re-generate them
go run tools/gen/main.go

# check docs/*.sql and do the necessary DDL to setup DB
# check Repository code to know which DataBase

# compile
go build

# check built-in help
./aiagent -h

# do what you want
./aiagent
```

### tools/client

A not most feature completed, debug purpose client.

Check its README to get knowledge about its usage.

## Decisions

### Why not `github.com/openai/openai-go`?

It often works. But occasionally I witness error.

> unexpected end of JSON input

Most likely there is something wrong with the response parser.

I don't want to fork the client to dig out how and fix it, but
decided to maintain a minimum one of top quality myself.

### Why not Server-Sent Events third-party package?

Because it's easy.

First I took a glance over third-party library such as
[sse](https://github.com/r3labs/sse)
and
[go-sse](https://github.com/tmaxmax/go-sse)
.

And then figure out that the official client maintain the codec by themselves.
[usage](https://github.com/openai/openai-go/blob/main/chatcompletion.go#L150)
&
[implementation](https://github.com/openai/openai-go/blob/main/packages/ssestream/ssestream.go)

Later I took [a tutorial](https://www.freecodecamp.org/news/how-to-implement-server-sent-events-in-go/) as an example.
Finding that it might be much easier if I maintain a codec by myself.

As I only need a narrow range of features, can drop compatibility and generic as long as it works in my case.
