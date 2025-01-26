module github.com/mateusf777/pubsub/example

go 1.23.4

require github.com/mateusf777/pubsub/client v0.1.5

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mateusf777/pubsub/core v0.1.6 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/mateusf777/pubsub/client v0.1.5 => ../client
	github.com/mateusf777/pubsub/core v0.1.6 => ../core
)
