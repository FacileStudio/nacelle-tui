package main

// The top-level key router, split from model.go because that file sat on
// filet's 250-line cap. The seam is the one menukeys.go and historykeys.go
// already use: model.go holds what the client is, and a file per group of
// bindings holds which press reaches what.
