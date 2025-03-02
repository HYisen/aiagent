# AI Agent

## Goal

A server side app that provide HTTP interface to access LLM service for me.

## Decisions

### Why not `github.com/openai/openai-go`?

It often works. But occasionally I witness error.

> unexpected end of JSON input

Most likely there is something wrong with the response parser.

I don't want to fork the client to dig out how and fix it, but
decided to maintain a minimum one of top quality myself.
