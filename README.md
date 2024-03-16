[![MIT license](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/kaatinga/luna/blob/main/LICENSE)
[![lint workflow](https://github.com/kaatinga/luna/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/luna/luna/actions?query=workflow%3Alinter)
[![help wanted](https://img.shields.io/badge/Help%20wanted-True-yellow.svg)](https://github.com/luna/strconv/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22)

# Luna

Luna is a flexible and generic worker pool implementation in Go, designed to manage and orchestrate workers efficiently. It supports generic types for keys and workers, allowing for a wide range of applications. Workers in the pool can be started, stopped, and have operations executed upon them, providing a robust framework for concurrent task execution.

## Features

- Generic implementation supporting any key and worker types.
- Thread-safe operations to add, delete, and manipulate workers.
- Error handling for start and stop operations of workers.

## Installation

To install Luna, you need a working Go environment. Luna can be installed using `go get`:

```sh
go get github.com/kaatinga/luna
```

## Usage

Here's a simple example of how to use Luna:

```go
package main

import (
	"fmt"
	"github.com/kaatinga/luna"
)

type myWorker struct{}

func (m myWorker) Start() error {
	fmt.Println("Worker started")
	return nil
}

func (m myWorker) Stop() error {
	fmt.Println("Worker stopped")
	return nil
}

func main() {
	pool := luna.NewWorkerPool[string, myWorker]()

	err := pool.Add("worker1", myWorker{})
	if err != nil {
		fmt.Printf("Error adding worker: %v\n", err)
	}

	pool.Do("worker1", func(item *luna.Item[string, myWorker]) {
		fmt.Println("Executing operation on worker")
	})

	err = pool.Delete("worker1")
	if err != nil {
		fmt.Printf("Error removing worker: %v\n", err)
	}
}
```

## Testing

To run the tests for Luna, navigate to the package directory and use the Go tool:

```sh
go test ./...
```

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open issues to discuss potential improvements or features.

## License

[MIT License](LICENSE.md)