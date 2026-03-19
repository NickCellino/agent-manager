# my-cli

A simple CLI application built with Go, Bubble Tea, and urfave/cli.

## Prerequisites

- Go 1.21 or later

## Installation

### Build from source

```bash
go build -o my-cli
```

### Install to $GOPATH/bin

```bash
go install
```

## Usage

### Run directly

```bash
go run . hello
```

### Run the binary

```bash
./my-cli hello
```

The `hello` command will launch an interactive TUI that asks for your name and greets you.

## Testing

Run all tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

## Project Structure

```
my-cli/
├── main.go           # Entry point with urfave/cli
├── commands/
│   ├── hello.go      # Bubbletea TUI model for hello command
│   └── hello_test.go # Tests for Update/View logic
├── go.mod
└── go.sum
```

## Adding New Commands

1. Create a new file in `commands/` directory
2. Define your bubbletea model and command function
3. Export a function that returns `*cli.Command`
4. Register the command in `main.go`

