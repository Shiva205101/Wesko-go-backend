# Cmd

This repository still uses the root `main.go` entrypoint for backward compatibility.

Keep future binary entrypoints in `cmd/`, for example `cmd/api`, and move the root wrapper to a thin compatibility shim when you are ready to finish that refactor.
