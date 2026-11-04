module github.com/yuhaiin/purego-sqlite-embed/tests/integration

go 1.26.0

require (
	github.com/yuhaiin/purego-sqlite v0.0.0-20261104052809-4c7657559a8b
	github.com/yuhaiin/purego-sqlite-embed v0.0.0
)

require (
	github.com/ebitengine/purego v0.11.3 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/yuhaiin/purego-sqlite-embed => ../..
