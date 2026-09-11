package storage

// Migrations embed path - the migrations directory lives at the storage package level
// We use go:embed via the migrations.go file's initialSchema var
// This file exists to ensure the embed works correctly relative to the package root
