# Go TUI Multiplayer Demo

## Run

Start the server:

```
go run ./cmd/server -addr :7777 -db game.db
```

Start one or more clients:

```
go run ./cmd/client -addr 127.0.0.1:7777 -name Alice
```

## Controls

- Arrow keys: move
- c: challenge selected target
- n/p: switch target
- y: accept challenge
- d: decline challenge
- q: quit
