# bRPC-Go

The Go implementation of [bRPC][].

## Prerequisites

- Go

- Protocol buffer compiler, protoc

- Go plugins for the protocol compiler:

    ```sh
    $ go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
    $ go install github.com/xxpbb/brpc-go/cmd/protoc-gen-go-brpc@latest
    ```

## Installation

With Go module support (Go 1.11+), simply add the following import

```go
import "github.com/xxpbb/brpc-go"
```

to your code, and then `go [build|run|test]` will automatically fetch the
necessary dependencies.

[bRPC]: https://github.com/apache/brpc
